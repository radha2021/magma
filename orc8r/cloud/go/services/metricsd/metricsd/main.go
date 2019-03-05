/*
Copyright (c) Facebook, Inc. and its affiliates.
All rights reserved.

This source code is licensed under the BSD-style license found in the
LICENSE file in the root directory of this source tree.
*/

package main

import (
	"flag"
	"log"
	"os"
	"time"

	"magma/orc8r/cloud/go/orc8r"
	"magma/orc8r/cloud/go/pluginimpl"
	"magma/orc8r/cloud/go/protos"
	"magma/orc8r/cloud/go/service"
	"magma/orc8r/cloud/go/services/metricsd"
	"magma/orc8r/cloud/go/services/metricsd/collection"
	"magma/orc8r/cloud/go/services/metricsd/servicers"

	"github.com/prometheus/client_model/go"
)

const (
	CloudMetricsCollectInterval = time.Second * 20
)

// Target variable for flag
var profileArg string

func main() {
	flag.StringVar(
		&profileArg,
		"profile",
		pluginimpl.ProfileNameController,
		"Name of the profile to start metricsd with. Determines which collectors and exporters will run. "+
			"Valid options are sys, controller, and prometheus. Default value is controller",
	)

	srv, err := service.NewOrchestratorService(orc8r.ModuleName, metricsd.ServiceName)
	if err != nil {
		log.Fatalf("Error creating service: %s", err)
	}
	controllerServer := servicers.NewMetricsControllerServer()
	protos.RegisterMetricsControllerServer(srv.GrpcServer, controllerServer)
	srv.GrpcServer.RegisterService(protos.GetLegacyMetricsdDesc(), controllerServer)

	selectedProfile, err := metricsd.GetMetricsProfile(profileArg)
	if err != nil {
		log.Fatalf("Error loading metrics profile: %s", err)
	}

	// Initialize metrics gatherer
	metricsChannel := make(chan *io_prometheus_client.MetricFamily)
	gatherer, err := collection.NewMetricsGatherer(selectedProfile.Collectors, CloudMetricsCollectInterval, metricsChannel)
	if err != nil {
		log.Fatalf("Error initializing MetricsGatherer: %s", err)
	}

	// Kick off gatherer and exporters
	go controllerServer.ConsumeCloudMetrics(metricsChannel, os.Getenv("HOST_NAME"))
	gatherer.Run()
	for _, exporter := range selectedProfile.Exporters {
		controllerServer.RegisterExporter(exporter)
		exporter.Start()
	}

	err = srv.Run()
	if err != nil {
		log.Fatalf("Error running service: %s", err)
	}
}
