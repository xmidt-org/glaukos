package parsers

import (
	"testing"

	"github.com/xmidt-org/glaukos/message"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
)

func TestParse(t *testing.T) {
	const (
		trustKey     = "trust"
		partnerIDKey = "partner-id"
		bootTimeKey  = "boot-time"
		randomKey    = "random"
	)

	p := xmetricstest.NewProvider(&xmetrics.Options{})

	tests := []struct {
		description        string
		message            message.Event
		expectedCount      map[string]float64
		expectedUnparsable float64
	}{
		{
			description: "Success",
			message: message.Event{
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
			message: message.Event{
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
				Measures: m,
			}

			mp.Parse(tc.message)
			for key, val := range tc.expectedCount {
				p.Assert(t, "metadata_keys", MetadataKeyLabel, key)(xmetricstest.Value(val))
			}

			p.Assert(t, "unparsable_events", ParserLabel, metadataParserLabel, ReasonLabel, noMetadataFoundErr)(xmetricstest.Value(tc.expectedUnparsable))
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

	p := xmetricstest.NewProvider(&xmetrics.Options{})
	messages := []message.Event{
		message.Event{
			Metadata: map[string]string{
				trustKey:     "1000",
				partnerIDKey: "random partner",
				bootTimeKey:  "1611700028",
				randomKey:    "random",
			},
		},
		message.Event{
			Metadata: map[string]string{
				trustKey:     "1000",
				partnerIDKey: "random partner",
			},
		},
		message.Event{
			Metadata: map[string]string{
				trustKey: "1000",
			},
		},
		message.Event{
			Metadata: map[string]string{},
		},
		message.Event{
			Metadata: map[string]string{},
		},
	}

	m := Measures{
		MetadataFields:        p.NewCounter("metadata_keys"),
		UnparsableEventsCount: p.NewCounter("unparsable_events"),
	}
	mp := MetadataParser{
		Measures: m,
	}

	for _, msg := range messages {
		mp.Parse(msg)
	}

	p.Assert(t, "metadata_keys", MetadataKeyLabel, trustKey)(xmetricstest.Value(3.0))
	p.Assert(t, "metadata_keys", MetadataKeyLabel, partnerIDKey)(xmetricstest.Value(2.0))
	p.Assert(t, "metadata_keys", MetadataKeyLabel, bootTimeKey)(xmetricstest.Value(1.0))
	p.Assert(t, "metadata_keys", MetadataKeyLabel, randomKey)(xmetricstest.Value(1.0))
	p.Assert(t, "unparsable_events", ParserLabel, metadataParserLabel, ReasonLabel, noMetadataFoundErr)(xmetricstest.Value(2.0))
}