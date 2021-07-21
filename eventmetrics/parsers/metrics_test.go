package parsers

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/touchstone"
	"github.com/xmidt-org/touchstone/touchtest"
)

func TestAddMetadata(t *testing.T) {
	m := Measures{
		MetadataFields: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "metadataCounter",
				Help: "metadataCounter",
			},
			[]string{metadataKeyLabel},
		),
	}

	metadataKey := "testKey"
	m.AddMetadata(metadataKey)
	assert.Equal(t, 1.0, testutil.ToFloat64(m.MetadataFields))

	m = Measures{}
	m.AddMetadata(metadataKey)

}

func TestAddTotalUnparsable(t *testing.T) {
	m := Measures{
		TotalUnparsableCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "totalUnparsable",
				Help: "totalUnparsable",
			},
			[]string{parserLabel},
		),
	}

	parserName := "testParser"
	m.AddTotalUnparsable(parserName)
	assert.Equal(t, 1.0, testutil.ToFloat64(m.TotalUnparsableCount))

	m = Measures{}
	m.AddTotalUnparsable(parserName)
}

func TestAddRebootUnparsable(t *testing.T) {
	m := Measures{
		RebootUnparsableCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rebootUnparsable",
				Help: "rebootUnparsable",
			},
			[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
		),
	}

	expectedRebootUnparsableCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rebootUnparsable",
			Help: "rebootUnparsable",
		},
		[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
	)

	testEvent := interpreter.Event{
		Metadata: map[string]string{
			firmwareMetadataKey:     "fw",
			hardwareMetadataKey:     "hw",
			rebootReasonMetadataKey: "reboot",
		},
		PartnerIDs: []string{
			"partner",
		},
	}
	reason := "testReason"

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedRebootUnparsableCount)
	actualRegistry.Register(m.RebootUnparsableCount)

	expectedRebootUnparsableCount.With(prometheus.Labels{firmwareLabel: "fw",
		hardwareLabel: "hw", partnerIDLabel: "partner", reasonLabel: reason}).Add(1.0)
	m.AddRebootUnparsable(reason, testEvent)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(t, testAssert.GatherAndCompare(actualRegistry))

	m = Measures{}
	m.AddRebootUnparsable(reason, testEvent)
}

func TestAddEventError(t *testing.T) {
	eventErrorTags := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventErrors",
			Help: "eventErrors",
		},
		[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
	)

	expectedEventErrorTags := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventErrors",
			Help: "eventErrors",
		},
		[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
	)

	testEvent := interpreter.Event{
		Metadata: map[string]string{
			firmwareMetadataKey:     "fw",
			hardwareMetadataKey:     "hw",
			rebootReasonMetadataKey: "reboot",
		},
		PartnerIDs: []string{
			"partner",
		},
	}
	reason := "testReason"

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedEventErrorTags)
	actualRegistry.Register(eventErrorTags)

	expectedEventErrorTags.With(prometheus.Labels{firmwareLabel: "fw",
		hardwareLabel: "hw", partnerIDLabel: "partner", reasonLabel: reason}).Add(1.0)
	AddEventError(eventErrorTags, testEvent, reason)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(t, testAssert.GatherAndCompare(actualRegistry))

	AddEventError(nil, testEvent, reason)
}

func TestAddCycleError(t *testing.T) {
	cycleErrorCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cycleErrors",
			Help: "cycleErrors",
		},
		[]string{partnerIDLabel, reasonLabel},
	)

	expectedCycleErrorTags := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cycleErrors",
			Help: "cycleErrors",
		},
		[]string{partnerIDLabel, reasonLabel},
	)

	testEvent := interpreter.Event{
		PartnerIDs: []string{
			"partner",
		},
	}
	reason := "testReason"

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedCycleErrorTags)
	actualRegistry.Register(cycleErrorCount)

	expectedCycleErrorTags.With(prometheus.Labels{partnerIDLabel: "partner", reasonLabel: reason}).Add(1.0)
	AddCycleError(cycleErrorCount, testEvent, reason)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(t, testAssert.GatherAndCompare(actualRegistry))

	AddCycleError(nil, testEvent, reason)
}

func TestAddDuration(t *testing.T) {
	actualHist := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rebootHistogram",
			Help:    "rebootHistogram",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	expectedHist := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rebootHistogram",
			Help:    "rebootHistogram",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	testEvent := interpreter.Event{
		Metadata: map[string]string{
			firmwareMetadataKey:     "fw",
			hardwareMetadataKey:     "hw",
			rebootReasonMetadataKey: "reboot",
		},
		PartnerIDs: []string{
			"partner",
		},
	}

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedHist)
	actualRegistry.Register(actualHist)

	expectedHist.WithLabelValues("fw", "hw", "reboot").Observe(5.0)
	AddDuration(actualHist, 5.0, testEvent)

	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(t, testAssert.GatherAndCompare(actualRegistry))

	AddDuration(nil, 5.0, testEvent)
}

