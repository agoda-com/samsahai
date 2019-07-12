package helmrelease

import (
	"time"

	"github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName("helm-release")

const ResourceName = "helmreleases"

type client struct {
	client    rest.Interface
	namespace string
}

func New(namespace string, config *rest.Config) internal.HelmReleaseClient {
	// create rest client
	restClient, err := rest.UnversionedRESTClientFor(getRESTconfig(config, &v1beta1.SchemeGroupVersion))
	if err != nil {
		logger.Error(err, "cannot create unversioned restclient")
		return nil
	}

	return &client{
		client:    restClient,
		namespace: namespace,
	}
}

// Get takes name of the helmrelease, and returns the corresponding HelmRelease object, and an error if there is any.
func (c *client) Get(name string, opts metav1.GetOptions) (result *v1beta1.HelmRelease, err error) {
	result = &v1beta1.HelmRelease{}
	err = c.client.
		Get().
		Namespace(c.namespace).
		Resource(ResourceName).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of HelmRelease that match those selectors.
func (c *client) List(opts metav1.ListOptions) (results *v1beta1.HelmReleaseList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	results = &v1beta1.HelmReleaseList{}
	err = c.client.
		Get().
		Namespace(c.namespace).
		Resource(ResourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(results)
	return
}

// Create takes the representation of a HelmRelease and creates it.
// Returns the server's representation of the HelmRelease, and an error, if there is any.
func (c *client) Create(release *v1beta1.HelmRelease) (result *v1beta1.HelmRelease, err error) {
	result = &v1beta1.HelmRelease{}
	err = c.client.Post().
		Namespace(c.namespace).
		Resource(ResourceName).
		Body(release).
		Do().
		Into(result)
	return
}

// Update takes the representation of a HelmRelease and updates it.
// Returns the server's representation of the HelmRelease, and an error, if there is any.
func (c *client) Update(release *v1beta1.HelmRelease) (result *v1beta1.HelmRelease, err error) {
	result = &v1beta1.HelmRelease{}
	err = c.client.Put().
		Namespace(c.namespace).
		Resource(ResourceName).
		Name(release.Name).
		Body(release).
		Do().
		Into(result)
	return
}

// Delete takes name of the HelmRelease and deletes it. Returns an error if one occurs.
func (c *client) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.namespace).
		Resource(ResourceName).
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *client) DeleteCollection(options *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.namespace).
		Resource(ResourceName).
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

func getRESTconfig(config *rest.Config, groupVersion *schema.GroupVersion) *rest.Config {
	cfg := *config
	cfg.ContentConfig.GroupVersion = groupVersion
	cfg.APIPath = "/apis"
	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	return &cfg
}
