// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"
)

func TestName(t *testing.T) {
	metadataParser := MetadataParser{name: "test_parser"}
	assert.Equal(t, "test_parser", metadataParser.Name())
}

func TestParse(t *testing.T) {
	const (
		trustKey     = "trust"
		partnerIDKey = "partner-id"
		bootTimeKey  = "boot-time"
		randomKey    = "random"
	)
	logger := zap.NewNop()

	tests := []struct {
		description        string
		message            interpreter.Event
		expectedUnparsable float64
	}{
		{
			description: "Success",
			message: interpreter.Event{
				Metadata: map[string]string{
					trustKey:     "1000",
					partnerIDKey: "random partner",
					bootTimeKey:  "1611700028",
					randomKey:    "random",
				},
			},
		},
		{
			description: "No metadata",
			message: interpreter.Event{
				Metadata: map[string]string{},
			},
			expectedUnparsable: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				assert                = assert.New(t)
				expectedRegistry      = prometheus.NewPedanticRegistry()
				actualRegistry        = prometheus.NewPedanticRegistry()
				actualMetadataCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "testMetadataCounter",
						Help: "testMetadataCounter",
					},
					[]string{metadataKeyLabel},
				)
				actualUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "testUnparsableCounter",
						Help: "testUnparsableCounter",
					},
					[]string{parserLabel, reasonLabel},
				)

				expectedMetadataCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "testMetadataCounter",
						Help: "testMetadataCounter",
					},
					[]string{metadataKeyLabel},
				)
				expectedUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "testUnparsableCounter",
						Help: "testUnparsableCounter",
					},
					[]string{parserLabel, reasonLabel},
				)
			)

			expectedRegistry.Register(expectedMetadataCounter)
			expectedRegistry.Register(expectedUnparsableCounter)
			actualRegistry.Register(actualMetadataCounter)
			actualRegistry.Register(actualUnparsableCounter)

			m := Measures{
				MetadataFields:       actualMetadataCounter,
				TotalUnparsableCount: actualUnparsableCounter,
			}
			mp := MetadataParser{
				measures: m,
				logger:   logger,
				name:     "metadata_parser",
			}

			mp.Parse(tc.message)
			for key := range tc.message.Metadata {
				expectedMetadataCounter.With(prometheus.Labels{metadataKeyLabel: key}).Inc()
			}

			if tc.expectedUnparsable > 0 {
				expectedUnparsableCounter.With(prometheus.Labels{parserLabel: "metadata_parser", reasonLabel: noMetadataFoundErr}).Add(tc.expectedUnparsable)
			}

			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.GatherAndCompare(actualRegistry))
		})
	}
}

func TestMultipleParse(t *testing.T) {
	const (
		trustKey     = "trust"
		partnerIDKey = "partner-id"
		bootTimeKey  = "boot-time"
		randomKey    = "random"
	)

	var (
		assert                = assert.New(t)
		expectedRegistry      = prometheus.NewPedanticRegistry()
		actualRegistry        = prometheus.NewPedanticRegistry()
		actualMetadataCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testMetadataCounter",
				Help: "testMetadataCounter",
			},
			[]string{metadataKeyLabel},
		)
		actualUnparsableCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testUnparsableCounter",
				Help: "testUnparsableCounter",
			},
			[]string{parserLabel, reasonLabel},
		)

		expectedMetadataCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testMetadataCounter",
				Help: "testMetadataCounter",
			},
			[]string{metadataKeyLabel},
		)
		expectedUnparsableCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testUnparsableCounter",
				Help: "testUnparsableCounter",
			},
			[]string{parserLabel, reasonLabel},
		)
	)

	expectedRegistry.Register(expectedMetadataCounter)
	expectedRegistry.Register(expectedUnparsableCounter)
	actualRegistry.Register(actualMetadataCounter)
	actualRegistry.Register(actualUnparsableCounter)

	messages := []interpreter.Event{
		interpreter.Event{
			Metadata: map[string]string{
				trustKey:     "1000",
				partnerIDKey: "random partner",
				bootTimeKey:  "1611700028",
				randomKey:    "random",
			},
		},
		interpreter.Event{
			Metadata: map[string]string{
				trustKey:     "1000",
				partnerIDKey: "random partner",
			},
		},
		interpreter.Event{
			Metadata: map[string]string{
				trustKey: "1000",
			},
		},
		interpreter.Event{
			Metadata: map[string]string{},
		},
		interpreter.Event{
			Metadata: map[string]string{},
		},
	}

	m := Measures{
		MetadataFields:       actualMetadataCounter,
		TotalUnparsableCount: actualUnparsableCounter,
	}
	mp := MetadataParser{
		measures: m,
		logger:   zap.NewNop(),
		name:     "metadata_parser",
	}

	for _, msg := range messages {
		mp.Parse(msg)
		for key := range msg.Metadata {
			expectedMetadataCounter.WithLabelValues(key).Inc()
		}

		if (len(msg.Metadata)) == 0 {
			expectedUnparsableCounter.WithLabelValues("metadata_parser", noMetadataFoundErr).Inc()
		}
	}

	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))
}
