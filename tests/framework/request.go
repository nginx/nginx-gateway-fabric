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
)

// Get sends a GET request to the specified url.
// It resolves to the specified address instead of using DNS.
// The status and body of the response is returned, or an error.
func Get(
	url, address string,
	timeout time.Duration,
	headers, queryParams map[string]string,
) (int, string, error) {
	resp, err := makeRequest(http.MethodGet, url, address, nil, timeout, headers, queryParams)
	if err != nil {
		return 0, "", err
	}

	defer resp.Body.Close()

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return resp.StatusCode, "", err
	}

	return resp.StatusCode, body.String(), nil
}

// Post sends a POST request to the specified url with the body as the payload.
// It resolves to the specified address instead of using DNS.
func Post(
	url, address string,
	body io.Reader,
	timeout time.Duration,
	headers, queryParams map[string]string,
) (*http.Response, error) {
	return makeRequest(http.MethodPost, url, address, body, timeout, headers, queryParams)
}

func makeRequest(
	method, url, address string,
	body io.Reader,
	timeout time.Duration,
	headers, queryParams map[string]string,
) (*http.Response, error) {
	dialer := &net.Dialer{}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("transport is not of type *http.Transport")
	}

	transport.DialContext = func(
		ctx context.Context,
		network,
		addr string,
	) (net.Conn, error) {
		split := strings.Split(addr, ":")
		port := split[len(split)-1]
		return dialer.DialContext(ctx, network, fmt.Sprintf("%s:%s", address, port))
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	if queryParams != nil {
		q := req.URL.Query()
		for key, value := range queryParams {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	var resp *http.Response
	if strings.HasPrefix(url, "https") {
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("transport is not of type *http.Transport")
		}

		customTransport := transport.Clone()
		// similar to how in our examples with https requests we run our curl command
		// we turn off verification of the certificate, we do the same here
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // for https test traffic
		client := &http.Client{Transport: customTransport}
		resp, err = client.Do(req)
	} else {
		resp, err = http.DefaultClient.Do(req)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}
