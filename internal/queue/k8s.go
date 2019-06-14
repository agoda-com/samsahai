package queue

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
)

// List takes label and field selectors, and returns the list of Queues that match those selectors.
func (c *controller) List(opts *metav1.ListOptions) (results *v1beta1.QueueList, err error) {
	var timeout time.Duration
	if opts == nil {
		opts = &metav1.ListOptions{FieldSelector: "metadata.namespace=" + c.namespace}
	}
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	results = &v1beta1.QueueList{}
	err = c.restClient.
		Get().
		Namespace(c.namespace).
		Resource("queues").
		VersionedParams(opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(results)
	return
}

// Delete takes name of the queue and deletes it. Returns an error if one occurs.
func (c *controller) Delete(name string, options *metav1.DeleteOptions) error {
	return c.restClient.Delete().
		Namespace(c.namespace).
		Resource("queues").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *controller) DeleteCollection(options *metav1.DeleteOptions, listOpts *metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts == nil {
		listOpts = &metav1.ListOptions{FieldSelector: "metadata.namespace=" + c.namespace}
	}
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.restClient.Delete().
		Namespace(c.namespace).
		Resource("queues").
		VersionedParams(listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Update takes the representation of a queue and updates it.
// Returns the server's representation of the queue, and an error, if there is any.
func (c *controller) Update(queue *v1beta1.Queue) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.restClient.Put().
		Namespace(c.namespace).
		Resource("queues").
		Name(queue.Name).
		Body(queue).
		Do().
		Into(result)
	return
}

// Create takes the representation of a queue and creates it.
// Returns the server's representation of the queue, and an error, if there is any.
func (c *controller) Create(queue *v1beta1.Queue) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.restClient.Post().
		Namespace(c.namespace).
		Resource("queues").
		Body(queue).
		Do().
		Into(result)
	return
}
