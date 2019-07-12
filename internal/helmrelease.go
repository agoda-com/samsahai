package internal

import (
	"github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HelmReleaseClient interface {
	// Get takes name of the helmrelease, and returns the corresponding helmrelease object, and an error if there is any.
	Get(name string, opts metav1.GetOptions) (*v1beta1.HelmRelease, error)

	// List takes label and field selectors, and returns the list of HelmRelease that match those selectors.
	List(options metav1.ListOptions) (result *v1beta1.HelmReleaseList, err error)

	// Create takes the representation of a HelmRelease and creates it.
	// Returns the server's representation of the HelmRelease, and an error, if there is any.
	Create(release *v1beta1.HelmRelease) (result *v1beta1.HelmRelease, err error)

	// Update takes the representation of a HelmRelease and updates it.
	// Returns the server's representation of the HelmRelease, and an error, if there is any.
	Update(release *v1beta1.HelmRelease) (result *v1beta1.HelmRelease, err error)

	// Delete takes name of the HelmRelease and deletes it. Returns an error if one occurs.
	Delete(name string, options *metav1.DeleteOptions) error

	// DeleteCollection deletes a collection of objects
	DeleteCollection(options *metav1.DeleteOptions, listOpts metav1.ListOptions) error
}
