package eventmetrics

import (
	"errors"
	"net/http"
	"testing"

	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestBadRequestErr(t *testing.T) {
	assert := assert.New(t)
	message := "bad request"
	err := BadRequestErr{Message: message}
	var statusCoder kithttp.StatusCoder
	assert.True(errors.As(err, &statusCoder))
	assert.Equal(message, err.Error())
	assert.Equal(http.StatusBadRequest, err.StatusCode())
}
