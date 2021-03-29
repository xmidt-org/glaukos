package parsers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/themis/xmetrics"
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
			measures:    Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)},
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
		testFactory, err := xmetrics.New(xmetrics.Options{})
		assert.Nil(err)
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
	measures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
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
	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	measures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
	testFactory.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Buckets: []float64{60, 21600},
	}, labelNames)
	added, err := measures.addTimeElapsedHistogram(testFactory, o, labelNames...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
	assert.False(added)
}

func testHistogramExistsErr(t *testing.T) {
	measures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
	o := prometheus.HistogramOpts{
		Name:    "test_parser",
		Help:    fmt.Sprintf("tracks %s durations in s", "test_parser"),
		Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
	}
	assert := assert.New(t)
	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	testFac, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	testHistogram, err := testFac.NewHistogram(o, nil)
	assert.Nil(err)
	measures.TimeElapsedHistograms[o.Name] = testHistogram
	added, err := measures.addTimeElapsedHistogram(testFactory, o, []string{"key1", "key2"}...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
	assert.False(added)
	assert.NotNil(measures.TimeElapsedHistograms[o.Name])
}
