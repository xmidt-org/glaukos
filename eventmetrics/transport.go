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
	"fmt"
	"io"
	"net/http"

	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/zap"
)

// EncodeResponseCode creates a go-kit EncodeResponseFunc that returns the
// response code given.
func EncodeResponseCode(statusCode int) kithttp.EncodeResponseFunc {
	return func(_ context.Context, response http.ResponseWriter, _ interface{}) error {
		response.WriteHeader(statusCode)
		return nil
	}
}

// EncodeError logs the error provided using the logger in the context.  The
// log message includes any details given.  If the error includes a status code,
// that is the status code given in the response.  Otherwise, a 500 is sent.
func EncodeError(getLogger GetLoggerFunc) kithttp.ErrorEncoder {
	return func(ctx context.Context, err error, response http.ResponseWriter) {
		statusCode := http.StatusInternalServerError

		var e kithttp.StatusCoder
		if errors.As(err, &e) {
			statusCode = e.StatusCode()
		}

		// get logger from context and log error
		logger := getLogger(ctx)
		if logger != nil {
			logger.Error("failed to process event", zap.Error(err), zap.Any("resp status code", statusCode))
		}

		response.WriteHeader(statusCode)
	}
}

// DecodeEvent decodes the request body into a wrp.Message type.
func DecodeEvent(_ context.Context, r *http.Request) (interface{}, error) {
	var msg wrp.Message
	var err error
	msgBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, BadRequestErr{Message: fmt.Sprintf("could not read request body: %v", err)}
	}

	err = wrp.NewDecoderBytes(msgBytes, wrp.Msgpack).Decode(&msg)
	if err != nil {
		return nil, BadRequestErr{Message: fmt.Sprintf("could not decode request body: %v", err)}
	}

	event, _ := interpreter.NewEvent(msg)
	return event, nil
}
