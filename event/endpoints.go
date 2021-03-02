/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"context"
	"errors"
	"time"

	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xlog"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/fx"
)

var (
	defaultLogger = log.NewNopLogger()
)

// GetLoggerFunc is the function used to get a request-specific logger from
// its context.
type GetLoggerFunc func(context.Context) log.Logger

// Endpoints is the register go-kit endpoints.
type Endpoints struct {
	Event endpoint.Endpoint `name:"eventEndpoint"`
}

// EndpointsDecodeIn provides everything needed to handle the endpoints
// provided.
type EndpointsDecodeIn struct {
	fx.In
	Endpoints
	GetLogger GetLoggerFunc
}

func NewEndpoints(eventQueue *queue.EventQueue, logger log.Logger) Endpoints {
	return Endpoints{
		Event: func(_ context.Context, request interface{}) (interface{}, error) {
			v, ok := request.(wrp.Message)
			if !ok {
				return nil, errors.New("invalid request info")
			}

			begin, err := parsing.GetValidBirthDate(time.Now, v.Payload)
			if err != nil {
				level.Error(logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to get valid birthdate from payload")
				begin = time.Now()
			}

			if err := eventQueue.Queue(queue.WrpWithTime{Message: v, Beginning: begin}); err != nil {
				level.Error(logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to queue message")
				return nil, err
			}
			return nil, nil
		},
	}
}

// GetLogger pulls the logger from the context and adds a timestamp to it.
func GetLogger(ctx context.Context) log.Logger {
	logger := log.With(xlog.GetDefault(ctx, defaultLogger), xlog.TimestampKey(), log.DefaultTimestampUTC)
	return logger
}
