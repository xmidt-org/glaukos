/**
 *  Copyright (c) 2021  Comcast Cable Communications Management, LLC
 */

package parsers

import (
	"errors"
	"strings"

	"github.com/xmidt-org/themis/xlog"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/xmidt-org/interpreter"
)

const (
	MetadataKeyLabel = "metadata_key"

	noMetadataFoundErr = "no_metadata_found"
)

var (
	errNoMetadata = errors.New("no metadata found")
)

// MetadataParser parses messages coming in and counts the various metadata keys of each request.
type MetadataParser struct {
	measures Measures
	name     string
	logger   log.Logger
}

// Parse gathers metrics for each metadata key.
func (m *MetadataParser) Parse(event interpreter.Event) {
	if len(event.Metadata) < 1 {
		m.measures.UnparsableEventsCount.With(ParserLabel, m.name, ReasonLabel, noMetadataFoundErr).Add(1.0)
		level.Error(m.logger).Log(xlog.ErrorKey(), errNoMetadata)
		return
	}

	for key := range event.Metadata {
		trimmedKey := strings.Trim(key, "/")
		m.measures.MetadataFields.With(MetadataKeyLabel, trimmedKey).Add(1.0)
	}
}

func (m *MetadataParser) Name() string {
	return m.name
}
