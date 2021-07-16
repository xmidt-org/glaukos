package parsers

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/glaukos/eventmetrics/parsers/enums"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/touchstone/touchtest"

	"github.com/stretchr/testify/assert"
)

func TestCreateBootDurationCallback(t *testing.T) {
	const (
		hwVal        = "hw"
		fwVal        = "fw"
		rebootReason = "reboot"
	)

	assert := assert.New(t)
	m := Measures{
		BootToManageableHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bootHistogram",
				Help:    "bootHistogram",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
		),
	}

	expectedHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bootHistogram",
			Help:    "bootHistogram",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	currentEvent := interpreter.Event{
		Metadata: map[string]string{
			hardwareMetadataKey:     hwVal,
			firmwareMetadataKey:     fwVal,
			rebootReasonMetadataKey: rebootReason,
		},
	}

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedHistogram)
	actualRegistry.Register(m.BootToManageableHistogram)
	callback := createBootDurationCallback(m)
	callback(currentEvent, 5.0)
	expectedHistogram.WithLabelValues(fwVal, hwVal, rebootReason).Observe(5.0)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))
}

func TestCreateRebootToManageableCallback(t *testing.T) {
	const (
		hwVal        = "hw"
		fwVal        = "fw"
		rebootReason = "reboot"
	)

	assert := assert.New(t)
	m := Measures{
		RebootToManageableHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rebootHistogram",
				Help:    "rebootHistogram",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
		),
	}

	expectedHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rebootHistogram",
			Help:    "rebootHistogram",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	currentEvent := interpreter.Event{
		Metadata: map[string]string{
			hardwareMetadataKey:     hwVal,
			firmwareMetadataKey:     fwVal,
			rebootReasonMetadataKey: rebootReason,
		},
	}

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedHistogram)
	actualRegistry.Register(m.RebootToManageableHistogram)
	callback := createRebootToManageableCallback(m)
	callback(currentEvent, interpreter.Event{}, 5.0)
	expectedHistogram.WithLabelValues(fwVal, hwVal, rebootReason).Observe(5.0)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))
}

func TestCreateEventValidator(t *testing.T) {
	tests := []struct {
		key         string
		expectedErr error
	}{
		{
			key:         "abcdefg",
			expectedErr: errNonExistentKey,
		},
		{key: enums.BootTimeValidationStr},
		{key: enums.BirthdateValidationStr},
		{key: enums.MinBootDurationStr},
		{key: enums.BirthdateAlignmentStr},
		{key: enums.ValidEventTypeStr},
		{key: enums.ConsistentDeviceIDStr},
		{
			key:         enums.ConsistentMetadataStr,
			expectedErr: errWrongEventValidatorKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			assert := assert.New(t)
			config := EventValidationConfig{
				Key: tc.key,
			}
			validator, err := createEventValidator(config)
			if tc.expectedErr != nil {
				assert.Nil(validator)
			}
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestCreateCycleValidator(t *testing.T) {
	tests := []struct {
		key         string
		expectedErr error
	}{
		{
			key:         "abcdefg",
			expectedErr: errNonExistentKey,
		},
		{key: enums.ConsistentMetadataStr},
		{key: enums.UniqueTransactionIDStr},
		{key: enums.SessionOnlineStr},
		{key: enums.SessionOfflineStr},
		{key: enums.EventOrderStr},
		{
			key:         enums.BootTimeValidationStr,
			expectedErr: errWrongCycleValidatorKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			assert := assert.New(t)
			config := CycleValidationConfig{
				Key: tc.key,
			}
			validator, err := createCycleValidator(config)
			if tc.expectedErr != nil {
				assert.Nil(validator)
			}
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestCreateCycleValidators(t *testing.T) {
	tests := []struct {
		description string
		cycleType   enums.CycleType
		configs     []CycleValidationConfig
		expectedLen int
		expectedErr error
	}{
		{
			description: "boot-time cycle type",
			cycleType:   enums.BootTime,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 2,
		},
		{
			description: "reboot cycle type",
			cycleType:   enums.Reboot,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 1,
		},
		{
			description: "wrong validation key",
			cycleType:   enums.Reboot,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       "some-validation",
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 0,
			expectedErr: errNonExistentKey,
		},
		{
			description: "not cycle validation key",
			cycleType:   enums.Reboot,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       "boot-time-validation",
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataStr,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 0,
			expectedErr: errWrongCycleValidatorKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			validator, err := createCycleValidators(tc.configs, tc.cycleType)
			if validator != nil {
				validators := validator.(history.CycleValidators)
				assert.Equal(tc.expectedLen, len(validators))
			}
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestCheckTimeValidations(t *testing.T) {
	tests := []struct {
		description    string
		config         EventValidationConfig
		expectedConfig EventValidationConfig
	}{
		{
			description: "no changes",
			config: EventValidationConfig{
				BootTimeValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				BirthdateValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				MinBootDuration:            5 * time.Second,
				BirthdateAlignmentDuration: 2 * time.Minute,
			},
			expectedConfig: EventValidationConfig{
				BootTimeValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				BirthdateValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				MinBootDuration:            5 * time.Second,
				BirthdateAlignmentDuration: 2 * time.Minute,
			},
		},
		{
			description: "defaults",
			config:      EventValidationConfig{},
			expectedConfig: EventValidationConfig{
				BootTimeValidator: TimeValidationConfig{
					ValidFrom: defaultValidFrom,
					ValidTo:   defaultValidTo,
				},
				BirthdateValidator: TimeValidationConfig{
					ValidFrom: defaultValidFrom,
					ValidTo:   defaultValidTo,
				},
				MinBootDuration:            defaultMinBootDuration,
				BirthdateAlignmentDuration: defaultBirthdateAlignmentDuration,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			resultingConfig := checkTimeValidations(tc.config)
			assert.Equal(tc.expectedConfig, resultingConfig)
		})
	}
}
