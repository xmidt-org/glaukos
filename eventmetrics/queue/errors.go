// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package queue

import "net/http"

type TooManyRequestsErr struct {
	Message string
}

func (e TooManyRequestsErr) Error() string {
	return e.Message
}

func (e TooManyRequestsErr) StatusCode() int {
	return http.StatusTooManyRequests
}
