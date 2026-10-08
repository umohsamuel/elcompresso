// Package redisstore keeps job records and upload slots in Redis.
//
// Keys (all under the configured prefix):
//
//	job:<id>      JSON job.Record
//	queued        sorted set of queued job ids, scored by creation time
//	processing    set of job ids being encoded
//	active:<ip>   count of a client's unfinished jobs
//	expiring      sorted set of finished job ids, scored by result expiry
//	cancel:<id>   cancel request for a running job
//	upload:<id>   JSON storage.UploadSlot
package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/umohsamuel/elcompresso/internal/domain/job"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
)

const (
	// Unfinished jobs get a long safety TTL so a lost record can't linger forever.
	unfinishedTTL = 48 * time.Hour
	// Finished records outlive their expiry a little so the janitor can still
	// read the result key when it deletes the result.
	finishedGrace = time.Hour
	cancelTTL     = 2 * time.Hour
	maxRetries    = 50
)

type Store struct {
	rdb    *redis.Client
	prefix string
}

var (
	_ job.Store           = (*Store)(nil)
	_ storage.UploadSlots = (*Store)(nil)
)

// Connect opens a client from a redis:// URL and checks the server answers.
func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	return rdb, nil
}

// New uses keys under prefix, e.g. "elcompresso:".
func New(rdb *redis.Client, prefix string) *Store {
	return &Store{rdb: rdb, prefix: prefix}
}

func (s *Store) k(name, id string) string {
	if id == "" {
		return s.prefix + name
	}
	return s.prefix + name + ":" + id
}

func (s *Store) Create(ctx context.Context, rec *job.Record) error {
	_, err := s.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		return s.write(ctx, p, nil, rec)
	})
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*job.Record, error) {
	raw, err := s.rdb.Get(ctx, s.k("job", id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, job.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decode(raw)
}

func (s *Store) Update(ctx context.Context, id string, fn func(*job.Record) error) (*job.Record, error) {
	var out *job.Record
	err := s.transact(ctx, id, func(old *job.Record) (*job.Record, error) {
		if old == nil {
			return nil, job.ErrNotFound
		}
		next := *old
		if old.Job.Result != nil {
			r := *old.Job.Result
			next.Job.Result = &r
		}
		if err := fn(&next); err != nil {
			return nil, err
		}
		out = &next
		return &next, nil
	})
	return out, err
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.transact(ctx, id, func(old *job.Record) (*job.Record, error) {
		if old == nil {
			return nil, errNoop
		}
		return nil, nil
	})
}

var errNoop = errors.New("noop")

// transact reads a record under WATCH and writes whatever change returns
// (nil deletes), retrying if the record changed concurrently.
func (s *Store) transact(ctx context.Context, id string, change func(old *job.Record) (*job.Record, error)) error {
	key := s.k("job", id)
	for range maxRetries {
		err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
			var old *job.Record
			raw, err := tx.Get(ctx, key).Bytes()
			switch {
			case errors.Is(err, redis.Nil):
			case err != nil:
				return err
			default:
				if old, err = decode(raw); err != nil {
					return err
				}
			}
			next, err := change(old)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
				return s.write(ctx, p, old, next)
			})
			return err
		}, key)
		switch {
		case errors.Is(err, redis.TxFailedErr):
			continue // changed underneath us; re-read and retry
		case errors.Is(err, errNoop):
			return nil
		}
		return err
	}
	return fmt.Errorf("job %s: too many concurrent updates", id)
}

