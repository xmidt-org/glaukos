// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package enums

import "strings"

type CycleValidationType int

const (
	UnknownCycleValidation CycleValidationType = iota
	ConsistentMetadataValidation
	UniqueTransactionIDValidation
	SessionOnlineValidation
	SessionOfflineValidation
	EventOrderValidation
)

const (
	UnknownCycleValidationStr        = "unknown"
	ConsistentMetadataValidationStr  = "consistent-metadata"
	UniqueTransactionIDValidationStr = "unique-transaction-id"
	SessionOnlineValidationStr       = "session-online"
	SessionOfflineValidationStr      = "session-offline"
	EventOrderValidationStr          = "event-order"
)

func (v *CycleValidationType) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	case ConsistentMetadataValidationStr:
		*v = ConsistentMetadataValidation
	case UniqueTransactionIDValidationStr:
		*v = UniqueTransactionIDValidation
	case SessionOnlineValidationStr:
		*v = SessionOnlineValidation
	case SessionOfflineValidationStr:
		*v = SessionOfflineValidation
	case EventOrderValidationStr:
		*v = EventOrderValidation
	default:
		*v = UnknownCycleValidation
	}

	return nil
}

func (v CycleValidationType) String() string {
	switch v {
	case ConsistentMetadataValidation:
		return ConsistentMetadataValidationStr
	case UniqueTransactionIDValidation:
		return UniqueTransactionIDValidationStr
	case SessionOnlineValidation:
		return SessionOnlineValidationStr
	case SessionOfflineValidation:
		return SessionOfflineValidationStr
	case EventOrderValidation:
		return EventOrderValidationStr
	}

	return UnknownCycleValidationStr
}
