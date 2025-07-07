// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package eventmetrics

import (
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
)

type mockQueue struct {
	mock.Mock
}

func (m *mockQueue) Queue(e queue.EventWithTime) error {
	args := m.Called(e)
	return args.Error(0)
}

type mockTimeTracker struct {
	mock.Mock
}

func (m *mockTimeTracker) TrackTime(length time.Duration) {
	m.Called(length)
}
