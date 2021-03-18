package history

import (
	"errors"

	"github.com/xmidt-org/glaukos/message"
	"github.com/xmidt-org/glaukos/message/validation"
)

var (
	EventNotFoundErr = errors.New("event not found")
)

// FinderFunc is a function type that takes in a slice of events
// and the current event and returns an Event from the slice.
type FinderFunc func([]message.Event, message.Event) (message.Event, error)

func (f FinderFunc) Find(events []message.Event, currentEvent message.Event) (message.Event, error) {
	return f(events, currentEvent)
}

// LastSessionFinder returns a function to find an event that is deemed valid by the Validator passed in
// with the boot-time of the previous session. If any of the fatalValidators returns false,
// it will stop searching and immediately exit, returning the event
// that prompted the error along with the error returned by the fatalValidator.
func LastSessionFinder(validators validation.Validator, fatalValidators validation.Validator) FinderFunc {
	return func(events []message.Event, currentEvent message.Event) (message.Event, error) {
		// verify that the current event has a boot-time
		currentBootTime, err := currentEvent.BootTime()
		if currentBootTime <= 0 {
			return message.Event{}, validation.InvalidBootTimeErr{OriginalErr: err}
		}

		var latestEvent message.Event
		var prevBootTime int64

		for _, event := range events {

			// if transaction UUIDs are the same, continue onto next event
			if event.TransactionUUID == currentEvent.TransactionUUID {
				continue
			}

			// if any fatalValidators return false, it means we should stop looking for an event
			// because there is something wrong with currentEvent, and we should not
			// perform calculations using it.
			if valid, err := fatalValidators.Valid(event); !valid {
				return event, validation.InvalidEventErr{OriginalErr: err}
			}

			// figure out the latest previous boot-time
			prevBootTime = getPreviousBootTime(event, prevBootTime, currentBootTime)
			// if event does not match validators, continue onto next event.
			latestEvent = compareEvents(event, latestEvent, validators, prevBootTime)
		}

		// final check to make sure that we actually found an event
		return checkEvent(latestEvent, prevBootTime)
	}

}

// CurrentSessionFinder returns a function to find an event that is deemed valid by the Validator passed in
// with the boot-time of the current event. If any of the fatalValidators returns false,
// it will stop searching and immediately exit, returning the event
// that prompted the error along with the error returned by the fatalValidator.
func CurrentSessionFinder(validators validation.Validator, fatalValidators validation.Validator) FinderFunc {
	return func(events []message.Event, currentEvent message.Event) (message.Event, error) {
		// verify that the current event has a boot-time
		currentBootTime, err := currentEvent.BootTime()
		if currentBootTime <= 0 {
			return message.Event{}, validation.InvalidBootTimeErr{OriginalErr: err}
		}

		var latestEvent message.Event
		for _, event := range events {
			// if transaction UUIDs are the same, continue onto next event
			if event.TransactionUUID == currentEvent.TransactionUUID {
				continue
			}

			// if any fatalValidators return false, it means we should stop looking for an event
			// because there is something wrong with currentEvent, and we should not
			// perform calculations using it.
			if valid, err := fatalValidators.Valid(event); !valid {
				return event, validation.InvalidEventErr{OriginalErr: err}
			}

			// Get the bootTime from the event we are checking. If boot-time
			// doesn't exist, move on to the next event.
			bootTime, _ := event.BootTime()
			if bootTime <= 0 {
				continue
			}

			latestEvent = compareEvents(event, latestEvent, validators, currentBootTime)
		}

		// final check to make sure that we actually found an event
		return checkEvent(latestEvent, currentBootTime)
	}
}

// SameEventFinder returns a function that goes through a list of events and compares the currentEvent
// to these events to make sure that currentEvent is valid. If any of the fatalValidators returns false,
// it will stop searching and immediately exit, returning the event
// that prompted the error along with the error returned by the fatalValidator. If all of the fatalValidators
// pass, the currentEvent is returned along with nil error.
func SameEventFinder(fatalValidators validation.Validator) FinderFunc {
	return func(events []message.Event, currentEvent message.Event) (message.Event, error) {
		// verify that the current event has a boot-time
		currentBootTime, err := currentEvent.BootTime()
		if currentBootTime <= 0 {
			return message.Event{}, validation.InvalidBootTimeErr{OriginalErr: err}
		}

		for _, event := range events {
			// if transaction UUIDs are the same, continue onto next event
			if event.TransactionUUID == currentEvent.TransactionUUID {
				continue
			}
			// if any fatalValidators return false, it means we should stop looking for an event
			// because there is something wrong with currentEvent, and we should not
			// perform calculations using it.
			if valid, err := fatalValidators.Valid(event); !valid {
				return event, validation.InvalidEventErr{OriginalErr: err}
			}
		}

		return currentEvent, nil
	}
}

// See if event has a boot-time that has greater than the one we are currently tracking but less than
// the latestBootTime.
func getPreviousBootTime(event message.Event, currentPrevTime int64, latestBootTime int64) int64 {
	// Get the bootTime from the event we are checking. If boot-time
	// doesn't exist, return currentPrevTime, which is the latest previous time currently found.
	bootTime, _ := event.BootTime()
	if bootTime <= 0 {
		return currentPrevTime
	}

	// if boot-time is greater than any we've found so far but less than the current boot-time,
	// return bootTime
	if bootTime > currentPrevTime && bootTime < latestBootTime {
		return bootTime
	}
	return currentPrevTime
}

// Sees if an event is valid based on the validators passed in and whether it has the targetBootTime.
// If not, returns defaultEvent.
func compareEvents(newEvent message.Event, defaultEvent message.Event, validators validation.Validator, targetBootTime int64) message.Event {
	bootTime, _ := newEvent.BootTime()
	currentPrevBootTime, _ := defaultEvent.BootTime()

	// if boot-time doesn't match target boot-time, return previous event
	if bootTime != targetBootTime {
		return defaultEvent
	}

	// if event does not match validators, return previous event
	if valid, _ := validators.Valid(newEvent); !valid {
		return defaultEvent
	}

	if currentPrevBootTime != targetBootTime || newEvent.Birthdate < defaultEvent.Birthdate {
		return newEvent
	}

	return defaultEvent
}

// checks that an event is not empty and matches the target boot-time
func checkEvent(event message.Event, targetBootTime int64) (message.Event, error) {
	bootTime, err := event.BootTime()
	if err != nil || bootTime <= 0 || bootTime != targetBootTime {
		return message.Event{}, EventNotFoundErr
	}
	return event, nil
}
