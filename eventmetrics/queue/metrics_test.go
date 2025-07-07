// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package queue

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/touchstone/touchtest"
)

func TestTimeTracker(t *testing.T) {
	tracker := &timeTracker{
		TimeInMemory: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "testHistogram",
				Help:    "testHistogram",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
		),
	}

	expectedHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "testHistogram",
			Help:    "testHistogram",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
	)
	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()

	expectedRegistry.Register(expectedHistogram)
	actualRegistry.Register(tracker.TimeInMemory.(prometheus.Collector))

	durations := []time.Duration{3 * time.Second, 250 * time.Millisecond, 2 * time.Millisecond, 300 * time.Millisecond, 6 * time.Minute, 2 * time.Hour}
	for _, duration := range durations {
		tracker.TrackTime(duration)
		expectedHistogram.Observe(duration.Seconds())
	}

	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(t, testAssert.CollectAndCompare(tracker.TimeInMemory.(prometheus.Collector)))

}
