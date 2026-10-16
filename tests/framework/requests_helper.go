package framework

import (
	"errors"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// RequestOutcome is the result VerifyRequest waits for.
type RequestOutcome struct {
	expectedContent string
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
		if !outcome.expectFailure {
			return ExpectRequestToSucceed(tc.RequestTimeout, url, addr, outcome.expectedContent, opts...)
		}

		resp, err := Get(Request{
			URL:     url,
			Address: addr,
			Timeout: tc.RequestTimeout,
		}, opts...)
		if err != nil {
			return nil //nolint:nilerr // transport error counts as a failed request
		}
		if resp.StatusCode != http.StatusOK {
			return nil
		}
		return errors.New("expected request to fail but got HTTP 200 instead")
	}).
		WithTimeout(tc.RequestTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}
