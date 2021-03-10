package metricparsers

import (
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xmetrics"
)

func TestCreateTimeElapsedParsersSuccess(t *testing.T) {
	assert := assert.New(t)
	config := TimeElapsedParsersConfig{
		DefaultTimeValidation: -2 * time.Hour,
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{
				Name: "test",
				InitialEvent: parsing.EventRule{
					Regex: ".*/online$",
				},
				IncomingEvent: parsing.EventRule{
					Regex: ".*/offline$",
				},
			},
			TimeElapsedConfig{
				Name: "test2",
				InitialEvent: parsing.EventRule{
					Regex: ".*/some-event/",
				},
				IncomingEvent: parsing.EventRule{
					Regex: ".*/some-event-2/",
				},
			},
		},
	}

	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)

	testMeasures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
	timeElapsedParsersIn := TimeElapsedParsersIn{
		config:      config,
		logger:      log.NewNopLogger(),
		measures:    testMeasures,
		codexClient: &client.CodexClient{},
		factory:     testFactory,
	}

	existentParsers := []queue.Parser{&MetadataParser{}, &TimeElapsedParser{label: "test"}}
	parsersIn := ParsersIn{
		Parsers: existentParsers,
	}

	restrictedNames := make([]string, 0, len(existentParsers))
	for _, parser := range existentParsers {
		restrictedNames = append(restrictedNames, parser.Name())
	}

	timeElapsedParsers, err := CreateTimeElapsedParsers(timeElapsedParsersIn, parsersIn)
	assert.Len(timeElapsedParsers, len(config.Parsers))

	for _, parser := range timeElapsedParsers {
		assert.NotContains(restrictedNames, parser.label)
		histogram, found := parser.measures.TimeElapsedHistograms[parser.label]
		assert.True(found)
		assert.NotNil(histogram)
	}
}

func TestCreateName(t *testing.T) {
	assert := assert.New(t)
	expectedNames := []string{"parser", "parser_2", "test", "test_2", "test_3", "random", "test_2_2", "already_exists"}
	names := map[string]int{"already_exists": 1}
	testParserNames := []string{"", "", "test", "test", "test", "random", "test_2"}
	for _, name := range testParserNames {
		createParserName(name, names)
	}

	for _, name := range expectedNames {
		assert.True(names[name] > 0)
	}
}

func TestCreateTimeElapsedParsersErrors(t *testing.T) {
	t.Run("histogram error", testHistogramError)
	t.Run("parser error", testParserError)
}

func testHistogramError(t *testing.T) {
	assert := assert.New(t)
	config := TimeElapsedParsersConfig{
		DefaultTimeValidation: -2 * time.Hour,
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{
				Name: "test1",
				InitialEvent: parsing.EventRule{
					Regex: ".*/online$",
				},
				IncomingEvent: parsing.EventRule{
					Regex: ".*/offline$",
				},
			},
		},
	}
	timeElapsedParsersIn := TimeElapsedParsersIn{
		config:      config,
		logger:      log.NewNopLogger(),
		measures:    Measures{},
		codexClient: &client.CodexClient{},
		factory:     nil,
	}
	parsersIn := ParsersIn{}

	parsers, err := CreateTimeElapsedParsers(timeElapsedParsersIn, parsersIn)
	assert.Nil(parsers)
	assert.NotNil(err)
}

func testParserError(t *testing.T) {
	assert := assert.New(t)
	config := TimeElapsedParsersConfig{
		DefaultTimeValidation: -2 * time.Hour,
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{
				Name: "test1",
				InitialEvent: parsing.EventRule{
					Regex: ".*/online$",
				},
				IncomingEvent: parsing.EventRule{
					Regex: ".*/offline$",
				},
			},
			TimeElapsedConfig{
				Name: "test2",
				InitialEvent: parsing.EventRule{
					Regex: `'(?=.*\d)'`,
				},
				IncomingEvent: parsing.EventRule{
					Regex: ".*/offline$",
				},
			},
		},
	}

	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	timeElapsedParsersIn := TimeElapsedParsersIn{
		config:      config,
		logger:      log.NewNopLogger(),
		measures:    Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)},
		codexClient: &client.CodexClient{},
		factory:     testFactory,
	}
	parsersIn := ParsersIn{}

	parsers, err := CreateTimeElapsedParsers(timeElapsedParsersIn, parsersIn)
	assert.Nil(parsers)
	assert.NotNil(err)
}
