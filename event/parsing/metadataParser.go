package parsing

import (
	"errors"

	"github.com/xmidt-org/wrp-go/v3"
)

const (
	MetadataKeyLabel = "metadata_key"

	metadataParserLabel = "metadata_parser"
	noMetadataFoundErr  = "no_metadata_found"
)

// MetadataParser parses messages coming in and counts the various metadata keys of each request.
type MetadataParser struct {
	Measures Measures
}

// Parse gathers metrics for each metadata key.
func (m MetadataParser) Parse(msg wrp.Message) error {
	if len(msg.Metadata) < 1 {
		m.Measures.UnparsableEventsCount.With(ParserLabel, metadataParserLabel, ReasonLabel, noMetadataFoundErr).Add(1.0)
		return errors.New("no metadata found")
	}
	for key := range msg.Metadata {
		m.Measures.MetadataFields.With(MetadataKeyLabel, key).Add(1.0)
	}
	return nil
}
