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

// provideMetrics builds the application metrics and makes them available to the container
// func provideMetrics() fx.Option {
// 	return fx.Provide(
// 		touchstone.CounterVec(
// 			prometheus.CounterOpts{
// 				Name: "server_request_count",
// 				Help: "total incoming HTTP requests",
// 			},
// 			touchhttp.CodeLabel,
// 			touchhttp.MethodLabel,
// 			touchhttp.ServerLabel,
// 		),
// 		touchstone.HistogramVec(
// 			prometheus.HistogramOpts{
// 				Name: "server_request_duration_ms",
// 				Help: "tracks incoming request durations in ms",
// 			},
// 			touchhttp.CodeLabel,
// 			touchhttp.MethodLabel,
// 			touchhttp.ServerLabel,
// 		),
// 		touchstone.GaugeVec(
// 			prometheus.GaugeOpts{
// 				Name: "server_requests_in_flight",
// 				Help: "tracks the current number of incoming requests being processed",
// 			},
// 			touchhttp.ServerLabel,
// 		),
// 	)
// }
