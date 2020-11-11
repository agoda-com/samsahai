package util

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log

func MustParseYAMLtoRuntimeObject(data []byte) (cliObj runtime.Object, kind *schema.GroupVersionKind) {
	codecFactory := serializer.NewCodecFactory(scheme.Scheme)
	decode := codecFactory.UniversalDeserializer().Decode
	obj, kind, err := decode(data, nil, nil)
	if err != nil {
		logger.Error(err, "cannot decode YAML to runtime.Object")
		os.Exit(1)
	}
	return obj, kind
}
