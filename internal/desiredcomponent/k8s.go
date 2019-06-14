package desiredcomponent

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	ResourceName = "desiredcomponents"
)

// Get takes name of the deployment, and returns the corresponding deployment object, and an error if there is any.
//func (c *deployments) Get(name string, options metav1.GetOptions) (result *v1.Deployment, err error) {
//	result = &v1.Deployment{}
//	err = c.client.Get().
//		Namespace(c.ns).
//		Resource("deployments").
//		Name(name).
//		VersionedParams(&options, scheme.ParameterCodec).
//		Do().
//		Into(result)
//	return
//}

//// Get takes name of the desiredcomponent, and returns the corresponding desiredcomponent object, and an error if there is any.
//func (c *controller) Get(name string, opts metav1.GetOptions) (result *v1beta1.DesiredComponent, err error) {
//	result = &v1beta1.DesiredComponent{}
//	err = c.restClient.
//		Get().
//		Namespace(c.namespace).
//		Resource(ResourceName).
//		Name(name).
//		VersionedParams(&opts, scheme.ParameterCodec).
//		Do().
//		Into(result)
//	return
//}
//
//// Create takes the representation of a desiredcomponent and creates it.
//// Returns the server's representation of the desiredcomponent, and an error, if there is any.
//func (c *controller) Create(d *v1beta1.DesiredComponent) (result *v1beta1.DesiredComponent, err error) {
//	result = &v1beta1.DesiredComponent{}
//	err = c.restClient.Post().
//		Namespace(c.namespace).
//		Resource(ResourceName).
//		Body(d).
//		Do().
//		Into(result)
//	return
//}
//
//// Update takes the representation of a desiredcomponent and updates it.
//// Returns the server's representation of the desiredcomponent, and an error, if there is any.
//func (c *controller) Update(d *v1beta1.DesiredComponent) (result *v1beta1.DesiredComponent, err error) {
//	result = &v1beta1.DesiredComponent{}
//	err = c.restClient.Put().
//		Namespace(c.namespace).
//		Resource(ResourceName).
//		Name(d.Name).
//		Body(d).
//		Do().
//		Into(result)
//	return
//}
//

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
		Resource(ResourceName).
		VersionedParams(listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}
