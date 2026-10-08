package job

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("job not found or expired")

// Store persists job records. Implementations keep their indexes (waiting
// order, per-client counts, expiry) in step with each record's status.
type Store interface {
	// Create saves a new job.
	Create(ctx context.Context, rec *Record) error

	// Get returns a job, or ErrNotFound.
	Get(ctx context.Context, id string) (*Record, error)

	// Update applies fn to the stored record atomically. fn may run more than
	// once if the record changes concurrently; returning an error aborts.
	Update(ctx context.Context, id string, fn func(*Record) error) (*Record, error)

	Delete(ctx context.Context, id string) error

	// QueuePosition is the 1-based place of a queued job among queued jobs.
	QueuePosition(ctx context.Context, id string) (int, error)

	// ActiveForClient counts a client's unfinished jobs.
	ActiveForClient(ctx context.Context, clientIP string) (int, error)

	// Counts returns how many jobs are queued and processing.
	Counts(ctx context.Context) (queued, processing int, err error)

	// Expired returns up to limit finished jobs whose results expired before now.
	Expired(ctx context.Context, now time.Time, limit int) ([]string, error)

	// RequestCancel asks the worker running a job to stop it.
	RequestCancel(ctx context.Context, id string) error
	CancelRequested(ctx context.Context, id string) (bool, error)
}
