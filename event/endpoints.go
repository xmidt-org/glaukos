/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"context"
	"errors"

	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/webpa-common/logging"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/fx"
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

			if err := eventQueue.Queue(v); err != nil {
				logging.Error(logger).Log(logging.MessageKey(), "failed to queue message", logging.ErrorKey(), err)
				return nil, err
			}
			return nil, nil
		},
	}
}

// GetLogger pulls the logger from the context and adds a timestamp to it.
func GetLogger(ctx context.Context) log.Logger {
	logger := log.With(xlog.GetDefault(ctx, nil), xlog.TimestampKey(), log.DefaultTimestampUTC)
	return logger
}
