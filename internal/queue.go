package internal

import (
	"time"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
)

// QueueController manages updating component queue through CRD
type QueueController interface {
	// Add adds Queue
	Add(q *s2hv1.Queue) error

	// AddTop adds Queue to the top
	AddTop(q *s2hv1.Queue) error

	// First returns first component in Queue or current running Queue
	First() (*s2hv1.Queue, error)

	// Remove removes Queue
	Remove(q *s2hv1.Queue) error

	// Size returns no of queues
	Size() int

	// SetLastOrder sets queue order to the last
	SetLastOrder(q *s2hv1.Queue) error

	// SetReverifyQueueAtFirst sets queue to reverify type
	SetReverifyQueueAtFirst(q *s2hv1.Queue) error

	// SetRetryQueue sets Queue to retry one time
	SetRetryQueue(q *s2hv1.Queue, noOfRetry int, nextAt time.Time) error

	// RemoveAllQueues removes all queues
	RemoveAllQueues() error
}
