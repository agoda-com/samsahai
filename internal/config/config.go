package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	EnvPrefix = "HTF"
)

// InitViper init viper
func InitViper() {
	log := log.Log.WithName("config")

	wd, err := os.Getwd()
	if err != nil {
		log.Error(err, "cannot os.getwd")
		os.Exit(1)
		return
	}

	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	cfgFile := filepath.Join(wd, "samsahai.yaml")
	log.V(1).Info(fmt.Sprintf("config: %s", cfgFile))
	viper.SetConfigType("yaml")
	viper.SetConfigFile(cfgFile)

	if _, err := os.Stat(cfgFile); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			log.Error(err, "viper reading initialization failed")
		}
	}
}

func GetRESTConfg(config *rest.Config, groupVersion *schema.GroupVersion) *rest.Config {
	cfg := *config
	cfg.ContentConfig.GroupVersion = groupVersion
	cfg.APIPath = "/apis"
	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	return &cfg
}
