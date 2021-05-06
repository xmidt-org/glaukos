package eventmetrics

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/zap"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetLogger(t *testing.T) {
	tests := []struct {
		description string
		ctx         context.Context
	}{
		{
			description: "Context Logger",
			ctx:         context.WithValue(context.Background(), struct{}{}, log.NewJSONLogger(os.Stdout)),
		},
		{
			description: "Default Logger",
			ctx:         context.Background(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert.NotNil(t, GetLogger(tc.ctx))
		})
	}
}

func TestNewEndpoints(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time { return now }
	tv := validation.TimeValidator{ValidFrom: -2 * time.Hour, ValidTo: time.Hour, Current: currTime}
	logger := zap.NewNop()

	tests := []struct {
		description string
		event       interface{}
		expectedErr error
		trackTime   bool
		queueErr    error
	}{
		{
			description: "Not an event",
			event:       wrp.Message{},
			expectedErr: errors.New("invalid request info"),
			trackTime:   true,
		},
		{
			description: "Invalid Birthdate",
			event: interpreter.Event{
				Birthdate: now.Add(-4 * time.Hour).UnixNano(),
			},
		},
		{
			description: "Queue Error",
			event: interpreter.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
			},
			queueErr:    errors.New("queue error"),
			expectedErr: errors.New("queue error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			m := new(mockQueue)
			m.On("Queue", mock.Anything).Return(tc.queueErr)
			mockTimeTracker := new(mockTimeTracker)
			if tc.trackTime {
				mockTimeTracker.On("TrackTime", mock.Anything).Once()
			}
			endpoints := NewEndpoints(m, tv, mockTimeTracker, logger)
			resp, err := endpoints.Event(context.Background(), tc.event)
			assert.Nil(resp)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
			mockTimeTracker.AssertExpectations(t)
		})
	}

}
