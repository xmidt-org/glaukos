package metricparsers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/webpa-common/xhttp"
	"go.uber.org/fx"
)

type TimeElapsedParsersConfig struct {
	DefaultBootTimeValidation  parsing.TimeRule
	DefaultBirthdateValidation parsing.TimeRule
	Parsers                    []TimeElapsedConfig
}

// CodexConfig determines the auth and address for connecting to the codex cluster.
type CodexConfig struct {
	Address        string
	Auth           AuthAcquirerConfig
	MaxRetryCount  int
	CircuitBreaker client.CircuitBreakerConfig
}

type AuthAcquirerConfig struct {
	JWT   acquire.RemoteBearerTokenAcquirerOptions
	Basic string
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
		provideParsers(),
		fx.Provide(
			arrange.UnmarshalKey("codex", CodexConfig{}),
			arrange.UnmarshalKey("timeElapsedParsers", TimeElapsedParsersConfig),
			determineCodexTokenAcquirer,
			createCircuitBreaker,
			func(config CodexConfig, cb *gobreaker.CircuitBreaker, codexAuth acquire.Acquirer, logger log.Logger) EventClient {
				if config.MaxRetryCount < 0 {
					config.MaxRetryCount = 3
				}
				retryOptions := xhttp.RetryOptions{
					Logger:   logger,
					Retries:  config.MaxRetryCount,
					Interval: time.Second * 30,
					// Always retry on failures up to the max count.
					ShouldRetry:       func(error) bool { return true },
					ShouldRetryStatus: func(code int) bool { return false },
				}
				return &client.CodexClient{
					Address:      config.Address,
					Auth:         codexAuth,
					RetryOptions: retryOptions,
					Client:       http.DefaultClient,
					Logger:       logger,
					CB:           cb,
				}
			},
			func(parsers ParsersIn) []queue.Parser {
				return parsers.Parsers
			},
		),
	)
}

func determineCodexTokenAcquirer(logger log.Logger, config CodexConfig) (acquire.Acquirer, error) {
	defaultAcquirer := &acquire.DefaultAcquirer{}
	jwt := config.Auth.JWT
	if jwt.AuthURL != "" && jwt.Buffer > 0 && jwt.Timeout > 0 {
		level.Debug(logger).Log(xlog.MessageKey(), "using jwt")
		return acquire.NewRemoteBearerTokenAcquirer(jwt)
	}

	if config.Auth.Basic != "" {
		level.Debug(logger).Log(xlog.MessageKey(), "using basic auth")
		return acquire.NewFixedAuthAcquirer(config.Auth.Basic)
	}

	level.Error(logger).Log(xlog.ErrorKey(), "failed to create acquirer")
	return defaultAcquirer, nil

}

func createCircuitBreaker(config CodexConfig) *gobreaker.CircuitBreaker {
	c := config.CircuitBreaker

	if c.ConsecutiveFailuresAllowed == 0 {
		c.ConsecutiveFailuresAllowed = 1
	}

	settings := gobreaker.Settings{
		Name:        "Codex Circuit Breaker",
		MaxRequests: c.MaxRequests,
		Interval:    c.Interval,
		Timeout:     c.Timeout,
		ReadyToTrip: createReadyToTripFunc(c),
	}

	return gobreaker.NewCircuitBreaker(settings)
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
		fx.Annotated{
			Group: "parsers",
			Target: func(logger log.Logger, measures Measures, client EventClient) queue.Parser {
				return &BootTimeParser{
					Measures: measures,
					Logger:   logger,
					Client:   client,
				}
			},
		},
		fx.Annotated{
			Group: "parsers",
			Target: func(logger log.Logger, measures Measures, client EventClient) queue.Parser {
				return &RebootTimeParser{
					Measures: measures,
					Logger:   logger,
					Client:   client,
					Label:    "reboot_to_manageable_parser",
				}
			},
		},
	)
}

func CreateTimeElapsedParsers(config TimeElapsedParsersConfig, measures Measures, logger log.Logger, client EventClient) {
	defaultName := "time_elapsed_parser"
	parserNames := make(map[string]int)

	bootTimeValidation := parsing.TimeValidator{
		CurrentTime: time.Now,
		ValidFrom:   config.BootTimeValidation.ValidFrom,
		ValidTo:     config.BootTimeValidation.ValidTo,
	}

	birthdateValidation := parsing.TimeValidator{
		CurrentTime: time.Now,
		ValidFrom:   config.BirthdateValidation.ValidFrom,
		ValidTo:     config.BirthdateValidation.ValidTo,
	}

	for _, config := range config.Parsers {
		var name string
		if len(config.Name) == 0 {
			name = CreateName(defaultName, parserNames)
		} else {
			name = CreateName(config.Name, parserNames)
		}

		if parsing.ParseTimeLocation(config.InitialEvent.CalculateUsing) == parsing.Birthdate {

		}
		initialValidation, err := parsing.NewEventValidator(config.InitialEvent)

		parser := TimeElapsedParser{}
	}
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
