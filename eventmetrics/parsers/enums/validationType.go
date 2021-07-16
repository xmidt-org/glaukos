package enums

import "strings"

type ValidationType int

const (
	UnknownValidation ValidationType = iota
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
	UnknownValidationStr   = "unknown"
	BootTimeValidationStr  = "boot-time-validation"
	BirthdateValidationStr = "birthdate-validation"
	MinBootDurationStr     = "min-boot-duration"
	BirthdateAlignmentStr  = "birthdate-alignment"
	ValidEventTypeStr      = "valid-event-type"
	ConsistentDeviceIDStr  = "consistent-device-id"
	ConsistentMetadataStr  = "consistent-metadata"
	UniqueTransactionIDStr = "unique-transaction-id"
	SessionOnlineStr       = "session-online"
	SessionOfflineStr      = "session-offline"
	EventOrderStr          = "event-order"
)

func (v ValidationType) String() string {
	switch v {
	case BootTimeValidation:
		return BootTimeValidationStr
	case BirthdateValidation:
		return BirthdateValidationStr
	case MinBootDuration:
		return MinBootDurationStr
	case BirthdateAlignment:
		return BirthdateAlignmentStr
	case ValidEventType:
		return ValidEventTypeStr
	case ConsistentDeviceID:
		return ConsistentDeviceIDStr
	case ConsistentMetadata:
		return ConsistentMetadataStr
	case UniqueTransactionID:
		return UniqueTransactionIDStr
	case SessionOnline:
		return SessionOnlineStr
	case SessionOffline:
		return SessionOfflineStr
	case EventOrder:
		return EventOrderStr
	}

	return UnknownValidationStr
}

// ParseValidationType returns the ValidationType enum when given a string.
func ParseValidationType(s string) ValidationType {
	s = strings.ToLower(s)

	switch s {
	case BootTimeValidationStr:
		return BootTimeValidation
	case BirthdateValidationStr:
		return BirthdateValidation
	case MinBootDurationStr:
		return MinBootDuration
	case BirthdateAlignmentStr:
		return BirthdateAlignment
	case ValidEventTypeStr:
		return ValidEventType
	case ConsistentDeviceIDStr:
		return ConsistentDeviceID
	case ConsistentMetadataStr:
		return ConsistentMetadata
	case UniqueTransactionIDStr:
		return UniqueTransactionID
	case SessionOnlineStr:
		return SessionOnline
	case SessionOfflineStr:
		return SessionOffline
	case EventOrderStr:
		return EventOrder
	}

	return UnknownValidation
}
