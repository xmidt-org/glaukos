// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package queue

import (
	"errors"
	"net/http"
	"testing"

	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestTooManyRequestsErr(t *testing.T) {
	assert := assert.New(t)
	message := "too many requests"
	err := TooManyRequestsErr{Message: message}
	var statusCoder kithttp.StatusCoder
	assert.True(errors.As(err, &statusCoder))
	assert.Equal(message, err.Error())
	assert.Equal(http.StatusTooManyRequests, err.StatusCode())
}
