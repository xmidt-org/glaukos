package event

import (
	"context"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

func TestGetLogger(t *testing.T) {
	tests := []struct {
		description string
		ctx         context.Context
	}{
		{
			description: "Context Logger",
			ctx:         context.WithValue(context.Background(), struct{}{}, log.NewJSONLogger(os.Stdout)),
		},
		{
			description: "Default Logger",
			ctx:         context.Background(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert.NotNil(t, GetLogger(tc.ctx))
		})
	}
}
