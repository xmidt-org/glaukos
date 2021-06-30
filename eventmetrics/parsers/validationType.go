package parsers

import "strings"

type ValidationType int

const (
	unknown ValidationType = iota
	bootTimeValidation
	birthdateValidation
	minBootDuration
	birthdateAlignment
	consistentMetadata
	uniqueTransactionID
	sessionOnline
	sessionOffline
	validEventType
	consistentDeviceID
)

const (
	unknownStr             = "unknown"
	bootTimeValidationStr  = "boot-time-validation"
	birthdateValidationStr = "birthdate-validation"
	minBootDurationStr     = "min-boot-duration"
	birthdateAlignmentStr  = "birthdate-alignment"
	consistentMetadataStr  = "consistent-metadata"
	uniqueTransactionIDStr = "unique-transaction-id"
	sessionOnlineStr       = "session-online"
	sessionOfflineStr      = "session-offline"
	validEventTypeStr      = "valid-event-type"
	consistentDeviceIDStr  = "consistent-device-id"
)

func (v ValidationType) String() string {
	switch v {
	case bootTimeValidation:
		return bootTimeValidationStr
	case birthdateValidation:
		return birthdateValidationStr
	case minBootDuration:
		return minBootDurationStr
	case birthdateAlignment:
		return birthdateAlignmentStr
	case consistentMetadata:
		return consistentMetadataStr
	case uniqueTransactionID:
		return uniqueTransactionIDStr
	case sessionOnline:
		return sessionOnlineStr
	case sessionOffline:
		return sessionOfflineStr
	case validEventType:
		return validEventTypeStr
	case consistentDeviceID:
		return consistentDeviceIDStr
	}

	return unknownStr
}

func ParseValidationType(s string) ValidationType {
	s = strings.ToLower(s)

	switch s {
	case bootTimeValidationStr:
		return bootTimeValidation
	case birthdateValidationStr:
		return birthdateValidation
	case minBootDurationStr:
		return minBootDuration
	case birthdateAlignmentStr:
		return birthdateAlignment
	case consistentMetadataStr:
		return consistentMetadata
	case uniqueTransactionIDStr:
		return uniqueTransactionID
	case sessionOnlineStr:
		return sessionOnline
	case sessionOfflineStr:
		return sessionOffline
	case validEventTypeStr:
		return validEventType
	case consistentDeviceIDStr:
		return consistentDeviceID
	}

	return unknown
}
