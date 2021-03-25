package internal

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// QueueController manages updating component queue through CRD
type QueueController interface {
	// Add adds Queue with priority list
	Add(q runtime.Object, priorityQueues []string) error

	// AddTop adds Queue to the top
	AddTop(q runtime.Object) error

	// First returns first component in Queue or current running Queue
	First(namespace string) (runtime.Object, error)

	// Remove removes Queue
	Remove(q runtime.Object) error

	// Size returns no of queues
	Size(namespace string) int

	// SetLastOrder sets queue order to the last
	SetLastOrder(obj runtime.Object) error

	// SetReverifyQueueAtFirst sets queue to reverify type
	SetReverifyQueueAtFirst(q runtime.Object) error

	// SetRetryQueue sets Queue to retry one time
	SetRetryQueue(q runtime.Object, noOfRetry int, nextAt time.Time,
		isTriggerFailed *bool, triggerCreateAt, triggerFinishedAt *metav1.Time) error

	// RemoveAllQueues removes all queues
	RemoveAllQueues(namespace string) error
}