// write queues the commands that store next (nil deletes) and move the job
// between indexes according to its old and new status.
func (s *Store) write(ctx context.Context, p redis.Pipeliner, old, next *job.Record) error {
	var id, clientIP string
	var oldStatus, newStatus job.Status
	if old != nil {
		id, clientIP, oldStatus = old.Job.ID, old.ClientIP, old.Job.Status
	}
	if next != nil {
		id, clientIP, newStatus = next.Job.ID, next.ClientIP, next.Job.Status
	}

	if next != nil {
		data, err := json.Marshal(next)
		if err != nil {
			return err
		}
		ttl := unfinishedTTL
		if newStatus.Finished() && next.Job.ExpiresAt != nil {
			ttl = max(time.Until(*next.Job.ExpiresAt), 0) + finishedGrace
		}
		p.Set(ctx, s.k("job", id), data, ttl)
	} else {
		p.Del(ctx, s.k("job", id), s.k("cancel", id))
		p.ZRem(ctx, s.k("expiring", ""), id)
	}

	switch {
	case newStatus == job.StatusQueued:
		p.ZAdd(ctx, s.k("queued", ""), redis.Z{Score: float64(next.Job.CreatedAt.UnixNano()), Member: id})
	case oldStatus == job.StatusQueued:
		p.ZRem(ctx, s.k("queued", ""), id)
	}
	switch {
	case newStatus == job.StatusProcessing:
		p.SAdd(ctx, s.k("processing", ""), id)
	case oldStatus == job.StatusProcessing:
		p.SRem(ctx, s.k("processing", ""), id)
	}

	wasActive := old != nil && !oldStatus.Finished()
	isActive := next != nil && !newStatus.Finished()
	activeKey := s.k("active", clientIP)
	switch {
	case isActive && !wasActive:
		p.Incr(ctx, activeKey)
		p.Expire(ctx, activeKey, unfinishedTTL)
	case wasActive && !isActive:
		p.Decr(ctx, activeKey)
	}

	if next != nil && newStatus.Finished() && next.Job.ExpiresAt != nil {
		p.ZAdd(ctx, s.k("expiring", ""), redis.Z{Score: float64(next.Job.ExpiresAt.UnixMilli()), Member: id})
		p.Del(ctx, s.k("cancel", id))
	}
	return nil
}

func (s *Store) QueuePosition(ctx context.Context, id string) (int, error) {
	rank, err := s.rdb.ZRank(ctx, s.k("queued", ""), id).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int(rank) + 1, nil
}

func (s *Store) ActiveForClient(ctx context.Context, clientIP string) (int, error) {
	n, err := s.rdb.Get(ctx, s.k("active", clientIP)).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return max(n, 0), err
}

func (s *Store) Counts(ctx context.Context) (int, int, error) {
	var queued, processing *redis.IntCmd
	_, err := s.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		queued = p.ZCard(ctx, s.k("queued", ""))
		processing = p.SCard(ctx, s.k("processing", ""))
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return int(queued.Val()), int(processing.Val()), nil
}

func (s *Store) Expired(ctx context.Context, now time.Time, limit int) ([]string, error) {
	return s.rdb.ZRangeByScore(ctx, s.k("expiring", ""), &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now.UnixMilli(), 10),
		Count: int64(limit),
	}).Result()
}

func (s *Store) RequestCancel(ctx context.Context, id string) error {
	return s.rdb.Set(ctx, s.k("cancel", id), 1, cancelTTL).Err()
}

func (s *Store) CancelRequested(ctx context.Context, id string) (bool, error) {
	n, err := s.rdb.Exists(ctx, s.k("cancel", id)).Result()
	return n > 0, err
}

func (s *Store) PutUploadSlot(ctx context.Context, id string, slot storage.UploadSlot, ttl time.Duration) error {
	data, err := json.Marshal(slot)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.k("upload", id), data, ttl).Err()
}

func (s *Store) TakeUploadSlot(ctx context.Context, id string) (*storage.UploadSlot, error) {
	raw, err := s.rdb.GetDel(ctx, s.k("upload", id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var slot storage.UploadSlot
	if err := json.Unmarshal(raw, &slot); err != nil {
		return nil, err
	}
	return &slot, nil
}

func decode(raw []byte) (*job.Record, error) {
	var rec job.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("decode job: %w", err)
	}
	return &rec, nil
}
