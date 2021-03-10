package metricparsers

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
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

type TimeElapsedParsersIn struct {
	fx.In
	config      TimeElapsedParsersConfig
	logger      log.Logger
	measures    Measures
	codexClient *client.CodexClient
	factory     xmetrics.Factory
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
		fx.Provide(
			CreateTimeElapsedParsers,
			arrange.UnmarshalKey("timeElapsedParsers", TimeElapsedParsersConfig{}),
			fx.Annotated{
				Group: "parsers",
				Target: func(measures Measures) queue.Parser {
					return &MetadataParser{
						Measures: measures,
					}
				},
			},
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

// GetParserLogger adds the parser name to the logger
func GetParserLogger(logger log.Logger, parserName string) log.Logger {
	return log.With(logger, "parser", parserName)
}

func CreateTimeElapsedParsers(parsers TimeElapsedParsersIn, otherParsers ParsersIn) ([]*TimeElapsedParser, error) {
	parserNames := make(map[string]int)
	parsersList := make([]*TimeElapsedParser, 0, len(parsers.config.Parsers))

	for _, parser := range otherParsers.Parsers {
		parserNames[parser.Name()] = 1
	}

	for _, parserConfig := range parsers.config.Parsers {
		var name string
		if len(parserConfig.Name) == 0 {
			name = createParserName(defaultName, parserNames)
		} else {
			name = createParserName(parserConfig.Name, parserNames)
		}

		if added, err := parsers.measures.addTimeElapsedHistogram(parsers.factory, name, FirmwareLabel, HardwareLabel); !added {
			return nil, err
		}

		parser, err := CreateNewTimeElapsedParser(parserConfig, name, parsers.codexClient, parsers.logger, parsers.measures)
		if err != nil {
			return nil, err
		}

		parsersList = append(parsersList, parser)
	}

	return parsersList, nil
}

// createParserName creates a unique name for parsers
// parsersMap keeps track of the names that already exist and how many times they have collided
func createParserName(name string, parsersMap map[string]int) string {
	if len(name) == 0 {
		name = "parser"
	}
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
