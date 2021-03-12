/**
 *  Copyright (c) 2021  Comcast Cable Communications Management, LLC
 */

package parsers

import (
	"errors"
	"strings"

	"github.com/xmidt-org/glaukos/event/queue"
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
func (m MetadataParser) Parse(wrpWithTime queue.WrpWithTime) error {
	if len(wrpWithTime.Message.Metadata) < 1 {
		m.Measures.UnparsableEventsCount.With(ParserLabel, metadataParserLabel, ReasonLabel, noMetadataFoundErr).Add(1.0)
		return errors.New("no metadata found")
	}
	for key := range wrpWithTime.Message.Metadata {
		trimmedKey := strings.Trim(key, "/")
		m.Measures.MetadataFields.With(MetadataKeyLabel, trimmedKey).Add(1.0)
	}
	return nil
}
