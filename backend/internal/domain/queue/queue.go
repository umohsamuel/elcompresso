package queue

import "context"

// Delivery is one job handed to a worker. Exactly one of Ack or Requeue must
// be called: Ack once the job reached a final state, Requeue to hand it to
// another worker (e.g. on shutdown).
type Delivery struct {
	JobID       string
	Redelivered bool
	Ack         func() error
	Requeue     func() error
}

// Queue carries job ids from the API to workers. Messages survive restarts
// and are redelivered if a worker dies before acknowledging them.
type Queue interface {
	Publish(ctx context.Context, jobID string) error

	// Consume delivers at most prefetch unacknowledged jobs at a time. The
	// channel closes when ctx ends or the connection is lost.
	Consume(ctx context.Context, prefetch int) (<-chan Delivery, error)

	Close() error
}
