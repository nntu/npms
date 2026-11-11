package counter

// UnsupportedError reports that a configured counter capability is not
// implemented by the target device. It is permanent for the current counter
// definition, so polling schedulers must not retry it as a transient failure.
type UnsupportedError struct {
	Message string
}

func (e *UnsupportedError) Error() string {
	return e.Message
}

func (e *UnsupportedError) Retryable() bool {
	return false
}
