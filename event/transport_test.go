package event

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/event/queue"
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
			err:                queue.NewErrorCode(400, errors.New("bad request")),
			expectedStatusCode: 400,
		},
		{
			description:        "Non-Status Coder Error",
			err:                errors.New("bad request"),
			expectedStatusCode: 500,
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
	goodMsg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "test",
		Destination: "test",
	}
	tests := []struct {
		description string
		request     interface{}
		expectedMsg wrp.Message
		expectedErr bool
	}{
		{
			description: "Success",
			request:     goodMsg,
			expectedMsg: goodMsg,
		},
		{
			description: "Error decoding request body",
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
			if tc.expectedErr {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
				assert.Equal(tc.expectedMsg, msg)
			}
		})
	}
}
