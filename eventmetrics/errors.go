package eventmetrics

import "net/http"

type BadRequestErr struct {
	Message string
}

func (e BadRequestErr) Error() string {
	return e.Message
}

func (e BadRequestErr) StatusCode() int {
	return http.StatusBadRequest
}
