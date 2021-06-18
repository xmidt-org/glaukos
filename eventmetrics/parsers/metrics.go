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

package parsers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/touchstone"
	"go.uber.org/fx"
)

const (
	parserLabel    = "parser_type"
	reasonLabel    = "reason"
	eventTypeLabel = "event_type"
)

// Measures tracks the various event-related metrics.
type Measures struct {
	fx.In
	MetadataFields              *prometheus.CounterVec `name:"metadata_fields"`
	TotalUnparsableEvents       *prometheus.CounterVec `name:"total_unparsable_events"`
	RebootUnparsableCount       *prometheus.CounterVec `name:"reboot_unparsable_count"`
	RebootEventErrors           *prometheus.CounterVec `name:"reboot_event_errors"`
	RebootCycleErrors           *prometheus.CounterVec `name:"reboot_cycle_errors"`
	BootToManageableHistogram   prometheus.ObserverVec `name:"boot_to_manageable"`
	RebootToManageableHistogram prometheus.ObserverVec `name:"reboot_to_manageable"`
}

// ProvideEventMetrics builds the event-related metrics and makes them available to the container.
func ProvideEventMetrics() fx.Option {
	return fx.Options(
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "metadata_fields",
				Help: "the metadata fields coming from each event received",
			},
			metadataKeyLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "total_unparsable_events",
				Help: "events that are unparsable, labelled by the parser name",
			},
			parserLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "reboot_unparsable_count",
				Help: "events that are not able to be fully processed, labelled by reason",
			},
			firmwareLabel,
			hardwareLabel,
			reasonLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "reboot_event_errors",
				Help: "individual event errors",
			},
			firmwareLabel,
			hardwareLabel,
			reasonLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "reboot_cycle_errors",
				Help: "cycle errors",
			},
			reasonLabel,
		),
		touchstone.HistogramVec(
			prometheus.HistogramOpts{
				Name:    "boot_to_manageable",
				Help:    "time elapsed between a device booting and fully-manageable event",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			firmwareLabel,
			hardwareLabel,
			rebootReasonLabel,
		),
		touchstone.HistogramVec(
			prometheus.HistogramOpts{
				Name:    "reboot_to_manageable",
				Help:    "time elapsed between a reboot-pending and fully-manageable event",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			firmwareLabel,
			hardwareLabel,
			rebootReasonLabel,
		),
	)
}
