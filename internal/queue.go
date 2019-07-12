package internal

import (
	"time"

	"github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

// QueueController manages updating component queue through CRD
type QueueController interface {
	// Add adds Queue
	Add(q *v1beta1.Queue) error

	// AddTop adds Queue to the top
	AddTop(q *v1beta1.Queue) error

	// First returns first component in Queue or current running Queue
	First() (*v1beta1.Queue, error)

	// Remove removes Queue
	Remove(q *v1beta1.Queue) error

	// Size returns no of queues
	Size() int

	// SetLastOrder sets queue order to the last
	SetLastOrder(q *v1beta1.Queue) error

	// SetReverifyQueueAtFirst sets queue to reverify type
	SetReverifyQueueAtFirst(q *v1beta1.Queue) error

	// SetRetryQueue sets Queue to retry one time
	SetRetryQueue(q *v1beta1.Queue, noOfRetry int, nextAt time.Time) error

	// RemoveAllQueues removes all queues
	RemoveAllQueues() error
}
