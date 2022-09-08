package parsers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/touchstone"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"
)

func TestBootTimeCalculator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	tests := []struct {
		description         string
		event               interpreter.Event
		expectedTimeElapsed float64
		expectedErr         error
	}{
		{
			description: "valid",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			expectedTimeElapsed: now.Sub(now.Add(-1 * time.Minute)).Seconds(),
		},
		{
			description: "no boot-time",
			event: interpreter.Event{
				Birthdate: now.UnixNano(),
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
		{
			description: "neg boot-time",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: "-1",
				},
				Birthdate: now.UnixNano(),
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
		{
			description: "no birthdate",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
		{
			description: "neg birthdate",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: -1,
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
		{
			description: "birthdate less than boot-time",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			calculator := BootDurationCalculator(zap.NewNop(), func(_ interpreter.Event, duration float64) {
				assert.Equal(tc.expectedTimeElapsed, duration)
			})
			err := calculator.Calculate([]interpreter.Event{}, tc.event)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestNewEventToCurrentCalculator(t *testing.T) {
	tests := []struct {
		description string
		expectedErr error
		logger      *zap.Logger
		successFunc func(currentEvent interpreter.Event, foundEvent interpreter.Event, duration float64)
		eventFinder Finder
	}{
		{
			description: "nil finder",
			logger:      zap.NewNop(),
			successFunc: func(_ interpreter.Event, _ interpreter.Event, _ float64) {},
			expectedErr: errMissingFinder,
		},
		{
			description: "missing success func and logger",
			logger:      zap.NewNop(),
			eventFinder: new(mockFinder),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			calculator, err := NewEventToCurrentCalculator(tc.eventFinder, tc.successFunc, tc.logger)
			assert.Equal(tc.expectedErr, err)
			if tc.expectedErr == nil {
				assert.NotNil(calculator.eventFinder)
				assert.NotNil(calculator.logger)
				assert.NotNil(calculator.successCallback)
			}
		})
	}
}

func TestEventToCurrentCalculator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	tests := []struct {
		description         string
		currentEvent        interpreter.Event
		finderEvent         interpreter.Event
		finderErr           error
		logger              *zap.Logger
		expectedTimeElapsed float64
		expectedErr         error
	}{
		{
			description: "success",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			logger:              zap.NewNop(),
			expectedTimeElapsed: now.Sub(now.Add(-1 * time.Minute)).Seconds(),
		},
		{
			description: "nil logger success",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			expectedTimeElapsed: now.Sub(now.Add(-1 * time.Minute)).Seconds(),
		},
		{
			description: "finder err",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderErr:   errors.New("test"),
			expectedErr: errEventNotFound,
		},
		{
			description: "current event missing birthdate",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			expectedErr: errCalculation,
		},
		{
			description: "found event missing birthdate",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			expectedErr: errCalculation,
		},
		{
			description: "negative time elapsed",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			expectedErr: errCalculation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			finder := new(mockFinder)
			finder.On("Find", mock.Anything, mock.Anything).Return(tc.finderEvent, tc.finderErr)
			calculator := EventToCurrentCalculator{
				logger:      tc.logger,
				eventFinder: finder,
				successCallback: func(_ interpreter.Event, _ interpreter.Event, duration float64) {
					assert.Equal(tc.expectedTimeElapsed, duration)
				},
			}
			err := calculator.Calculate([]interpreter.Event{}, tc.currentEvent)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

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
			testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(io.Discard, "", 0), prometheus.NewPedanticRegistry())

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

func TestCreateDurationCalculatorsHistogramErr(t *testing.T) {
	assert := assert.New(t)
	testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(io.Discard, "", 0), prometheus.NewPedanticRegistry())
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
