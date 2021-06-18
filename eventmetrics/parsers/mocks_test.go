package parsers

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"
)

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Valid(e interpreter.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}

type mockCycleValidator struct {
	mock.Mock
}

func (m *mockCycleValidator) Valid(events []interpreter.Event) (bool, error) {
	args := m.Called(events)
	return args.Bool(0), args.Error(1)
}

type mockEventClient struct {
	mock.Mock
}

func (m *mockEventClient) GetEvents(deviceID string) []interpreter.Event {
	args := m.Called(deviceID)
	return args.Get(0).([]interpreter.Event)
}

type mockFinder struct {
	mock.Mock
}

func (m *mockFinder) Find(events []interpreter.Event, incomingEvent interpreter.Event) (interpreter.Event, error) {
	args := m.Called(events, incomingEvent)
	return args.Get(0).(interpreter.Event), args.Error(1)
}

type mockEventsParser struct {
	mock.Mock
}

func (m *mockEventsParser) Parse(eventsHistory []interpreter.Event, currentEvent interpreter.Event) ([]interpreter.Event, error) {
	args := m.Called(eventsHistory, currentEvent)
	return args.Get(0).([]interpreter.Event), args.Error(1)
}

type testTaggedError struct {
	tag validation.Tag
}

func (e testTaggedError) Error() string {
	return "test error"
}

func (e testTaggedError) Tag() validation.Tag {
	return e.tag
}

type testTaggedErrors struct {
	tags []validation.Tag
}

func (e testTaggedErrors) Error() string {
	return "test error"
}

func (e testTaggedErrors) Tags() []validation.Tag {
	return e.tags
}

func (t testTaggedErrors) UniqueTags() []validation.Tag {
	var tags []validation.Tag
	existingTags := make(map[validation.Tag]bool)

	for _, tag := range t.tags {
		if !existingTags[tag] {
			existingTags[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}
