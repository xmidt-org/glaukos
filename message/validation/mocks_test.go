package validation

type testError struct {
	err   error
	label string
}

func (t testError) Error() string {
	return t.err.Error()
}

func (t testError) ErrorLabel() string {
	return t.label
}

func (t testError) Unwrap() error {
	return t.err
}
