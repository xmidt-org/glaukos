// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package eventmetrics

import "net/http"

type BadRequestErr struct {
	Message string
}

func (e BadRequestErr) Error() string {
	return e.Message
}

func (e BadRequestErr) StatusCode() int {
	return http.StatusBadRequest
}
