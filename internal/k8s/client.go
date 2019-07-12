package k8s

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

type SamsahaiResourceType string

const (
	ActivePromotions         SamsahaiResourceType = "activepromotions"
	ActivePromotionHistories SamsahaiResourceType = "activepromotionhistories"
	Teams                    SamsahaiResourceType = "teams"
	DesiredComponents        SamsahaiResourceType = "desiredcomponents"
	Queues                   SamsahaiResourceType = "queues"
	StableComponents         SamsahaiResourceType = "stablecomponents"
	QueueHistories           SamsahaiResourceType = "queuehistories"
)

// NewRESTClient returns rest.Interface from client-go for interact with Samsahai CRDs,
// also registered the CRDs to kubernetes/scheme
func NewRESTClient(cfg *rest.Config) (client rest.Interface, err error) {
	// create rest client
	return rest.UnversionedRESTClientFor(getSamsahaiRESTConfig(cfg))
}

func getSamsahaiRESTConfig(config *rest.Config) *rest.Config {
	cfg := *config
	cfg.ContentConfig.GroupVersion = &s2hv1beta1.SchemeGroupVersion
	cfg.APIPath = "/apis"
	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	if cfg.UserAgent == "" {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return &cfg
}

// DeleteCollection deletes a collection of `resource`.
func DeleteCollection(
	c rest.Interface,
	namespace string,
	resource SamsahaiResourceType,
	deleteOpts *metav1.DeleteOptions,
	listOpts *metav1.ListOptions,
) error {
	var timeout time.Duration
	if listOpts == nil {
		listOpts = &metav1.ListOptions{}
	}
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.Delete().
		Resource(string(resource)).
		Namespace(namespace).
		VersionedParams(listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(deleteOpts).
		Do().
		Error()
}

// DeleteAllQueues deletes all of Queues in the namespace.
func DeleteAllQueues(c rest.Interface, namespace string) error {
	return DeleteCollection(c, namespace, Queues, nil, nil)
}

// DeleteAllStableComponents deletes all of StableComponents in the namespace.
func DeleteAllStableComponents(c rest.Interface, namespace string) error {
	return DeleteCollection(c, namespace, StableComponents, nil, nil)
}

// DeleteAllQueueHistories deletes all of QueueHistories in the namespace.
func DeleteAllQueueHistories(c rest.Interface, namespace string) error {
	return DeleteCollection(c, namespace, QueueHistories, nil, nil)
}

// DeleteAllActivePromotions deletes all of ActivePromotions with specific labels.
func DeleteAllActivePromotions(c rest.Interface, selectors map[string]string) error {
	listOpt := &metav1.ListOptions{LabelSelector: labels.SelectorFromSet(selectors).String()}
	return DeleteCollection(c, "", ActivePromotions, nil, listOpt)
}

// DeleteAllActivePromotionHistories deletes all of ActivePromotionHistories with specific labels.
func DeleteAllActivePromotionHistories(c rest.Interface, selectors map[string]string) error {
	listOpt := &metav1.ListOptions{LabelSelector: labels.SelectorFromSet(selectors).String()}
	return DeleteCollection(c, "", ActivePromotionHistories, nil, listOpt)
}

// DeleteAllTeams deletes all of ActivePromotions with specific labels.
func DeleteAllTeams(c rest.Interface, selectors map[string]string) error {
	listOpt := &metav1.ListOptions{LabelSelector: labels.SelectorFromSet(selectors).String()}
	return DeleteCollection(c, "", Teams, nil, listOpt)
}
