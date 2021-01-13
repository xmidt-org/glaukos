/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	KeyLabel      = "metadata_key"
	FirmwareLabel = "firmware"
	HardwareLabel = "hardware"
)

type MetricsIn struct {
	fx.In
	MetadataFields    metrics.Counter   `name:"metadata_fields"`
	BootTimeHistogram metrics.Histogram `name:"boot_time_duration"`
}

// ProvideMetrics builds the application metrics and makes them available to the container.
func ProvideMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "metadata_fields",
				Help: "the metadata fields coming from each event received",
			},
			KeyLabel,
		),
		xmetrics.ProvideHistogram(
			prometheus.HistogramOpts{
				Name:    "boot_time_duration",
				Help:    "tracks boot time durations in s",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600},
			},
			FirmwareLabel,
			HardwareLabel,
		),
	)
}
