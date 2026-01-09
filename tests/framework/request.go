package framework

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
)

type Response struct {
	Headers    http.Header
	Body       string
	StatusCode int
}

type Request struct {
	Body        io.Reader
	Headers     map[string]string
	QueryParams map[string]string
	URL         string
	Address     string
	Timeout     time.Duration
}

// Get sends a GET request to the specified url.
// It resolves to the specified address instead of using DNS.
// It returns the response body, headers, and status code.
func Get(request Request, opts ...Option) (Response, error) {
	options := TestOptions(opts...)

	resp, err := makeRequest(http.MethodGet, request, opts...)
	if err != nil {
		if options.logEnabled {
			GinkgoWriter.Printf(
				"ERROR occurred during getting response, error: %s\nReturning status: 0, body: ''\n",
				err,
			)
		}

		return Response{StatusCode: 0}, err
	}
	defer resp.Body.Close()

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		GinkgoWriter.Printf("ERROR in Body content: %v returning body: ''\n", err)
		return Response{StatusCode: resp.StatusCode}, err
	}
	if options.logEnabled {
		printResponseBody(body)
	}

	return Response{
		Body:       body.String(),
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
	}, nil
}

func printResponseBody(body *bytes.Buffer) {
	maxLogBodyBytes := 2048
	bs := body.Bytes()
	if len(bs) <= maxLogBodyBytes {
		GinkgoWriter.Printf("Successfully received response and parsed body (%d bytes): %s\n", len(bs), string(bs))
	} else {
		GinkgoWriter.Printf(
			"Successfully received response and parsed body (%d bytes, showing first %d): %s...\n",
			len(bs),
			maxLogBodyBytes,
			string(bs[:maxLogBodyBytes]),
		)
	}
}

// Post sends a POST request to the specified url with the body as the payload.
// It resolves to the specified address instead of using DNS.
func Post(request Request) (*http.Response, error) {
	response, err := makeRequest(http.MethodPost, request)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting response, error: %s\n", err)
	}

	return response, err
}

func makeRequest(method string, request Request, opts ...Option) (*http.Response, error) {
	dialer := &net.Dialer{}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("transport is not of type *http.Transport")
	}

	customTransport := transport.Clone()
	customTransport.DialContext = func(
		ctx context.Context,
		network,
		addr string,
	) (net.Conn, error) {
		split := strings.Split(addr, ":")
		port := split[len(split)-1]
		return dialer.DialContext(ctx, network, fmt.Sprintf("%s:%s", request.Address, port))
	}

	options := TestOptions(opts...)
	if options.logEnabled {
		requestDetails := fmt.Sprintf(
			"Method: %s, URL: %s, Address: %s, Headers: %v, QueryParams: %v\n",
			strings.ToUpper(method),
			request.URL,
			request.Address,
			request.Headers,
			request.QueryParams,
		)
		GinkgoWriter.Printf("\nSending request: %s", requestDetails)
	}

	var (
		req *http.Request
		err error
	)
	if options.withContext {
		ctx, cancel := context.WithTimeout(context.Background(), request.Timeout)
		defer cancel()

		req, err = http.NewRequestWithContext(ctx, method, request.URL, request.Body)
	} else {
		// Create request without a short-lived context; rely on http.Client.Timeout
		// so that the response body can be fully read by the caller without
		// being canceled when this function returns.
		req, err = http.NewRequest(method, request.URL, request.Body) //nolint:noctx
	}
	if err != nil {
		return nil, err
	}

	for key, value := range request.Headers {
		req.Header.Add(key, value)
	}

	if request.QueryParams != nil {
		q := req.URL.Query()
		for key, value := range request.QueryParams {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	var resp *http.Response
	if strings.HasPrefix(request.URL, "https") {
		// similar to how in our examples with https requests we run our curl command
		// we turn off verification of the certificate, we do the same here
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // for https test traffic
	}

	client := &http.Client{
		Transport: customTransport,
	}
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ExpectRequestToSucceed(
	timeout time.Duration,
	appURL,
	address,
	responseBodyMessage string,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	request := Request{
		Headers: options.requestHeaders,
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, err := Get(request)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status was not 200, got %d: %w", resp.StatusCode, err)
	}

	if !strings.Contains(resp.Body, responseBodyMessage) {
		return fmt.Errorf("expected response body to contain correct body message, got: %s", resp.Body)
	}

	return err
}

// The function is expecting the request to fail (hence the name) because NGINX is not there to route the request.
// The purpose of the graceful recovery test is to simulate various failure scenarios including NGINX
// container restarts, NGF pod restarts, and Kubernetes node restarts to show the system can recover
// after these real world scenarios and resume serving application traffic after recovery.
// In this case, we verify that our requests fail and then that eventually are successful again - verifying that
// NGINX went down and came back up again.
// We only want an error returned from this particular function if it does not appear that NGINX has
// stopped serving traffic.
func ExpectRequestToFail(timeout time.Duration, appURL, address string) error {
	request := Request{
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, err := Get(request)
	if resp.StatusCode != 0 {
		return errors.New("expected http status to be 0")
	}

	if resp.Body != "" {
		return fmt.Errorf("expected response body to be empty, instead received: %s", resp.Body)
	}

	if err == nil {
		return errors.New("expected request to error")
	}

	return nil
}

func ExpectUnauthenticatedRequest(timeout time.Duration, appURL, address string, opts ...Option) error {
	options := LogOptions(opts...)
	request := Request{
		Headers: options.requestHeaders,
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, _ := Get(request)
	if resp.StatusCode != http.StatusUnauthorized {
		return errors.New("expected http status to be 401")
	}

	return nil
}

func Expect500Response(timeout time.Duration, appURL, address string, opts ...Option) error {
	options := LogOptions(opts...)
	request := Request{
		Headers: options.requestHeaders,
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, _ := Get(request)
	if resp.StatusCode != http.StatusInternalServerError {
		return errors.New("expected http status to be 500")
	}

	return nil
}
