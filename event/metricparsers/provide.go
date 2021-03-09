package metricparsers

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

type TimeElapsedConfig struct {
	Name          string
	InitialEvent  parsing.EventRule
	IncomingEvent parsing.EventRule
}

type TimeElapsedParsersConfig struct {
	DefaultTimeValidation time.Duration
	Parsers               []TimeElapsedConfig
}

// ParsersIn brings together all of the different types of parsers that glaukos uses.
type ParsersIn struct {
	fx.In
	Parsers []queue.Parser `group:"parsers"`
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		client.Provide(),
		provideParsers(),
		fx.Provide(
			CreateTimeElapsedParsers,
			arrange.UnmarshalKey("timeElapsedParsers", TimeElapsedParsersConfig{}),
			func(parsers ParsersIn, timeElapsedParsers []*TimeElapsedParser) []queue.Parser {
				allParsers := parsers.Parsers
				for _, tep := range timeElapsedParsers {
					allParsers = append(allParsers, tep)
				}
				return allParsers
			},
		),
	)
}

func provideParsers() fx.Option {
	return fx.Provide(
		fx.Annotated{
			Group: "parsers",
			Target: func(measures Measures) queue.Parser {
				return &MetadataParser{
					Measures: measures,
				}
			},
		},
	)
}

func CreateTimeElapsedParsers(config TimeElapsedParsersConfig, measures Measures, logger log.Logger, codexClient *client.CodexClient, f xmetrics.Factory) ([]*TimeElapsedParser, error) {
	parserNames := make(map[string]int)
	parsers := make([]*TimeElapsedParser, 0, len(config.Parsers))

	defaultTimeValidator := parsing.TimeValidator{
		Current:   time.Now,
		ValidFrom: config.DefaultTimeValidation,
		ValidTo:   time.Hour,
	}

	for _, config := range config.Parsers {
		var name string
		if len(config.Name) == 0 {
			name = CreateName(defaultName, parserNames)
		} else {
			name = CreateName(config.Name, parserNames)
		}

		initialValidator, err := parsing.NewEventValidator(config.InitialEvent, defaultTimeValidator)
		if err != nil {
			return nil, err
		}
		endValidator, err := parsing.NewEventValidator(config.IncomingEvent, defaultTimeValidator)
		if err != nil {
			return nil, err
		}

		added, err := measures.addTimeElapsedHistogram(f, prometheus.HistogramOpts{
			Name:    name,
			Help:    fmt.Sprintf("tracks %s durations in s", name),
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
			FirmwareLabel,
			HardwareLabel)

		if !added {
			return nil, err
		}

		parser := &TimeElapsedParser{
			measures:         measures,
			logger:           logger,
			client:           codexClient,
			initialValidator: initialValidator,
			endValidator:     endValidator,
			label:            name,
		}

		parsers = append(parsers, parser)
	}

	return parsers, nil
}

func CreateName(name string, parsersMap map[string]int) string {
	name = strings.ReplaceAll(name, " ", "_")
	num := parsersMap[name]
	newCount := num + 1
	var newName string
	if num == 0 {
		newName = name
	} else {
		tempName := fmt.Sprintf("%s_%d", name, newCount)
		for parsersMap[tempName] > 0 {
			newCount++
			tempName = fmt.Sprintf("%s_%d", name, newCount)
		}
		newName = tempName
		parsersMap[newName] = 1
	}
	parsersMap[name] = newCount
	return newName
}
