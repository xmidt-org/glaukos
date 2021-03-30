package parsers

import (
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
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
	logger := log.NewNopLogger()
	p := xmetricstest.NewProvider(&xmetrics.Options{})

	tests := []struct {
		description        string
		message            interpreter.Event
		expectedCount      map[string]float64
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
			expectedCount: map[string]float64{
				trustKey:     1,
				partnerIDKey: 1,
				bootTimeKey:  1,
				randomKey:    1,
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
			m := Measures{
				MetadataFields:        p.NewCounter("metadata_keys"),
				UnparsableEventsCount: p.NewCounter("unparsable_events"),
			}
			mp := MetadataParser{
				measures: m,
				logger:   logger,
				name:     "metadata_parser",
			}

			mp.Parse(tc.message)
			for key, val := range tc.expectedCount {
				p.Assert(t, "metadata_keys", metadataKeyLabel, key)(xmetricstest.Value(val))
			}

			p.Assert(t, "unparsable_events", parserLabel, "metadata_parser", reasonLabel, noMetadataFoundErr)(xmetricstest.Value(tc.expectedUnparsable))
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

	logger := log.NewNopLogger()
	p := xmetricstest.NewProvider(&xmetrics.Options{})
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
		MetadataFields:        p.NewCounter("metadata_keys"),
		UnparsableEventsCount: p.NewCounter("unparsable_events"),
	}
	mp := MetadataParser{
		measures: m,
		logger:   logger,
		name:     "metadata_parser",
	}

	for _, msg := range messages {
		mp.Parse(msg)
	}

	p.Assert(t, "metadata_keys", metadataKeyLabel, trustKey)(xmetricstest.Value(3.0))
	p.Assert(t, "metadata_keys", metadataKeyLabel, partnerIDKey)(xmetricstest.Value(2.0))
	p.Assert(t, "metadata_keys", metadataKeyLabel, bootTimeKey)(xmetricstest.Value(1.0))
	p.Assert(t, "metadata_keys", metadataKeyLabel, randomKey)(xmetricstest.Value(1.0))
	p.Assert(t, "unparsable_events", parserLabel, "metadata_parser", reasonLabel, noMetadataFoundErr)(xmetricstest.Value(2.0))
}
