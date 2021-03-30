package parsers

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/events"
	"github.com/xmidt-org/themis/xmetrics"
)

func TestValidNames(t *testing.T) {
	assert := assert.New(t)
	validParsers := TimeElapsedParsersConfig{
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{Name: "test"},
			TimeElapsedConfig{Name: "test1"},
			TimeElapsedConfig{Name: "random_parser"},
		},
	}

	valid, errName := validNames(validParsers)
	assert.True(valid)
	assert.Empty(errName)

	invalidParsers := TimeElapsedParsersConfig{
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{Name: "test"},
			TimeElapsedConfig{Name: "test1"},
			TimeElapsedConfig{Name: "test"},
			TimeElapsedConfig{Name: "test1"},
		},
	}

	valid, errName = validNames(invalidParsers)
	assert.False(valid)
	assert.Equal("test", errName)
}

func TestTimeElapsedParsersSuccess(t *testing.T) {
	assert := assert.New(t)
	config := TimeElapsedParsersConfig{
		DefaultTimeValidation: -2 * time.Hour,
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{
				Name: "test",
				IncomingEvent: EventConfig{
					Regex: ".*/online$",
				},
				SearchedEvent: EventConfig{
					Regex: ".*/offline$",
				},
			},
			TimeElapsedConfig{
				Name: "test2",
				IncomingEvent: EventConfig{
					Regex: ".*/some-event/",
				},
				SearchedEvent: EventConfig{
					Regex: ".*/some-event-2/",
				},
			},
		},
	}

	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)

	testMeasures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
	timeElapsedParsersIn := TimeElapsedParsersIn{
		Config:      config,
		Logger:      log.NewNopLogger(),
		Measures:    testMeasures,
		CodexClient: &events.CodexClient{},
		Factory:     testFactory,
	}

	timeElapsedParsers, err := TimeElapsedParsers(timeElapsedParsersIn)
	assert.Len(timeElapsedParsers, len(config.Parsers))
	assert.Nil(err)

	for _, parser := range timeElapsedParsers {
		histogram, found := testMeasures.TimeElapsedHistograms[parser.Name()]
		assert.True(found)
		assert.NotNil(histogram)
	}
}

func TestCreateTimeElapsedParsersErrors(t *testing.T) {
	t.Run("histogram error", testHistogramError)
	t.Run("parser error", testParserError)
	t.Run("repeated parser names", testRepeatedNamesError)
}

func TestParserLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := log.NewJSONLogger(buf)

	parserLogger := ParserLogger(logger, "test_parser")

	if err := parserLogger.Log(); err != nil {
		t.Fatal(err)
	}
	if want, have := `{"parser":"test_parser"}`+"\n", buf.String(); want != have {
		t.Errorf("\nwant %#v\nhave %#v", want, have)
	}
}

func testHistogramError(t *testing.T) {
	assert := assert.New(t)
	config := TimeElapsedParsersConfig{
		DefaultTimeValidation: -2 * time.Hour,
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{
				Name: "test1",
				IncomingEvent: EventConfig{
					Regex: ".*/online$",
				},
				SearchedEvent: EventConfig{
					Regex: ".*/offline$",
				},
			},
		},
	}
	timeElapsedParsersIn := TimeElapsedParsersIn{
		Config:      config,
		Logger:      log.NewNopLogger(),
		Measures:    Measures{},
		CodexClient: &events.CodexClient{},
		Factory:     nil,
	}

	parsers, err := TimeElapsedParsers(timeElapsedParsersIn)
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
				IncomingEvent: EventConfig{
					Regex: ".*/online$",
				},
				SearchedEvent: EventConfig{
					Regex: ".*/offline$",
				},
			},
			TimeElapsedConfig{
				Name: "test2",
				IncomingEvent: EventConfig{
					Regex: `[`,
				},
				SearchedEvent: EventConfig{
					Regex: ".*/offline$",
				},
			},
		},
	}

	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	timeElapsedParsersIn := TimeElapsedParsersIn{
		Config:      config,
		Logger:      log.NewNopLogger(),
		Measures:    Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)},
		CodexClient: &events.CodexClient{},
		Factory:     testFactory,
	}

	parsers, err := TimeElapsedParsers(timeElapsedParsersIn)
	assert.Nil(parsers)
	assert.NotNil(err)
}

func testRepeatedNamesError(t *testing.T) {
	assert := assert.New(t)
	config := TimeElapsedParsersConfig{
		DefaultTimeValidation: -2 * time.Hour,
		Parsers: []TimeElapsedConfig{
			TimeElapsedConfig{
				Name: "test1",
				IncomingEvent: EventConfig{
					Regex: ".*/online$",
				},
				SearchedEvent: EventConfig{
					Regex: ".*/offline$",
				},
			},
			TimeElapsedConfig{
				Name: "test1",
				IncomingEvent: EventConfig{
					Regex: ".*/online$",
				},
				SearchedEvent: EventConfig{
					Regex: ".*/offline$",
				},
			},
		},
	}

	testFactory, err := xmetrics.New(xmetrics.Options{})
	assert.Nil(err)
	timeElapsedParsersIn := TimeElapsedParsersIn{
		Config:      config,
		Logger:      log.NewNopLogger(),
		Measures:    Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)},
		CodexClient: &events.CodexClient{},
		Factory:     testFactory,
	}

	parsers, err := TimeElapsedParsers(timeElapsedParsersIn)
	assert.Nil(parsers)
	assert.NotNil(err)

}
