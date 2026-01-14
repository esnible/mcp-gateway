/*
Package main is that starting point for the mcp controller
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This package contains the main of the mcp controller
*/
package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/mcp-gateway/api/v1alpha1"
	"github.com/Kuadrant/mcp-gateway/internal/controller"
)

func init() {
	runtime.Must(v1alpha1.AddToScheme(scheme.Scheme))
	runtime.Must(gatewayv1.Install(scheme.Scheme))
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	fmt.Println("Controller starting (health: :8081, metrics: :8082)...")
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8082"},
		LeaderElection:         false,
		HealthProbeBindAddress: ":8081",
	})
	if err != nil {
		panic("unable to start manager : " + err.Error())
	}

	if err = (&controller.MCPReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		APIReader: mgr.GetAPIReader(),
	}).SetupWithManager(mgr); err != nil {
		panic("unable to start manager : " + err.Error())
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		panic("unable to start manager : " + err.Error())
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		panic("unable to start manager : " + err.Error())
	}

	fmt.Println("Starting controller manager...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		panic("unable to start manager : " + err.Error())
	}
}
