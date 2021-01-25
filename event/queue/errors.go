package queue

type ErrorCode struct {
	code int
	err  error
}

// NewErrorCode creates a new error code with the specified status code and error message
func NewErrorCode(code int, err error) ErrorCode {
	return ErrorCode{
		code: code,
		err:  err,
	}
}

func (e ErrorCode) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e ErrorCode) StatusCode() int {
	return e.code
}
