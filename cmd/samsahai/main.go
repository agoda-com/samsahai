/*
Copyright 2019 Agoda DevOps Container.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	fluxv1beta1 "github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	httpSwagger "github.com/swaggo/http-swagger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	docs2 "github.com/agoda-com/samsahai/docs"
	s2h "github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/samsahai/activepromotion"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	s2hhttp "github.com/agoda-com/samsahai/internal/samsahai/webhook"
	"github.com/agoda-com/samsahai/internal/stablecomponent"
	"github.com/agoda-com/samsahai/internal/util"
	"github.com/agoda-com/samsahai/internal/util/random"
)

var (
	scheme = runtime.NewScheme()

	logger = s2hlog.S2HLog.WithName("cmd")

	cmd = &cobra.Command{
		Use:   "samsahai",
		Short: "Samsahai Controller",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			l := s2hlog.GetLogger(viper.GetBool(s2h.VKDebug))
			logf.SetLogger(l)
			s2hlog.SetLogger(l)
		},
	}
)

func init() {
	cobra.OnInitialize(util.InitViper)

	_ = clientgoscheme.AddToScheme(scheme)
	_ = s2hv1beta1.AddToScheme(scheme)
	_ = fluxv1beta1.SchemeBuilder.AddToScheme(scheme)
	//_ = appv1beta1.AddToScheme(scheme)

	cmd.PersistentFlags().Bool(s2h.VKDebug, false, "More debugging log.")

	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		log.Printf("viper cannot bind pflags: %+v\n", err)
	}

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(startCtrlCmd())
}

func main() {
	_ = cmd.Execute()
}

func startCtrlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start Staging Controller",
		Run: func(cmd *cobra.Command, args []string) {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				log.Fatalf("cannot bindpflags: %v", err)
			}

			namespace := viper.GetString(s2h.VKPodNamespace)
			httpServerPort := viper.GetString(s2h.VKServerHTTPPort)
			httpMetricPort := viper.GetString(s2h.VKMetricHTTPPort)

			//create metrics description
			exporter.RegisterMetrics()

			logger.Debug(fmt.Sprintf("running on: %s", namespace))
			// Get a config to talk to the apiserver
			logger.Info("setting up client for manager")
			cfg, err := config.GetConfig()
			if err != nil {
				logger.Error(err, "unable to set up client config")
				os.Exit(1)
			}

			// Create a new Cmd to provide shared dependencies and start components
			logger.Info("setting up manager")
			mgr, err := cr.NewManager(cfg, manager.Options{
				Scheme:             scheme,
				MetricsBindAddress: ":" + httpMetricPort,
			})
			if err != nil {
				logger.Error(err, "unable to set up overall controller manager")
				os.Exit(1)
			}

			logger.Info("setting up internal components")

			authToken := viper.GetString(s2h.VKS2HAuthToken)
			if authToken == "" {
				authToken = random.GenerateRandomString(32)
				logger.Debug("auth token not provided, generate new one", "auth-token", authToken)
			}

			configs := s2h.SamsahaiConfig{
				TeamcityURL: viper.GetString(s2h.VKTeamcityURL),
				SamsahaiURL: fmt.Sprintf("%s://%s.%s:%s",
					viper.GetString(s2h.VKS2HServiceScheme),
					viper.GetString(s2h.VKS2HServiceName),
					namespace,
					httpServerPort),
				SamsahaiExternalURL: viper.GetString(s2h.VKS2HExternalURL),
				SamsahaiImage:       viper.GetString(s2h.VKS2HImage),
				SamsahaiHTTPProxy:   viper.GetString(s2h.VKS2HHTTPProxy),
				SamsahaiHTTPSProxy:  viper.GetString(s2h.VKS2HHTTPSProxy),
				SamsahaiNoProxy:     viper.GetString(s2h.VKS2HNoProxy),
				ClusterDomain:       viper.GetString(s2h.VKClusterDomain),
				ActivePromotion: s2h.ActivePromotionConfig{
					Concurrences:     viper.GetInt(s2h.VKActivePromotionConcurrences),
					Timeout:          metav1.Duration{Duration: viper.GetDuration(s2h.VKActivePromotionTimeout)},
					DemotionTimeout:  metav1.Duration{Duration: viper.GetDuration(s2h.VKActivePromotionDemotionTimeout)},
					RollbackTimeout:  metav1.Duration{Duration: viper.GetDuration(s2h.VKActivePromotionRollbackTimeout)},
					TearDownDuration: metav1.Duration{Duration: viper.GetDuration(s2h.VKActivePromotionTearDownDuration)},
					MaxHistories:     viper.GetInt(s2h.VKActivePromotionMaxHistories),
				},
				SamsahaiCredential: s2h.SamsahaiCredential{
					InternalAuthToken: authToken,
					SlackToken:        viper.GetString(s2h.VKSlackToken),
					TeamcityUsername:  viper.GetString(s2h.VKTeamcityUsername),
					TeamcityPassword:  viper.GetString(s2h.VKTeamcityPassword),
				},
			}

			configPath := viper.GetString(s2h.VKS2HConfigPath)
			if err := loadFileConfiguration(configPath, &configs); err != nil {
				logger.Error(err, "cannot load file configuration", "path", configPath)
				os.Exit(1)
			}

			s2hCtrl := samsahai.New(mgr, namespace, configs)
			activepromotion.New(mgr, s2hCtrl, configs)
			stablecomponent.New(mgr, s2hCtrl)

			logger.Info("setup signal handler")
			stop := signals.SetupSignalHandler()

			// setup http server
			logger.Info("setup http server")
			mux := http.NewServeMux()
			mux.Handle(s2hCtrl.PathPrefix(), s2hCtrl)

			// start swagger doc
			logger.Info("setup swagger")
			swaggerDocs()
			mux.Handle("/swagger/", httpSwagger.Handler(
				httpSwagger.URL("/swagger/doc.json"),
				//The url pointing to API definition"
			))

			mux.Handle("/", s2hhttp.New(s2hCtrl))
			httpServer := http.Server{Handler: mux, Addr: ":" + httpServerPort}

			logger.Info("starting http server")
			go func() {
				if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
					logger.Error(err, "cannot start web server")
				}
			}()

			go s2hCtrl.Start(stop)

			logger.Info("starting manager")
			if err := mgr.Start(stop); err != nil {
				logger.Error(err, "unable to run the manager")
				os.Exit(1)
			}

			logger.Info("stopping controller")

			logger.Info("shutting down http server")
			if err := httpServer.Shutdown(context.TODO()); err != nil {
				logger.Error(err, "http server shutdown")
			}
		},
	}
	defaultImage := "quay.io/samsahai/samsahai:latest"
	if s2h.Version != "" {
		defaultImage = "quay.io/samsahai/samsahai:" + s2h.Version
	}
	cmd.Flags().String(s2h.VKPodNamespace, "default", "Namespace that the controller works on.")
	cmd.Flags().String(s2h.VKS2HConfigPath, "samsahai.yaml", "Samsahai configuration file path.")
	cmd.Flags().String(s2h.VKClusterDomain, "cluster.local", "Internal domain of the cluster.")
	cmd.Flags().String(s2h.VKServerHTTPPort, s2h.SamsahaiDefaultPort, "The port for http server to listens to.")
	cmd.Flags().String(s2h.VKMetricHTTPPort, "8081", "The port for prometheus metric to binds to.")
	cmd.Flags().String(s2h.VKS2HAuthToken, "<random>", "Samsahai server authentication token.")
	cmd.Flags().String(s2h.VKSlackToken, "", "Slack token for send notification if using slack.")
	cmd.Flags().String(s2h.VKS2HImage, defaultImage, "Docker image for running Staging.")
	cmd.Flags().String(s2h.VKS2HServiceScheme, "http", "Scheme to use for connecting to Samsahai.")
	cmd.Flags().String(s2h.VKS2HServiceName, "samsahai", "Service name for connecting to Samsahai.")
	cmd.Flags().String(s2h.VKS2HExternalURL, "http://localhost:8080", "External url for Samsahai.")
	cmd.Flags().String(s2h.VKS2HHTTPProxy, "", "http proxy for Samsahai.")
	cmd.Flags().String(s2h.VKS2HHTTPSProxy, "", "https proxy for Samsahai.")
	cmd.Flags().String(s2h.VKS2HNoProxy, "", "no proxy for Samsahai.")
	cmd.Flags().String(s2h.VKTeamcityURL, "", "Teamcity Base URL.")
	cmd.Flags().String(s2h.VKTeamcityUsername, "", "Teamcity Username.")
	cmd.Flags().String(s2h.VKTeamcityPassword, "", "Teamcity Password.")
	cmd.Flags().Int(s2h.VKActivePromotionConcurrences, 1, "Concurrent active promotions.")
	cmd.Flags().Duration(s2h.VKActivePromotionTimeout, 30*time.Minute, "Active promotion timeout.")
	cmd.Flags().Duration(s2h.VKActivePromotionDemotionTimeout, 3*time.Minute, "Active demotion timeout.")
	cmd.Flags().Duration(s2h.VKActivePromotionRollbackTimeout, 5*time.Minute,
		"Active promotion rollback timeout.")
	cmd.Flags().Duration(s2h.VKActivePromotionTearDownDuration, 20*time.Minute,
		"Previous active environment teardown duration.")
	cmd.Flags().Int(s2h.VKActivePromotionMaxHistories, 7,
		"Max stored active promotion histories per team.")

	return cmd
}

func getVersion() string {
	return fmt.Sprintf("v%s (commit:%s)", s2h.Version, s2h.GitCommit)
}

func versionCmd() *cobra.Command {
	isShortVersion := false
	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "show version",
		Run: func(cmd *cobra.Command, args []string) {
			if isShortVersion {
				fmt.Println(s2h.Version)
				return
			}
			fmt.Println("samsahai-ctrl version:", getVersion())
		},
	}
	cmd.Flags().BoolVarP(&isShortVersion, "short", "s", false, "print only version")

	return cmd
}

func loadFileConfiguration(filePath string, s2hConfig *s2h.SamsahaiConfig) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	pwd, _ := os.Getwd()
	filePath = path.Join(pwd, filePath)
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrapf(err, "cannot read config from file %s", filePath)
	}

	if err := yaml.Unmarshal(b, s2hConfig); err != nil {
		return errors.Wrapf(err, "cannot unmarshal configuration of samsahai itself, file path: %s", filePath)
	}

	s2hConfig.ConfigDirPath = filepath.Dir(filePath)

	return nil
}

func swaggerDocs() {
	docs2.SwaggerInfo.Title = "Swagger Samsahai API"
	docs2.SwaggerInfo.Description = "Samsahai public API."
	docs2.SwaggerInfo.Version = "1.0"

	// @license.name Apache 2.0
	// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
}
