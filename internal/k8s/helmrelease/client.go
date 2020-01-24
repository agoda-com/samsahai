package helmrelease

import (
	"context"

	"github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName("helm-release")

type client struct {
	client    crclient.Client
	namespace string
}

func New(namespace string, c crclient.Client) internal.HelmReleaseClient {
	return &client{
		client:    c,
		namespace: namespace,
	}
}

// Get takes name of the helmrelease, and returns the corresponding HelmRelease object, and an error if there is any.
func (c *client) Get(name string, _ metav1.GetOptions) (result *v1beta1.HelmRelease, err error) {
	result = &v1beta1.HelmRelease{}
	err = c.client.Get(context.TODO(), types.NamespacedName{Namespace: c.namespace, Name: name}, result)
	return
}

// List takes label and field selectors, and returns the list of HelmRelease that match those selectors.
func (c *client) List(_ metav1.ListOptions) (results *v1beta1.HelmReleaseList, err error) {
	results = &v1beta1.HelmReleaseList{}
	err = c.client.List(context.TODO(), results, &crclient.ListOptions{Namespace: c.namespace})
	return
}

// Create takes the representation of a HelmRelease and creates it.
// Returns the server's representation of the HelmRelease, and an error, if there is any.
func (c *client) Create(release *v1beta1.HelmRelease) (result *v1beta1.HelmRelease, err error) {
	release.Namespace = c.namespace
	err = c.client.Create(context.TODO(), release)
	return
}

// Update takes the representation of a HelmRelease and updates it.
// Returns the server's representation of the HelmRelease, and an error, if there is any.
func (c *client) Update(release *v1beta1.HelmRelease) (result *v1beta1.HelmRelease, err error) {
	result = release
	result.Namespace = c.namespace
	err = c.client.Update(context.TODO(), result)
	return
}

// Delete takes name of the HelmRelease and deletes it. Returns an error if one occurs.
func (c *client) Delete(name string, options *metav1.DeleteOptions) error {
	result, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return c.client.Delete(context.TODO(), result)
}

// DeleteCollection deletes a collection of objects.
func (c *client) DeleteCollection(options *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return c.client.DeleteAllOf(context.TODO(), &v1beta1.HelmRelease{}, crclient.InNamespace(c.namespace))
}
