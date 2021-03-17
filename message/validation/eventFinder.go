package validation

import (
	"regexp"

	"github.com/xmidt-org/glaukos/message"
)

type FinderFunc func([]message.Event, message.Event) (message.Event, error)

func (f FinderFunc) Find(events []message.Event, currentEvent message.Event) (message.Event, error) {
	return f(events, currentEvent)
}

// LastSessionFinder is a function to find the specified event with the boot-time of the previous session.
// It also verifies that the current event has the latest boot-time and is not a duplicate, meaning that
// an event of the same type and boot-time has not been previously seen before.
func LastSessionFinder(validators Validators, destRegex *regexp.Regexp) FinderFunc {
	return func(events []message.Event, currentEvent message.Event) (message.Event, error) {
		// verify that the current event has a boot-time
		currentBootTime, err := currentEvent.BootTime()
		if err != nil || currentBootTime <= 0 {
			return message.Event{}, InvalidBootTimeErr{}
		}

		var latestEvent message.Event
		var prevBootTime int64
		// if any fatalValidators return false, it means we should stop looking for an event
		// because there is something wrong with currentEvent, and we should not
		// perform calculations using it.
		fatalValidators := Validators([]Validator{NewestBootTimeValidator(currentEvent), UniqueEventValidator(currentEvent, destRegex)})
		for _, event := range events {
			// check if currentEvent is still valid
			if valid, err := fatalValidators.Valid(event); !valid {
				return message.Event{}, err
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
				// see if the event could be the event we are looking for.
				if valid, _ := validators.Valid(event); valid {
					latestEvent = event
				}
			}
		}

		// Compare the boot-time of the event found to the most recent previous boot-time we found.
		// If it doesn't match, then we haven't found a valid event that is a part of the previous session.
		latestEventBootTime, err := latestEvent.BootTime()
		if err != nil || latestEventBootTime <= 0 || latestEventBootTime != prevBootTime {
			return message.Event{}, EventNotFoundErr{EventType: destRegex.String()}
		}

		return latestEvent, nil
	}

}
