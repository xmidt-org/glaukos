// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package enums

import "strings"

// SessionType is an enum to determine which session an event should be searched from
type SessionType int

const (
	Previous SessionType = iota
	Current
)

var (
	sessionTypeUnmarshal = map[string]SessionType{
		"previous": Previous,
		"current":  Current,
	}
)

// ParseSessionType returns the SessionType enum when given a string.
func ParseSessionType(session string) SessionType {
	session = strings.ToLower(session)
	if value, ok := sessionTypeUnmarshal[session]; ok {
		return value
	}
	return Previous
}
