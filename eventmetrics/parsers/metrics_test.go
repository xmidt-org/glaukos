package parsers

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/touchstone"
)

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
		added, err := tc.measures.addTimeElapsedHistogram(testFactory, o, tc.labelNames...)
		assert.Equal(tc.expectedErr, err)
		assert.True(added)
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
	added, err := measures.addTimeElapsedHistogram(nil, o, []string{"key1", "key2"}...)
	assert.Equal(errNilFactory, err)
	assert.False(added)
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
	added, err := measures.addTimeElapsedHistogram(testFactory, o, labelNames...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
	assert.False(added)
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
	added, err := measures.addTimeElapsedHistogram(testFactory, o, []string{"key1", "key2"}...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
	assert.False(added)
	assert.NotNil(measures.TimeElapsedHistograms[o.Name])
}
