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
