package event

import (
	"net/http"
)

type QueueFullError struct {
	Message string
}

func (qfe QueueFullError) Error() string {
	return qfe.Message
}

func (qfe QueueFullError) StatusCode() int {
	return http.StatusTooManyRequests
}
