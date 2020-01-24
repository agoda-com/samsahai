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
	"log"
	"net/http"
	"os"
	"strings"

	fluxv1beta1 "github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	s2h "github.com/agoda-com/samsahai/internal"
	s2hconfig "github.com/agoda-com/samsahai/internal/config"
	desiredctrl "github.com/agoda-com/samsahai/internal/desiredcomponent"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	stagingctrl "github.com/agoda-com/samsahai/internal/staging"
	"github.com/agoda-com/samsahai/internal/util"
)

var (
	scheme = runtime.NewScheme()

	logger = s2hlog.Log.WithName("cmd")

	cmd = &cobra.Command{
		Use:   "staging",
		Short: "Staging Controller",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			l := zap.New(func(o *zap.Options) {
				o.Development = viper.GetBool(s2h.VKDebug)
			})
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				log.Fatalf("cannot bindpflags: %v\n", err)
			}

			requiredConfig := []string{s2h.VKS2HTeamName, s2h.VKS2HServerURL, s2h.VKS2HAuthToken, s2h.VKPodNamespace}
			for _, flag := range requiredConfig {
				if _, err := checkRequiredConfig(flag); err != nil {
					return err
				}
			}

			namespace := viper.GetString(s2h.VKPodNamespace)
			httpServerPort := viper.GetString(s2h.VKServerHTTPPort)
			httpMetricPort := viper.GetString(s2h.VKMetricHTTPPort)
			teamName := viper.GetString(s2h.VKS2HTeamName)

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
			mgr, err := manager.New(cfg, manager.Options{
				Scheme:             scheme,
				MetricsBindAddress: ":" + httpMetricPort,
				Namespace:          namespace,
			})
			if err != nil {
				logger.Error(err, "unable to set up overall controller manager")
				os.Exit(1)
			}

			// Setup Scheme for all resources
			logger.Info("setting up scheme")

			// Create runtime client
			runtimeClient, err := client.New(cfg, client.Options{Scheme: scheme})
			if err != nil {
				logger.Error(err, "cannot create unversioned restclient")
				os.Exit(1)
			}

			logger.Info("setting up internal components")

			//isShuttingDown := false

			samsahaiClient := rpc.NewRPCProtobufClient(viper.GetString(s2h.VKS2HServerURL), &http.Client{})
			configManager, err := s2hconfig.NewWithSamsahaiClient(samsahaiClient, teamName, viper.GetString(s2h.VKS2HAuthToken))
			if err != nil {
				logger.Error(err, "cannot load configuration from server")
				os.Exit(1)
			}
			queueCtrl := queue.New(namespace, runtimeClient)
			desiredctrl.New(teamName, mgr, queueCtrl)
			authToken := viper.GetString(s2h.VKS2HAuthToken)
			tcBaseURL := viper.GetString(s2h.VKTeamcityURL)
			tcUsername := viper.GetString(s2h.VKTeamcityUsername)
			tcPassword := viper.GetString(s2h.VKTeamcityPassword)
			stagingCtrl := stagingctrl.NewController(teamName, namespace, authToken, samsahaiClient, mgr, queueCtrl, configManager, tcBaseURL, tcUsername, tcPassword)

			logger.Info("setup signal handler")
			stop := signals.SetupSignalHandler()

			logger.Info("starting controller")
			go stagingCtrl.Start(stop)

			logger.Info("initializing http routes")

			httpServer := http.Server{Handler: stagingCtrl, Addr: ":" + httpServerPort}

			logger.Info("starting http server")
			go func() {
				if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
					logger.Error(err, "cannot start web server")
				}
			}()

			logger.Info("starting manager")
			if err := mgr.Start(stop); err != nil {
				logger.Error(err, "unable to run the manager")
				os.Exit(1)
			}

			logger.Info("shutting down http server")
			if err := httpServer.Shutdown(context.Background()); err != nil {
				logger.Error(err, "http server shutdown")
			}

			return nil
		},
	}

	cmd.Flags().String(s2h.VKPodNamespace, "", "Namespace that the controller works on.")
	cmd.Flags().String(s2h.VKS2HTeamName, "", "Samsahai Team Name.")
	cmd.Flags().String(s2h.VKS2HServerURL, "", "Samsahai server endpoint.")
	cmd.Flags().String(s2h.VKS2HAuthToken, "", "Samsahai server authentication token.")
	cmd.Flags().String(s2h.VKTeamcityURL, "", "Teamcity api base url.")
	cmd.Flags().String(s2h.VKTeamcityUsername, "", "Teamcity username.")
	cmd.Flags().String(s2h.VKTeamcityPassword, "", "Teamcity password.")
	cmd.Flags().String(s2h.VKServerHTTPPort, "8090", "The port for http server to listens to.")
	cmd.Flags().String(s2h.VKMetricHTTPPort, "8091", "The port for prometheus metric to binds to.")

	return cmd
}

func checkRequiredConfig(name string) (string, error) {
	v := viper.GetString(name)
	if v == "" {
		return "", fmt.Errorf("config '%s' is required", strings.Replace(strings.ToUpper(name), "-", "_", -1))
	}
	return v, nil
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
			fmt.Println("staging-ctrl version:", fmt.Sprintf("v%s (commit:%s)", s2h.Version, s2h.GitCommit))
		},
	}
	cmd.Flags().BoolVarP(&isShortVersion, "short", "s", false, "print only version")

	return cmd
}
