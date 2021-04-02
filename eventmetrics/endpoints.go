/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package eventmetrics

import (
	"context"
	"errors"
	"time"

	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"

	"github.com/xmidt-org/themis/xlog"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
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

func NewEndpoints(eventQueue queue.Queue, validator validation.TimeValidation, logger log.Logger) Endpoints {
	return Endpoints{
		Event: func(_ context.Context, request interface{}) (interface{}, error) {
			v, ok := request.(interpreter.Event)
			if !ok {
				return nil, errors.New("invalid request info: unable to convert to Event")
			}

			if valid, err := validator.Valid(time.Unix(0, v.Birthdate)); !valid {
				level.Error(logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "invalid birthdate", "birthdate", v.Birthdate)
				v.Birthdate = time.Now().UnixNano()
			}

			if err := eventQueue.Queue(v); err != nil {
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
