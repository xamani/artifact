package artifact

import "errors"

type RichError struct {
	Sentinel error
	Message  string
	Details  map[string]string
}

func (e *RichError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Sentinel != nil {
		return e.Sentinel.Error()
	}
	return "unknown error"
}

func (e *RichError) Unwrap() error {
	return e.Sentinel
}

func NewRichError(sentinel error, message string, details map[string]string) error {
	return &RichError{
		Sentinel: sentinel,
		Message:  message,
		Details:  details,
	}
}

func AsRichError(err error) (*RichError, bool) {
	var rich *RichError
	if errors.As(err, &rich) {
		return rich, true
	}
	return nil, false
}
