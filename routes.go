/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"github.com/gorilla/mux"
	"github.com/xmidt-org/themis/xhealth"
	"github.com/xmidt-org/touchstone/touchhttp"
	"go.uber.org/fx"
)

type MetricsRoutesIn struct {
	fx.In
	Router  *mux.Router `name:"servers.metrics"`
	Handler touchhttp.Handler
}

func BuildMetricsRoutes(in MetricsRoutesIn) {
	if in.Router != nil && in.Handler != nil {
		in.Router.Handle("/metrics", in.Handler).Methods("GET")
	}
}

type HealthRoutesIn struct {
	fx.In
	Router  *mux.Router `name:"servers.health"`
	Handler xhealth.Handler
}

func BuildHealthRoutes(in HealthRoutesIn) {
	if in.Router != nil && in.Handler != nil {
		in.Router.Handle("/health", in.Handler).Methods("GET")
	}
}
