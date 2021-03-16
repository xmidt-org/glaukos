package eventmetrics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/message"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestEncodeResponseCode(t *testing.T) {
	assert := assert.New(t)
	codes := []int{200, 201, 400, 404, 500, 403}
	for _, c := range codes {
		f := EncodeResponseCode(c)
		rec := httptest.NewRecorder()
		f(context.Background(), rec, nil)
		assert.Equal(c, rec.Code)
	}
}

func TestEncodeError(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		description        string
		err                error
		expectedStatusCode int
	}{
		{
			description:        "Status Coder Error",
			err:                BadRequestErr{Message: "bad request"},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "Non-Status Coder Error",
			err:                errors.New("bad request"),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	f := EncodeError(GetLogger)

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			rec := httptest.NewRecorder()
			f(context.Background(), tc.err, rec)
			assert.Equal(tc.expectedStatusCode, rec.Code)
		})
	}
}

func TestDecodeEvent(t *testing.T) {
	assert := assert.New(t)
	timeString := "2021-03-02T18:00:01Z"
	now, err := time.Parse(time.RFC3339Nano, timeString)
	assert.Nil(err)

	goodMsg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "test",
		Destination: "test",
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		TransactionUUID: "some-id",
		ContentType:     "content",
		Payload:         []byte(fmt.Sprintf(`{"ts": "%s"}`, timeString)),
		PartnerIDs:      []string{"partner1", "partner2"},
	}

	goodEvent := message.Event{
		MsgType:     int(wrp.SimpleEventMessageType),
		Source:      "test",
		Destination: "test",
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		TransactionUUID: "some-id",
		ContentType:     "content",
		Payload:         fmt.Sprintf(`{"ts": "%s"}`, timeString),
		PartnerIDs:      []string{"partner1", "partner2"},
		Birthdate:       now.UnixNano(),
	}
	tests := []struct {
		description   string
		request       interface{}
		expectedEvent message.Event
		expectedErr   bool
	}{
		{
			description:   "Success",
			request:       goodMsg,
			expectedEvent: goodEvent,
		},
		{
			description: "Error decoding msgpack",
			request:     "{{{",
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var marshaledMsg []byte
			var err error
			err = wrp.NewEncoderBytes(&marshaledMsg, wrp.Msgpack).Encode(tc.request)
			assert.Nil(err)
			request, e := http.NewRequest(http.MethodGet, "/", bytes.NewReader(marshaledMsg))
			assert.Nil(e)
			msg, err := DecodeEvent(context.Background(), request)
			if !tc.expectedErr {
				assert.Equal(tc.expectedEvent, msg)
				assert.Nil(err)
			} else {
				var statusCoder kithttp.StatusCoder
				assert.True(errors.As(err, &statusCoder))
				assert.Equal(statusCoder.StatusCode(), http.StatusBadRequest)
			}
		})
	}
}
