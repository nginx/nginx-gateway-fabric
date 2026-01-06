package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

type TestHeader func(*TestHeaders)

type TestHeaders = map[string]string

func WithTestHeaders(headers map[string]string) TestHeader {
	return func(hdrs *TestHeaders) {
		*hdrs = headers
	}
}

func RequestWithTestHeaders(hdrs ...TestHeader) TestHeaders {
	var headers TestHeaders
	for _, hdr := range hdrs {
		hdr(&headers)
	}

	return headers
}

func ExpectRequestToSucceed(
	timeout time.Duration,
	appURL, address string,
	responseBodyMessage string,
	hdrs ...TestHeader,
) error {
	headers := RequestWithTestHeaders(hdrs...)
	request := framework.Request{
		Headers: headers,
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, err := framework.Get(request)

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
	request := framework.Request{
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, err := framework.Get(request)
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

func ExpectUnauthorizedRequest(timeout time.Duration, appURL, address string, hdrs ...TestHeader) error {
	headers := RequestWithTestHeaders(hdrs...)
	request := framework.Request{
		Headers: headers,
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, _ := framework.Get(request)
	if resp.StatusCode != http.StatusUnauthorized {
		return errors.New("expected http status to be 401")
	}

	return nil
}

func Expect500Response(timeout time.Duration, appURL, address string, hdrs ...TestHeader) error {
	headers := RequestWithTestHeaders(hdrs...)
	request := framework.Request{
		Headers: headers,
		URL:     appURL,
		Address: address,
		Timeout: timeout,
	}
	resp, _ := framework.Get(request)
	if resp.StatusCode != http.StatusInternalServerError {
		return errors.New("expected http status to be 500")
	}

	return nil
}

func ExpectGRPCRequestToSucceed(
	timeout time.Duration,
	address string,
	hdrs ...TestHeader,
) error {
	headers := RequestWithTestHeaders(hdrs...)
	request := framework.GRPCRequest{
		Headers: headers,
		Address: address,
		Timeout: timeout,
	}
	err := framework.SendGRPCRequest(request)
	if err != nil {
		GinkgoWriter.Printf("ERROR:gRPC request returned error: %v\n", err)
		return fmt.Errorf("expected gRPC request to succeed, but got error: %w", err)
	}

	return nil
}

func ExpectUnauthorizedGRPCRequest(
	timeout time.Duration,
	address string,
	hdrs ...TestHeader,
) error {
	headers := RequestWithTestHeaders(hdrs...)
	request := framework.GRPCRequest{
		Headers: headers,
		Address: address,
		Timeout: timeout,
	}
	err := framework.SendGRPCRequest(request)

	if err == nil {
		GinkgoWriter.Printf("ERROR: gRPC request was successful when failure was expected\n")
		return errors.New("expected Unauthenticated error, but gRPC request succeeded")
	}

	// Verify the gRPC status code is Unauthenticated (HTTP 401 equivalent).
	if status.Code(err) != codes.Unauthenticated {
		GinkgoWriter.Printf("ERROR: expected Unauthenticated, got %s (err: %v)\n", status.Code(err), err)
		return fmt.Errorf("expected gRPC code %s, got %s", codes.Unauthenticated, status.Code(err))
	}

	return nil
}

// ConditionView and ControllerStatusView provide a minimal, unified view used by the generic checker.
type ConditionView struct {
	Type   string
	Status metav1.ConditionStatus
	Reason string
}

type ControllerStatusView struct {
	ControllerName v1.GatewayController
	Conditions     []ConditionView
}

// Filter is a type set constraint for supported NGF filter types.
// This improves compile-time safety for the generic checker.
type Filter interface {
	ngfAPI.SnippetsFilter | ngfAPI.AuthenticationFilter
}

// CheckFilterAccepted is a generic acceptance checker for different NGF filter types.
// - T: the concrete filter type (e.g., ngfAPI.SnippetsFilter, ngfAPI.AuthenticationFilter)
// - getControllers: adapter that extracts controller statuses from T into ControllerStatusView
// - expectedCondType/expectedCondReason: the condition type and reason to assert (passed in for flexibility).
func CheckFilterAccepted[T Filter](
	filter T,
	ngfControllerName string,
	getControllers func(T) []ControllerStatusView,
	expectedCondType string,
	expectedCondReason string,
) error {
	controllers := getControllers(filter)
	if len(controllers) != 1 {
		tooManyStatusesErr := fmt.Errorf("filter has %d controller statuses, expected 1", len(controllers))
		GinkgoWriter.Printf("ERROR: %v\n", tooManyStatusesErr)
		return tooManyStatusesErr
	}

	filterStatus := controllers[0]
	if filterStatus.ControllerName != (v1.GatewayController)(ngfControllerName) {
		wrongNameErr := fmt.Errorf(
			"expected controller name to be %s, got %s",
			ngfControllerName,
			filterStatus.ControllerName,
		)
		GinkgoWriter.Printf("ERROR: %v\n", wrongNameErr)
		return wrongNameErr
	}

	if len(filterStatus.Conditions) == 0 {
		noCondErr := fmt.Errorf("expected at least one condition, got 0")
		GinkgoWriter.Printf("ERROR: %v\n", noCondErr)
		return noCondErr
	}

	condition := filterStatus.Conditions[0]
	if condition.Type != expectedCondType {
		wrongTypeErr := fmt.Errorf("expected condition type to be %s, got %s", expectedCondType, condition.Type)
		GinkgoWriter.Printf("ERROR: %v\n", wrongTypeErr)
		return wrongTypeErr
	}

	if condition.Status != metav1.ConditionTrue {
		wrongStatusErr := fmt.Errorf("expected condition status to be %s, got %s", metav1.ConditionTrue, condition.Status)
		GinkgoWriter.Printf("ERROR: %v\n", wrongStatusErr)
		return wrongStatusErr
	}

	if condition.Reason != expectedCondReason {
		wrongReasonErr := fmt.Errorf("expected condition reason to be %s, got %s", expectedCondReason, condition.Reason)
		GinkgoWriter.Printf("ERROR: %v\n", wrongReasonErr)
		return wrongReasonErr
	}

	return nil
}

// Adapters: extract ControllerStatusView slices from concrete NGF filter types.
func snippetsFilterControllers(sf ngfAPI.SnippetsFilter) []ControllerStatusView {
	out := make([]ControllerStatusView, 0, len(sf.Status.Controllers))
	for _, st := range sf.Status.Controllers {
		cv := make([]ConditionView, 0, len(st.Conditions))
		for _, c := range st.Conditions {
			cv = append(cv, ConditionView{Type: c.Type, Status: c.Status, Reason: c.Reason})
		}
		out = append(out, ControllerStatusView{ControllerName: st.ControllerName, Conditions: cv})
	}
	return out
}

func authenticationFilterControllers(af ngfAPI.AuthenticationFilter) []ControllerStatusView {
	out := make([]ControllerStatusView, 0, len(af.Status.Controllers))
	for _, st := range af.Status.Controllers {
		cv := make([]ConditionView, 0, len(st.Conditions))
		for _, c := range st.Conditions {
			cv = append(cv, ConditionView{Type: c.Type, Status: c.Status, Reason: c.Reason})
		}
		out = append(out, ControllerStatusView{ControllerName: st.ControllerName, Conditions: cv})
	}
	return out
}
