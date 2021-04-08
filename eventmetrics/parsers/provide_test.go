package parsers

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"testing"
	"time"

	logging "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/events"
	"github.com/xmidt-org/touchstone"
)

func TestValidNames(t *testing.T) {
	tests := []struct {
		description   string
		parsers       []TimeElapsedConfig
		expectedErr   error
		expectedValid bool
	}{
		{
			description: "valid",
			parsers: []TimeElapsedConfig{
				TimeElapsedConfig{Name: "test"},
				TimeElapsedConfig{Name: "test1"},
				TimeElapsedConfig{Name: "random_parser"},
			},
			expectedValid: true,
		},
		{
			description: "repeated names",
			parsers: []TimeElapsedConfig{
				TimeElapsedConfig{Name: "test"},
				TimeElapsedConfig{Name: "test1"},
				TimeElapsedConfig{Name: "test"},
				TimeElapsedConfig{Name: "test1"},
			},
			expectedValid: false,
			expectedErr:   errors.New("test"),
		},
		{
			description: "blank name",
			parsers: []TimeElapsedConfig{
				TimeElapsedConfig{Name: "test"},
				TimeElapsedConfig{Name: ""},
				TimeElapsedConfig{Name: "test"},
				TimeElapsedConfig{Name: "test1"},
			},
			expectedValid: false,
			expectedErr:   errInvalidName,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			valid, err := validNames(tc.parsers)
			assert.Equal(tc.expectedValid, valid)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
		})
	}

}

func TestTimeElapsedParsersSuccess(t *testing.T) {
	tests := []struct {
		description string
		config      TimeElapsedParsersConfig
	}{
		{
			description: "success",
			config: TimeElapsedParsersConfig{
				DefaultValidFrom: -2 * time.Hour,
				Parsers: []TimeElapsedConfig{
					TimeElapsedConfig{
						Name: "test",
						IncomingEvent: EventConfig{
							Regex:     ".*/online$",
							ValidFrom: -1 * time.Hour,
						},
						SearchedEvent: EventConfig{
							Regex:     ".*/offline$",
							ValidFrom: -1 * time.Hour,
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
			},
		},
		{
			description: "success with defaults",
			config: TimeElapsedParsersConfig{
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
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())

			testMeasures := Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)}
			timeElapsedParsersIn := TimeElapsedParsersIn{
				Config:      tc.config,
				Logger:      logging.NewNopLogger(),
				Measures:    testMeasures,
				CodexClient: &events.CodexClient{},
				Factory:     testFactory,
			}

			timeElapsedParsers, err := TimeElapsedParsers(timeElapsedParsersIn)
			assert.Len(timeElapsedParsers, len(tc.config.Parsers))
			assert.Nil(err)

			for _, parser := range timeElapsedParsers {
				histogram, found := testMeasures.TimeElapsedHistograms[parser.Name()]
				assert.True(found)
				assert.NotNil(histogram)
			}
		})
	}
}

func TestCreateTimeElapsedParsersErrors(t *testing.T) {
	t.Run("histogram error", testHistogramError)
	t.Run("parser error", testParserError)
	t.Run("repeated parser names", testRepeatedNamesError)
}

func TestParserLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.NewJSONLogger(buf)

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
		DefaultValidFrom: -2 * time.Hour,
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
		Logger:      logging.NewNopLogger(),
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
		DefaultValidFrom: -2 * time.Hour,
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

	testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())
	timeElapsedParsersIn := TimeElapsedParsersIn{
		Config:      config,
		Logger:      logging.NewNopLogger(),
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
		DefaultValidFrom: -2 * time.Hour,
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

	testFactory := touchstone.NewFactory(touchstone.Config{}, log.New(ioutil.Discard, "", 0), prometheus.NewPedanticRegistry())
	timeElapsedParsersIn := TimeElapsedParsersIn{
		Config:      config,
		Logger:      logging.NewNopLogger(),
		Measures:    Measures{TimeElapsedHistograms: make(map[string]metrics.Histogram)},
		CodexClient: &events.CodexClient{},
		Factory:     testFactory,
	}

	parsers, err := TimeElapsedParsers(timeElapsedParsersIn)
	assert.Nil(parsers)
	assert.NotNil(err)

}
