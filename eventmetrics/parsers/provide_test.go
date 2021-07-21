package parsers

import (
	"errors"
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/glaukos/eventmetrics/parsers/enums"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/touchstone"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"
)

func TestCreateDurationCalculators(t *testing.T) {
	tests := []struct {
		description string
		configs     []TimeElapsedConfig
		expectedErr error
	}{
		{
			description: "success",
			configs: []TimeElapsedConfig{
				TimeElapsedConfig{
					Name:        "test",
					SessionType: "current",
					EventType:   "test-event-type",
				},
				TimeElapsedConfig{
					Name:        "test1",
					SessionType: "previous",
					EventType:   "test-event-type2",
				},
			},
		},
		{
			description: "duplicate Name",
			configs: []TimeElapsedConfig{
				TimeElapsedConfig{
					Name:        "test",
					SessionType: "current",
					EventType:   "test-event-type",
				},
				TimeElapsedConfig{
					Name:        "test",
					SessionType: "previous",
					EventType:   "test-event-type2",
				},
			},
			expectedErr: errNewHistogram,
		},
		{
			description: "blank name",
			configs: []TimeElapsedConfig{
				TimeElapsedConfig{
					Name:        "test",
					SessionType: "current",
					EventType:   "test-event-type",
				},
				TimeElapsedConfig{
					Name:        "",
					SessionType: "previous",
					EventType:   "test-event-type2",
				},
			},
			expectedErr: errBlankHistogramName,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())

			testMeasures := Measures{TimeElapsedHistograms: make(map[string]prometheus.ObserverVec)}
			durationCalculators, err := createDurationCalculators(testFactory, tc.configs, testMeasures, RebootLoggerIn{Logger: zap.NewNop()})

			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr))
			} else {
				assert.NotNil(durationCalculators)
				assert.Equal(len(tc.configs), len(durationCalculators))
				for _, config := range tc.configs {
					histogram, found := testMeasures.TimeElapsedHistograms[config.Name]
					assert.True(found)
					assert.NotNil(histogram)
				}
			}
		})
	}
}

func TestCreateCalculatorsHistogramErr(t *testing.T) {
	assert := assert.New(t)
	testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())
	testMeasures := Measures{TimeElapsedHistograms: make(map[string]prometheus.ObserverVec)}
	testName := "test_hist"
	config := TimeElapsedConfig{
		Name: testName,
	}
	options := prometheus.HistogramOpts{
		Name:    testName,
		Help:    "test_help",
		Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
	}

	testMeasures.addTimeElapsedHistogram(testFactory, options)
	durationCalculators, err := createDurationCalculators(testFactory, []TimeElapsedConfig{config}, testMeasures, RebootLoggerIn{Logger: zap.NewNop()})
	assert.True(errors.Is(err, errNewHistogram))
	assert.Nil(durationCalculators)
}

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
	callback, err := createBootDurationCallback(m)
	assert.Nil(err)
	callback(currentEvent, 5.0)
	expectedHistogram.WithLabelValues(fwVal, hwVal, rebootReason).Observe(5.0)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))

	nilCallback, err := createBootDurationCallback(Measures{})
	assert.Nil(nilCallback)
	assert.Equal(errNilBootHistogram, err)
}

func TestCreateTimeElapsedCallback(t *testing.T) {
	const (
		hwVal        = "hw"
		fwVal        = "fw"
		rebootReason = "reboot"
		histogramKey = "test_histogram"
	)

	assert := assert.New(t)
	actualHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rebootHistogram",
			Help:    "rebootHistogram",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	expectedHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rebootHistogram",
			Help:    "rebootHistogram",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	m := Measures{
		TimeElapsedHistograms: map[string]prometheus.ObserverVec{
			histogramKey: actualHistogram,
		},
	}

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
	actualRegistry.Register(actualHistogram)
	callback, err := createTimeElapsedCallback(m, histogramKey)
	assert.Nil(err)
	callback(currentEvent, interpreter.Event{}, 5.0)
	expectedHistogram.WithLabelValues(fwVal, hwVal, rebootReason).Observe(5.0)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))

	nilCallback, err := createTimeElapsedCallback(Measures{}, histogramKey)
	assert.Nil(nilCallback)
	assert.Equal(errNilHistogram, err)
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
