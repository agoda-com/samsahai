package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

// InitViper init viper
func InitViper() {
	logger := s2hlog.Log.WithName("config")

	wd, err := os.Getwd()
	if err != nil {
		logger.Error(err, "cannot os.getwd")
		os.Exit(1)
		return
	}

	//viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	cfgFile := filepath.Join(wd, "samsahai.yaml")
	logger.Debug(fmt.Sprintf("config: %s", cfgFile))
	viper.SetConfigType("yaml")
	viper.SetConfigFile(cfgFile)

	if _, err := os.Stat(cfgFile); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			logger.Error(err, "viper reading initialization failed")
		}
	}
}
