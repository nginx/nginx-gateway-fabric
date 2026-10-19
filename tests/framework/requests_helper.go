package framework

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// RequestOutcome is the result VerifyRequest waits for.
type RequestOutcome struct {
	expectedContent string
	expectedStatus  int
	expectFailure   bool
}

// RequestSucceedsWith expects HTTP 200 with expectedContent in the body.
func RequestSucceedsWith(expectedContent string) RequestOutcome {
	return RequestOutcome{expectedContent: expectedContent}
}

// RequestFails expects a transport error or any non-200 response.
func RequestFails() RequestOutcome {
	return RequestOutcome{expectFailure: true}
}

// RequestReturnsStatus expects a response with exactly the given HTTP status code.
func RequestReturnsStatus(code int) RequestOutcome {
	return RequestOutcome{expectedStatus: code}
}

// VerifyRequest polls until the HTTP request to url produces the expected outcome.
func VerifyRequest(
	tc TimeoutConfig,
	url string,
	addr string,
	outcome RequestOutcome,
	opts ...Option,
) {
	GinkgoHelper()
	Eventually(func() error {
		if !outcome.expectFailure && outcome.expectedStatus == 0 {
			return ExpectRequestToSucceed(tc.RequestTimeout, url, addr, outcome.expectedContent, opts...)
		}

		resp, err := Get(Request{
			URL:     url,
			Address: addr,
			Headers: TestOptions(opts...).requestHeaders,
			Timeout: tc.RequestTimeout,
		}, opts...)

		if outcome.expectFailure {
			if err != nil || resp.StatusCode != http.StatusOK {
				return nil //nolint:nilerr // transport error counts as a failed request
			}
			return errors.New("expected request to fail but got HTTP 200 instead")
		}

		if err != nil {
			return fmt.Errorf("GET %s: %w", url, err)
		}
		if resp.StatusCode != outcome.expectedStatus {
			return fmt.Errorf("GET %s: expected status %d, got %d", url, outcome.expectedStatus, resp.StatusCode)
		}
		return nil
	}).
		WithTimeout(tc.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}
