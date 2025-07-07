// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package enums

import "strings"

type CycleType int

const (
	BootTime CycleType = iota
	Reboot
)

var (
	cycleTypeUnmarshal = map[string]CycleType{
		"boot-time": BootTime,
		"reboot":    Reboot,
	}
)

// ParseCycleType returns the CycleType enum when given a string.
func ParseCycleType(cycleType string) CycleType {
	cycleType = strings.ToLower(cycleType)
	if value, ok := cycleTypeUnmarshal[cycleType]; ok {
		return value
	}
	return BootTime
}
