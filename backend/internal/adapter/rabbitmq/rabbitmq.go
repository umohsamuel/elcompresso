// Package rabbitmq is the job queue: a durable queue of job ids with
// publisher confirms and manual acknowledgements.
package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/umohsamuel/elcompresso/internal/domain/queue"
)

type Queue struct {
	conn *amqp.Connection
	name string

	pubMu sync.Mutex
	pub   *amqp.Channel
}

var _ queue.Queue = (*Queue)(nil)

// Connect dials url and declares the durable queue name.
func Connect(url, name string) (*Queue, error) {
	conn, err := amqp.DialConfig(url, amqp.Config{Heartbeat: 10 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: %w", err)
	}
	pub, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}
	if _, err := pub.QueueDeclare(name, true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}
	if err := pub.Confirm(false); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable confirms: %w", err)
	}
	return &Queue{conn: conn, name: name, pub: pub}, nil
}

// Closed reports when the connection drops. The process should exit then and
// let its supervisor restart it; unacknowledged jobs are redelivered.
func (q *Queue) Closed() <-chan *amqp.Error {
	return q.conn.NotifyClose(make(chan *amqp.Error, 1))
}

func (q *Queue) Publish(ctx context.Context, jobID string) error {
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
	conf, err := q.pub.PublishWithDeferredConfirmWithContext(ctx, "", q.name, true, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "text/plain",
		Body:         []byte(jobID),
		Timestamp:    time.Now(),
	})
	if err != nil {
		return fmt.Errorf("publish job: %w", err)
	}
	ok, err := conf.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("publish job: %w", err)
	}
	if !ok {
		return errors.New("publish job: rejected by broker")
	}
	return nil
}

func (q *Queue) Consume(ctx context.Context, prefetch int) (<-chan queue.Delivery, error) {
	ch, err := q.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		ch.Close()
		return nil, err
	}
	msgs, err := ch.ConsumeWithContext(ctx, q.name, "", false, false, false, false, nil)
	if err != nil {
		ch.Close()
		return nil, fmt.Errorf("consume: %w", err)
	}

	out := make(chan queue.Delivery)
	go func() {
		defer close(out)
		defer ch.Close()
		for m := range msgs {
			d := queue.Delivery{
				JobID:       string(m.Body),
				Redelivered: m.Redelivered,
				Ack:         func() error { return m.Ack(false) },
				Requeue:     func() error { return m.Nack(false, true) },
			}
			select {
			case out <- d:
			case <-ctx.Done():
				_ = m.Nack(false, true)
				return
			}
		}
	}()
	return out, nil
}

func (q *Queue) Close() error {
	return q.conn.Close()
}
