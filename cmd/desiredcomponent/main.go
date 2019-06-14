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
	stdlog "log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	envv1beta1 "github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
	samsahaiconfig "github.com/agoda-com/samsahai/internal/config"
	desiredctrl "github.com/agoda-com/samsahai/internal/desiredcomponent"
	desiredhttp "github.com/agoda-com/samsahai/internal/desiredcomponent/http"
	"github.com/agoda-com/samsahai/internal/queue"
)

func main() {
	samsahaiconfig.InitViper()

	pflag.String("pod-namespace", "default", "Namespace that the controller works on.")
	pflag.String("webhook-addr", ":8080", "The address the webhook endpoint binds to.")
	pflag.String("metrics-addr", ":8081", "The address the webhook endpoint binds to.")
	pflag.Bool("debug", false, "More debugging log.")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		stdlog.Fatalf("cannot bindpflags: %v", err)
	}
	namespace := viper.GetString("pod-namespace")
	webhookAddr := viper.GetString("webhook-addr")
	metricsAddr := viper.GetString("metrics-addr")

	logf.SetLogger(logf.ZapLogger(viper.GetBool("debug")))
	log := logf.Log.WithName("cmd")

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: metricsAddr,
		Namespace:          namespace,
	})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := envv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add apis to scheme")
		os.Exit(1)
	}
	// register types at the scheme builder
	if err := v1beta1.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "cannot addtoscheme")
		os.Exit(1)
	}

	// Create REST client
	restCfg := samsahaiconfig.GetRESTConfg(cfg, &v1beta1.SchemeGroupVersion)
	restClient, err := rest.UnversionedRESTClientFor(restCfg)
	if err != nil {
		log.Error(err, "cannot create unversioned restclient")
		os.Exit(1)
	}

	log.Info("setting up internal components")

	isShuttingDown := false

	router := httprouter.New()
	srv := http.Server{Handler: router, Addr: webhookAddr}

	queueCtrl := queue.New(namespace, cfg)
	desiredComponentCtrl := desiredctrl.NewWithClient(namespace, mgr, restClient, queueCtrl)

	desiredComponentCtrl.Start()

	log.Info("initialzing http routes")
	desiredhttp.New(router, desiredComponentCtrl)
	router.GET("/healthz", func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		if isShuttingDown {
			res.WriteHeader(http.StatusInternalServerError)
			_, _ = res.Write([]byte("shutting down"))
			return
		}
		res.WriteHeader(http.StatusOK)
		_, _ = res.Write([]byte("ok"))
	})

	log.Info("setup signal handler")
	stop := signals.SetupSignalHandler()

	log.Info("starting http server")
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Error(err, "cannot start web server")
		}
	}()

	log.Info("starting manager")
	if err := mgr.Start(stop); err != nil {
		log.Error(err, "unable to run the manager")
		os.Exit(1)
	}

	isShuttingDown = true

	log.Info("stopping controller")
	desiredComponentCtrl.Stop()

	log.Info("shutting down http server")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error(err, "http server shutdown")
	}
}
