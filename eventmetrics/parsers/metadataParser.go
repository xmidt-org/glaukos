/**
 *  Copyright (c) 2021  Comcast Cable Communications Management, LLC
 */

package parsers

import (
	"errors"
	"strings"

	"github.com/xmidt-org/glaukos/message"
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
func (m MetadataParser) Parse(event message.Event) error {
	if len(event.Metadata) < 1 {
		m.Measures.UnparsableEventsCount.With(ParserLabel, metadataParserLabel, ReasonLabel, noMetadataFoundErr).Add(1.0)
		return errors.New("no metadata found")
	}
	for key := range event.Metadata {
		trimmedKey := strings.Trim(key, "/")
		m.Measures.MetadataFields.With(MetadataKeyLabel, trimmedKey).Add(1.0)
	}
	return nil
}
