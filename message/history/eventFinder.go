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
		if err != nil || currentBootTime <= 0 {
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

			// Get the bootTime from the event we are checking. If boot-time
			// doesn't exist, move on to the next event.
			bootTime, err := event.BootTime()
			if err != nil || bootTime <= 0 {
				continue
			}

			// if boot-time is greater than any we've found so far but less than the current boot-time,
			// save the boot-time.
			if bootTime > prevBootTime && bootTime < currentBootTime {
				prevBootTime = bootTime
			}

			// see if the event could be the event we are looking for.
			if valid, _ := validators.Valid(event); valid {
				latestBootTime, err := latestEvent.BootTime()
				if err != nil || bootTime > latestBootTime {
					latestEvent = event
				} else if bootTime == latestBootTime && event.Birthdate < latestEvent.Birthdate {
					latestEvent = event
				}
			}
		}

		// Compare the boot-time of the event found to the most recent previous boot-time we found.
		// If it doesn't match, then we haven't found a valid event that is a part of the previous session.
		latestEventBootTime, err := latestEvent.BootTime()
		if err != nil || latestEventBootTime <= 0 || latestEventBootTime != prevBootTime {
			return message.Event{}, EventNotFoundErr
		}

		return latestEvent, nil
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
		if err != nil || currentBootTime <= 0 {
			return message.Event{}, validation.InvalidBootTimeErr{OriginalErr: err}
		}

		var oldEvent message.Event
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
			bootTime, err := event.BootTime()
			if err != nil || bootTime <= 0 {
				continue
			}

			// See if event is a valid event based on what we are looking for.
			// Also check event's boot-time to see if it is part of the current session.
			if valid, _ := validators.Valid(event); valid && bootTime == currentBootTime {
				// if there has not been another valid event from the current session found or
				// if event's birthdate is older than the current one tracked, track this event instead.
				if oldEvent.Birthdate == 0 || event.Birthdate < oldEvent.Birthdate {
					oldEvent = event
				}
			}
		}

		eventBootTime, err := oldEvent.BootTime()
		if err != nil || eventBootTime <= 0 {
			return message.Event{}, EventNotFoundErr
		}

		return oldEvent, nil
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
		if err != nil || currentBootTime <= 0 {
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
