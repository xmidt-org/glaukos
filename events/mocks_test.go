// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"net/http"

	"github.com/stretchr/testify/mock"
)

type mockAcquirer struct {
	mock.Mock
}

func (m *mockAcquirer) Acquire() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

type mockClient struct {
	mock.Mock
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) != nil {
		return args.Get(0).(*http.Response), args.Error(1)
	}
	return nil, args.Error(1)
}
