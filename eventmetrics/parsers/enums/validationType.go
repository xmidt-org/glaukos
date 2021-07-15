package enums

import "strings"

type ValidationType int

const (
	Unknown ValidationType = iota
	BootTimeValidation
	BirthdateValidation
	MinBootDuration
	BirthdateAlignment
	ConsistentMetadata
	UniqueTransactionID
	SessionOnline
	SessionOffline
	ValidEventType
	ConsistentDeviceID
	EventOrder
)

const (
	unknownStr             = "unknown"
	bootTimeValidationStr  = "boot-time-validation"
	birthdateValidationStr = "birthdate-validation"
	minBootDurationStr     = "min-boot-duration"
	birthdateAlignmentStr  = "birthdate-alignment"
	validEventTypeStr      = "valid-event-type"
	consistentDeviceIDStr  = "consistent-device-id"
	consistentMetadataStr  = "consistent-metadata"
	uniqueTransactionIDStr = "unique-transaction-id"
	sessionOnlineStr       = "session-online"
	sessionOfflineStr      = "session-offline"
	eventOrderStr          = "event-order"
)

func (v ValidationType) String() string {
	switch v {
	case BootTimeValidation:
		return bootTimeValidationStr
	case BirthdateValidation:
		return birthdateValidationStr
	case MinBootDuration:
		return minBootDurationStr
	case BirthdateAlignment:
		return birthdateAlignmentStr
	case ValidEventType:
		return validEventTypeStr
	case ConsistentDeviceID:
		return consistentDeviceIDStr
	case ConsistentMetadata:
		return consistentMetadataStr
	case UniqueTransactionID:
		return uniqueTransactionIDStr
	case SessionOnline:
		return sessionOnlineStr
	case SessionOffline:
		return sessionOfflineStr
	case EventOrder:
		return eventOrderStr
	}

	return unknownStr
}

func ParseValidationType(s string) ValidationType {
	s = strings.ToLower(s)

	switch s {
	case bootTimeValidationStr:
		return BootTimeValidation
	case birthdateValidationStr:
		return BirthdateValidation
	case minBootDurationStr:
		return MinBootDuration
	case birthdateAlignmentStr:
		return BirthdateAlignment
	case validEventTypeStr:
		return ValidEventType
	case consistentDeviceIDStr:
		return ConsistentDeviceID
	case consistentMetadataStr:
		return ConsistentMetadata
	case uniqueTransactionIDStr:
		return UniqueTransactionID
	case sessionOnlineStr:
		return SessionOnline
	case sessionOfflineStr:
		return SessionOffline
	case eventOrderStr:
		return EventOrder
	}

	return Unknown
}
