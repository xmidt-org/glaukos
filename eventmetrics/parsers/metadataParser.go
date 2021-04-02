/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
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
	metadataKeyLabel = "metadata_key"

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
		m.measures.UnparsableEventsCount.With(parserLabel, m.name, reasonLabel, noMetadataFoundErr).Add(1.0)
		level.Error(m.logger).Log(xlog.ErrorKey(), errNoMetadata)
		return
	}

	for key := range event.Metadata {
		trimmedKey := strings.Trim(key, "/")
		m.measures.MetadataFields.With(metadataKeyLabel, trimmedKey).Add(1.0)
	}
}

// Name returns the name of the parser. Implements the Parser interface.
func (m *MetadataParser) Name() string {
	return m.name
}
