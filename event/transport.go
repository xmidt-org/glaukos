/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/wrp-go/v3"
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
		details := []interface{}{}

		var e kithttp.StatusCoder
		if errors.As(err, &e) {
			statusCode = e.StatusCode()
		}

		// get logger from context and log error
		logger := getLogger(ctx)
		if logger != nil {
			logger = log.With(logger, details...)
			logger.Log(level.Key(), level.ErrorValue(), xlog.MessageKey(), "failed to process event",
				xlog.ErrorKey(), err, "resp status code", statusCode)
		}

		response.WriteHeader(statusCode)
	}
}

func DecodeEvent(_ context.Context, r *http.Request) (interface{}, error) {
	var message wrp.Message
	msgBytes, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("could not read request body: %v", err)
	}

	err = wrp.NewDecoderBytes(msgBytes, wrp.Msgpack).Decode(&message)
	if err != nil {
		return nil, fmt.Errorf("could not decode request body: %v", err)
	}
	return message, nil
}
