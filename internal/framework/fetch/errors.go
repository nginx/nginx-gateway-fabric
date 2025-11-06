package fetch

import "fmt"

// ChecksumMismatchError represents an error when the calculated checksum doesn't match the expected checksum.
// This type of error should not trigger retries as it indicates data corruption or tampering.
type ChecksumMismatchError struct {
	Expected string
	Actual   string
}

func (e *ChecksumMismatchError) Error() string {
	return fmt.Sprintf("checksum mismatch: expected %s, got %s", e.Expected, e.Actual)
}

// HTTPStatusError represents an HTTP status code error for retry logic.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.StatusCode)
}
