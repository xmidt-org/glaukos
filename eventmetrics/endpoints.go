/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
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

func NewEndpoints(eventQueue queue.Queue, validator validation.TimeValidation, timeTracker queue.TimeTracker, logger log.Logger) Endpoints {
	return Endpoints{
		Event: func(_ context.Context, request interface{}) (interface{}, error) {
			begin := time.Now()
			v, ok := request.(interpreter.Event)
			if !ok {
				timeTracker.TrackTime(time.Since(begin))
				return nil, errors.New("invalid request info: unable to convert to Event")
			}

			if valid, err := validator.Valid(time.Unix(0, v.Birthdate)); !valid {
				level.Error(logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "invalid birthdate", "birthdate", v.Birthdate)
				v.Birthdate = time.Now().UnixNano()
			}

			if err := eventQueue.Queue(queue.EventWithTime{Event: v, BeginTime: begin}); err != nil {
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
