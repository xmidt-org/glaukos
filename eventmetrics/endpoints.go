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
	"github.com/xmidt-org/sallust"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	defaultLogger = zap.NewNop()
)

// GetLoggerFunc is the function used to get a request-specific logger from
// its context.
type GetLoggerFunc func(context.Context) *zap.Logger

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

func NewEndpoints(eventQueue queue.Queue, validator validation.TimeValidation, timeTracker queue.TimeTracker, logger *zap.Logger) Endpoints {
	return Endpoints{
		Event: func(_ context.Context, request interface{}) (interface{}, error) {
			begin := time.Now()
			v, ok := request.(interpreter.Event)
			if !ok {
				timeTracker.TrackTime(time.Since(begin))
				return nil, errors.New("invalid request info: unable to convert to Event")
			}

			if valid, err := validator.Valid(time.Unix(0, v.Birthdate)); !valid {
				logger.Error("invalid birthdate", zap.Error(err), zap.Int64("birthdate", v.Birthdate))
				v.Birthdate = time.Now().UnixNano()
			}

			if err := eventQueue.Queue(queue.EventWithTime{Event: v, BeginTime: begin}); err != nil {
				logger.Error("failed to queue message", zap.Error(err))
				return nil, err
			}
			return nil, nil
		},
	}
}

// GetLogger pulls the logger from the context and adds a timestamp to it.
func GetLogger(ctx context.Context) *zap.Logger {
	logger := sallust.GetDefault(ctx, defaultLogger).With(zap.Any("ts", log.DefaultTimestampUTC))
	return logger
}
