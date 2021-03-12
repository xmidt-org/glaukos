package parsers

import (
	"testing"
	"time"

	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
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
		message            wrp.Message
		expectedCount      map[string]float64
		expectedUnparsable float64
	}{
		{
			description: "Success",
			message: wrp.Message{
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
			message: wrp.Message{
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

			mp.Parse(queue.WrpWithTime{Message: tc.message, Beginning: time.Now()})
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
	messages := []wrp.Message{
		wrp.Message{
			Metadata: map[string]string{
				trustKey:     "1000",
				partnerIDKey: "random partner",
				bootTimeKey:  "1611700028",
				randomKey:    "random",
			},
		},
		wrp.Message{
			Metadata: map[string]string{
				trustKey:     "1000",
				partnerIDKey: "random partner",
			},
		},
		wrp.Message{
			Metadata: map[string]string{
				trustKey: "1000",
			},
		},
		wrp.Message{
			Metadata: map[string]string{},
		},
		wrp.Message{
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
		mp.Parse(queue.WrpWithTime{Message: msg, Beginning: time.Now()})
	}

	p.Assert(t, "metadata_keys", MetadataKeyLabel, trustKey)(xmetricstest.Value(3.0))
	p.Assert(t, "metadata_keys", MetadataKeyLabel, partnerIDKey)(xmetricstest.Value(2.0))
	p.Assert(t, "metadata_keys", MetadataKeyLabel, bootTimeKey)(xmetricstest.Value(1.0))
	p.Assert(t, "metadata_keys", MetadataKeyLabel, randomKey)(xmetricstest.Value(1.0))
	p.Assert(t, "unparsable_events", ParserLabel, metadataParserLabel, ReasonLabel, noMetadataFoundErr)(xmetricstest.Value(2.0))
}
