package queue

import "net/http"

type TooManyRequestsErr struct {
	Message string
}

func (e TooManyRequestsErr) Error() string {
	return e.Message
}

func (e TooManyRequestsErr) StatusCode() int {
	return http.StatusTooManyRequests
}
