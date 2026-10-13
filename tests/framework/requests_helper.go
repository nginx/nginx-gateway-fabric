package framework

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// VerifyRequest polls until the HTTP request to url produces the expected outcome.
// Pass http.StatusOK (200) with a non-empty expectedContent to assert a successful response.
// Pass 0 for expectedStatus to assert that the request fails (transport error or any non-200 response).
func VerifyRequest(tc TimeoutConfig, url, addr string, expectedStatus int, expectedContent string, opts ...Option) {
	GinkgoHelper()
	Eventually(func() error {
		if expectedStatus == http.StatusOK {
			return ExpectRequestToSucceed(tc.RequestTimeout, url, addr, expectedContent, opts...)
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
		return fmt.Errorf("expected request to fail but got HTTP 200 instead")
	}).
		WithTimeout(tc.RequestTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}
