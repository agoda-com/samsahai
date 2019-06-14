package internal

import (
	envv1beta1 "github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
)

// QueueController manages updating component queue through CRD
type QueueController interface {
	// Add adds Queue
	Add(q *envv1beta1.Queue) error

	// AddTop adds Queue to the top
	AddTop(q *envv1beta1.Queue) error

	// First returns first component in Queue
	First() (*envv1beta1.Queue, error)

	// Remove removes Queue
	Remove(q *envv1beta1.Queue) error

	// Size returns no of queues
	Size() int

	// RemoveAll removes all Queue
	RemoveAll() error
}
