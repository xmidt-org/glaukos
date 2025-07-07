// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/gorilla/mux"
	"github.com/xmidt-org/touchstone"
	"github.com/xmidt-org/touchstone/touchhttp"
	"go.uber.org/fx"
)

type MetricsRoutesIn struct {
	fx.In
	Router       *mux.Router `name:"servers.metrics"`
	ServerBundle touchhttp.ServerBundle
	Handler      touchhttp.Handler
}

func BuildMetricsRoutes(in MetricsRoutesIn) {
	if in.Router != nil && in.Handler != nil {
		instrumenter, err := in.ServerBundle.NewInstrumenter("servers.metrics")(&touchstone.Factory{})
		if err != nil {
			return
		}
		in.Router.Handle("/metrics", instrumenter.Then(in.Handler)).Methods("GET")
	}
}
