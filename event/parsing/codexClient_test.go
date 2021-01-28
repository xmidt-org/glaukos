package parsing

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/bascule/acquire"
)

func TestBuildRequest(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		description string
		address     string
		auth        string
	}{
		{
			description: "Success",
			address:     "host/test",
			auth:        "test",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			auth, _ := acquire.NewFixedAuthAcquirer(tc.auth)
			req, err := buildRequest(tc.address, auth)
			assert.Equal(http.MethodGet, req.Method)
			assert.Equal(tc.address, req.URL.Path)
			assert.Equal("test", req.Header.Get("Authorization"))
			assert.Nil(err)
		})
	}
}
