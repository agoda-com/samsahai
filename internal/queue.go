package internal

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QueueController manages updating component queue through CRD
type QueueController interface {
	// Add adds Queue with priority list
	Add(q client.Object, priorityQueues []string) error

	// AddTop adds Queue to the top
	AddTop(q client.Object) error

	// First returns first component in Queue or current running Queue
	First(namespace string) (client.Object, error)

	// Remove removes Queue
	Remove(q client.Object) error

	// Size returns no of queues
	Size(namespace string) int

	// SetLastOrder sets queue order to the last
	SetLastOrder(obj client.Object) error

	// SetReverifyQueueAtFirst sets queue to reverify type
	SetReverifyQueueAtFirst(q client.Object) error

	// SetRetryQueue sets Queue to retry one time
	SetRetryQueue(q client.Object, noOfRetry int, nextAt time.Time,
		isTriggerFailed *bool, triggerCreateAt, triggerFinishedAt *metav1.Time) error

	// RemoveAllQueues removes all queues
	RemoveAllQueues(namespace string) error
}
