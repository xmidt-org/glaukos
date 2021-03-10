package metricparsers

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
		assert := assert.New(t)
		testFactory, err := xmetrics.New(xmetrics.Options{})
		assert.Nil(err)
		added, err := tc.measures.addTimeElapsedHistogram(testFactory, tc.name, tc.labelNames...)
		assert.Equal(tc.expectedErr, err)
		assert.True(added)
		assert.NotNil(tc.measures.TimeElapsedHistograms[tc.name])
	}
}

func TestAddTimeElapsedErr(t *testing.T) {
	t.Run("Nil Histogram", testHistogramNilFactory)
	t.Run("New Histogram Error", testNewHistogramErr)
}

func testHistogramNilFactory(t *testing.T) {
	assert := assert.New(t)
	name := "test_parser"
	measures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
	added, err := measures.addTimeElapsedHistogram(nil, name, []string{"key1", "key2"}...)
	assert.Equal(errNilFactory, err)
	assert.False(added)
}

func testNewHistogramErr(t *testing.T) {
	assert := assert.New(t)
	name := "test_parser"
	labelNames := []string{"key1", "key2"}
	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	measures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
	testFactory.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Buckets: []float64{60, 21600},
	}, labelNames)
	added, err := measures.addTimeElapsedHistogram(testFactory, name, labelNames...)
	assert.True(errors.Is(err, errNewHistogram),
		fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
			err, errNewHistogram),
	)
	assert.False(added)
}
