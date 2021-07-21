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
	"github.com/xmidt-org/bascule/basculechecks"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/touchstone"
	"go.uber.org/fx"
)

const (
	parserLabel = "parser_type"
	reasonLabel = "reason"
)

// Measures tracks the various event-related metrics.
type Measures struct {
	fx.In
	MetadataFields              *prometheus.CounterVec `name:"metadata_fields"`
	TotalUnparsableCount        *prometheus.CounterVec `name:"total_unparsable_count"`
	RebootUnparsableCount       *prometheus.CounterVec `name:"reboot_unparsable_count"`
	EventErrorTags              *prometheus.CounterVec `name:"event_errors"`
	BootCycleErrorTags          *prometheus.CounterVec `name:"boot_cycle_errors"`
	RebootCycleErrorTags        *prometheus.CounterVec `name:"reboot_cycle_errors"`
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
				Name: "total_unparsable_count",
				Help: "events that are unparsable, labelled by the parser name",
			},
			parserLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "reboot_unparsable_count",
				Help: "events that are not able to be fully processed, labelled by reason",
			},
			firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "event_errors",
				Help: "individual event errors",
			},
			firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "boot_cycle_errors",
				Help: "cycle errors",
			},
			reasonLabel, partnerIDLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "reboot_cycle_errors",
				Help: "cycle errors",
			},
			reasonLabel, partnerIDLabel,
		),
		touchstone.HistogramVec(
			prometheus.HistogramOpts{
				Name:    "boot_to_manageable",
				Help:    "time elapsed between a device booting and fully-manageable event",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			firmwareLabel, hardwareLabel, rebootReasonLabel,
		),
		touchstone.HistogramVec(
			prometheus.HistogramOpts{
				Name:    "reboot_to_manageable",
				Help:    "time elapsed between a reboot-pending and fully-manageable event",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			firmwareLabel, hardwareLabel, rebootReasonLabel,
		),
	)
}

// AddMetadata adds to the metadata parser.
func (m *Measures) AddMetadata(metadataKey string) {
	if m.MetadataFields != nil {
		m.MetadataFields.With(prometheus.Labels{metadataKeyLabel: metadataKey}).Add(1.0)
	}
}

// AddTotalUnparsable adds to the total unparsable counter.
func (m *Measures) AddTotalUnparsable(parserName string) {
	if m.TotalUnparsableCount != nil {
		m.TotalUnparsableCount.With(prometheus.Labels{parserLabel: parserName}).Add(1.0)
	}
}

// AddRebootUnparsable adds to the RebootUnparsable counter.
func (m *Measures) AddRebootUnparsable(reason string, event interpreter.Event) {
	if m.RebootUnparsableCount != nil {
		hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
		partner := basculechecks.DeterminePartnerMetric(event.PartnerIDs)
		m.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, partnerIDLabel: partner, reasonLabel: reason}).Add(1.0)
	}
}

// AddEventError adds a error tag to the event error counter.
func AddEventError(counter *prometheus.CounterVec, event interpreter.Event, errorTag string) {
	if counter != nil {
		hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
		partner := basculechecks.DeterminePartnerMetric(event.PartnerIDs)
		counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, partnerIDLabel: partner, reasonLabel: errorTag}).Add(1.0)
	}
}

// AddCycleError adds a cycle error tag to the cycle error counter.
func AddCycleError(counter *prometheus.CounterVec, event interpreter.Event, errorTag string) {
	if counter != nil {
		partner := basculechecks.DeterminePartnerMetric(event.PartnerIDs)
		counter.With(prometheus.Labels{partnerIDLabel: partner, reasonLabel: errorTag}).Add(1.0)
	}
}

// AddDuration adds the duration to the specific histogram.
func AddDuration(histogram prometheus.ObserverVec, duration float64, event interpreter.Event) {
	if histogram != nil {
		labels := getTimeElapsedHistogramLabels(event)
		histogram.With(labels).Observe(duration)
	}
}

// get hardware and firmware values from event metadata, returning false if either one or both are not found
func getHardwareFirmware(event interpreter.Event) (hardwareVal string, firmwareVal string, found bool) {
	hardwareVal, hardwareFound := event.GetMetadataValue(hardwareMetadataKey)
	firmwareVal, firmwareFound := event.GetMetadataValue(firmwareMetadataKey)

	found = true
	if !hardwareFound {
		hardwareVal = unknownLabelValue
		found = false
	}
	if !firmwareFound {
		firmwareVal = unknownLabelValue
		found = false
	}

	return
}

// grab relevant information from event metadata and return prometheus labels
func getTimeElapsedHistogramLabels(event interpreter.Event) prometheus.Labels {
	hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
	rebootReason, reasonFound := event.GetMetadataValue(rebootReasonMetadataKey)
	if !reasonFound {
		rebootReason = unknownLabelValue
	}

	return prometheus.Labels{
		hardwareLabel:     hardwareVal,
		firmwareLabel:     firmwareVal,
		rebootReasonLabel: rebootReason,
	}
}
