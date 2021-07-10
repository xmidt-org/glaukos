package parsers

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter"
	"go.uber.org/zap"
)

var (
	errCalculation   = errors.New("time elapsed calculation error")
	errEventNotFound = errors.New("event not found")
)

type CalculatorFunc func([]interpreter.Event, interpreter.Event) error

func (cf CalculatorFunc) Calculate(events []interpreter.Event, event interpreter.Event) error {
	return cf(events, event)
}

func BootTimeCalculator(logger *zap.Logger, histogram prometheus.ObserverVec) CalculatorFunc {
	return func(events []interpreter.Event, event interpreter.Event) error {
		bootTime, _ := event.BootTime()
		bootTimeUnix := time.Unix(bootTime, 0)
		birthdateUnix := time.Unix(0, event.Birthdate)
		var bootDuration float64
		if bootTime > 0 && event.Birthdate > 0 {
			bootDuration = birthdateUnix.Sub(bootTimeUnix).Seconds()
		}

		if bootDuration <= 0 {
			deviceID, _ := event.DeviceID()
			logger.Error("invalid time calculated", zap.String("deviceID", deviceID), zap.Float64("invalid time elapsed", bootDuration), zap.String("incoming event", event.TransactionUUID))
			return errCalculation
		}

		labels := getTimeElapsedHistogramLabels(event)
		histogram.With(labels).Observe(bootDuration)
		return nil
	}
}

type TimeBetweenEventsCalculator struct {
	eventFinder Finder
	histogram   prometheus.ObserverVec
	logger      *zap.Logger
}

func (c TimeBetweenEventsCalculator) Calculate(events []interpreter.Event, event interpreter.Event) error {
	currentBirthdate := time.Unix(0, event.Birthdate)
	startingEvent, err := c.eventFinder.Find(events, event)
	if err != nil {
		c.logger.Error("time calculation error", zap.Error(err))
		return errEventNotFound
	}

	startingEventTime := time.Unix(0, startingEvent.Birthdate)
	var timeElapsed float64
	if event.Birthdate > 0 && startingEvent.Birthdate > 0 {
		timeElapsed = currentBirthdate.Sub(startingEventTime).Seconds()
	}

	if timeElapsed <= 0 {
		deviceID, _ := event.DeviceID()
		c.logger.Error("time calculation error",
			zap.String("deviceID", deviceID),
			zap.String("incoming event", event.TransactionUUID),
			zap.String("comparison event", startingEvent.TransactionUUID),
			zap.Float64("time calculated", timeElapsed))
		return errCalculation
	}

	labels := getTimeElapsedHistogramLabels(event)
	c.histogram.With(labels).Observe(timeElapsed)
	return nil
}
