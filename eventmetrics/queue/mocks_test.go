// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package queue

import (
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type mockParser struct {
	mock.Mock
}

func (mp *mockParser) Parse(event interpreter.Event) {
	mp.Called(event)
}

func (mp *mockParser) Name() string {
	args := mp.Called()
	return args.String(0)
}

type mockTimeTracker struct {
	mock.Mock
}

func (m *mockTimeTracker) TrackTime(length time.Duration) {
	m.Called(length)
}