func TestGetHardwareFirmware(t *testing.T) {
	tests := []struct {
		description   string
		event         interpreter.Event
		expectedHwVal string
		expectedFwVal string
		expectedFound bool
	}{
		{
			description: "all exists",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey: "testHw",
					firmwareMetadataKey: "testFw",
				},
			},
			expectedHwVal: "testHw",
			expectedFwVal: "testFw",
			expectedFound: true,
		},
		{
			description: "missing hw",
			event: interpreter.Event{
				Metadata: map[string]string{
					firmwareMetadataKey: "testFw",
				},
			},
			expectedHwVal: unknownLabelValue,
			expectedFwVal: "testFw",
			expectedFound: false,
		},
		{
			description: "missing fw",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey: "testHw",
				},
			},
			expectedHwVal: "testHw",
			expectedFwVal: unknownLabelValue,
			expectedFound: false,
		},
		{
			description: "missing both",
			event: interpreter.Event{
				Metadata: map[string]string{},
			},
			expectedHwVal: unknownLabelValue,
			expectedFwVal: unknownLabelValue,
			expectedFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			hwVal, fwVal, found := getHardwareFirmware(tc.event)
			assert.Equal(tc.expectedHwVal, hwVal)
			assert.Equal(tc.expectedFwVal, fwVal)
			assert.Equal(tc.expectedFound, found)
		})
	}
}

func TestGetTimeElapsedHistogramLabels(t *testing.T) {
	tests := []struct {
		description    string
		event          interpreter.Event
		expectedLabels prometheus.Labels
	}{
		{
			description: "all exists",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey:     "testHw",
					firmwareMetadataKey:     "testFw",
					rebootReasonMetadataKey: "testReboot",
				},
			},
			expectedLabels: prometheus.Labels{
				hardwareLabel:     "testHw",
				firmwareLabel:     "testFw",
				rebootReasonLabel: "testReboot",
			},
		},
		{
			description: "missing reboot reason",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey: "testHw",
					firmwareMetadataKey: "testFw",
				},
			},
			expectedLabels: prometheus.Labels{
				hardwareLabel:     "testHw",
				firmwareLabel:     "testFw",
				rebootReasonLabel: unknownLabelValue,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			labels := getTimeElapsedHistogramLabels(tc.event)
			assert.Equal(tc.expectedLabels, labels)
		})
	}
}

func TestAddTimeElapsedHistogramSuccess(t *testing.T) {
	tests := []struct {
		description string
		name        string
		labelNames  []string
		measures    Measures
		expectedErr error
	}{
		{
			description: "Success",
			name:        "test_parser",
			labelNames:  []string{"key1", "key2"},
			measures:    Measures{TimeElapsedHistograms: make(map[string]prometheus.ObserverVec)},
		},
		{
			description: "Success with empty Measures",
			name:        "test_parser",
			labelNames:  []string{"key1", "key2"},
			measures:    Measures{},
		},
	}

	for _, tc := range tests {
		o := prometheus.HistogramOpts{
			Name:    tc.name,
			Help:    fmt.Sprintf("tracks %s durations in s", tc.name),
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		}
		assert := assert.New(t)
		testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())
		err := tc.measures.addTimeElapsedHistogram(testFactory, o, tc.labelNames...)
		assert.Equal(tc.expectedErr, err)
		assert.NotNil(tc.measures.TimeElapsedHistograms[tc.name])
	}
}

func TestAddTimeElapsedErr(t *testing.T) {
	t.Run("Nil Histogram", testHistogramNilFactory)
	t.Run("New Histogram Error", testNewHistogramErr)
	t.Run("Histogram already exists in map", testHistogramExistsErr)
}

func testHistogramNilFactory(t *testing.T) {
	assert := assert.New(t)
	o := prometheus.HistogramOpts{
		Name:    "test_parser",
		Help:    fmt.Sprintf("tracks %s durations in s", "test_parser"),
		Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
	}
	measures := Measures{TimeElapsedHistograms: make(map[string]prometheus.ObserverVec)}
	err := measures.addTimeElapsedHistogram(nil, o, []string{"key1", "key2"}...)
	assert.Equal(errNilFactory, err)
}

func testNewHistogramErr(t *testing.T) {
	assert := assert.New(t)
	name := "test_parser"
	o := prometheus.HistogramOpts{
		Name:    name,
		Help:    fmt.Sprintf("tracks %s durations in s", name),
		Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
	}
	labelNames := []string{"key1", "key2"}
	testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())
	measures := Measures{TimeElapsedHistograms: make(map[string]prometheus.ObserverVec)}
	testFactory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Buckets: []float64{60, 21600},
	}, labelNames...)
	err := measures.addTimeElapsedHistogram(testFactory, o, labelNames...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
}

func testHistogramExistsErr(t *testing.T) {
	measures := Measures{TimeElapsedHistograms: make(map[string]prometheus.ObserverVec)}
	o := prometheus.HistogramOpts{
		Name:    "test_parser",
		Help:    fmt.Sprintf("tracks %s durations in s", "test_parser"),
		Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
	}
	assert := assert.New(t)
	testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())
	testHistogram := prometheus.NewHistogramVec(o, nil)
	measures.TimeElapsedHistograms[o.Name] = testHistogram
	err := measures.addTimeElapsedHistogram(testFactory, o, []string{"key1", "key2"}...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
	assert.NotNil(measures.TimeElapsedHistograms[o.Name])
}
