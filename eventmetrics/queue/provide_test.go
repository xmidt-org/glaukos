package queue

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/themis/xmetrics"
)

func TestNewTimeTrackerSuccess(t *testing.T) {
	testFactory, err := xmetrics.New(xmetrics.Options{})
	tracker, err := newTimeTracker(testFactory)
	assert.NotNil(t, tracker)
	assert.Nil(t, err)
}

func TestNewTimeTrackerErr(t *testing.T) {
	testFactory, err := xmetrics.New(xmetrics.Options{})
	testFactory.NewHistogram(
		prometheus.HistogramOpts{
			Name: "time_in_memory",
		}, []string{},
	)
	tracker, err := newTimeTracker(testFactory)
	assert.Nil(t, tracker)
	assert.NotNil(t, err)
}
