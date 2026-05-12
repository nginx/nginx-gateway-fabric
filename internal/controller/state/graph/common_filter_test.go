package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestValidateFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filter         Filter
		name           string
		expectErrCount int
	}{
		{
			filter: Filter{
				RouteType:       RouteTypeHTTP,
				FilterType:      FilterRequestRedirect,
				RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
			},
			expectErrCount: 0,
			name:           "valid HTTP redirect filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterURLRewrite,
				URLRewrite: &gatewayv1.HTTPURLRewriteFilter{},
			},
			expectErrCount: 0,
			name:           "valid HTTP rewrite filter",
		},
		{
			filter: Filter{
				RouteType:             RouteTypeHTTP,
				FilterType:            FilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
			},
			expectErrCount: 0,
			name:           "valid HTTP request header modifiers filter",
		},
		{
			filter: Filter{
				RouteType:              RouteTypeHTTP,
				FilterType:             FilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
			},
			expectErrCount: 0,
			name:           "valid HTTP response header modifiers filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterExtensionRef,
				ExtensionRef: &gatewayv1.LocalObjectReference{
					Group: ngfAPI.GroupName,
					Kind:  kinds.SnippetsFilter,
					Name:  "sf",
				},
			},
			expectErrCount: 0,
			name:           "valid SnippetsFilter HTTP extension ref filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterExtensionRef,
				ExtensionRef: &gatewayv1.LocalObjectReference{
					Group: ngfAPI.GroupName,
					Kind:  kinds.AuthenticationFilter,
					Name:  "af",
				},
			},
			expectErrCount: 0,
			name:           "valid AuthenticationFilter HTTP extension ref filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: "invalid-filter",
			},
			expectErrCount: 1,
			name:           "unsupported HTTP filter type",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterCORS,
				CORS: &gatewayv1.HTTPCORSFilter{
					AllowOrigins: []gatewayv1.CORSOrigin{"https://example.com"},
					AllowMethods: []gatewayv1.HTTPMethodWithWildcard{"GET", "POST"},
				},
			},
			expectErrCount: 0,
			name:           "valid HTTP CORS filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterCORS,
				CORS:       nil,
			},
			expectErrCount: 1,
			name:           "invalid HTTP CORS filter with nil value",
		},
		{
			filter: Filter{
				RouteType:             RouteTypeGRPC,
				FilterType:            FilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
			},
			expectErrCount: 0,
			name:           "valid GRPC request header modifiers filter",
		},
		{
			filter: Filter{
				RouteType:              RouteTypeGRPC,
				FilterType:             FilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
			},
			expectErrCount: 0,
			name:           "valid GRPC response header modifiers filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeGRPC,
				FilterType: FilterExtensionRef,
				ExtensionRef: &gatewayv1.LocalObjectReference{
					Group: ngfAPI.GroupName,
					Kind:  kinds.SnippetsFilter,
					Name:  "sf",
				},
			},
			expectErrCount: 0,
			name:           "valid GRPC extension ref filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeGRPC,
				FilterType: FilterURLRewrite,
			},
			expectErrCount: 1,
			name:           "unsupported GRPC filter type",
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			allErrs := validateFilter(&validationfakes.FakeHTTPFieldsValidator{}, test.filter, filterPath)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestValidateFilterMirror(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filter         Filter
		name           string
		expectErrCount int
	}{
		{
			filter: Filter{
				RouteType:     RouteTypeHTTP,
				FilterType:    FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{},
			},
			expectErrCount: 0,
			name:           "valid HTTP mirror filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
					Percent: helpers.GetPointer(int32(50)),
				},
			},
			expectErrCount: 0,
			name:           "valid HTTP mirror filter with percentage set",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
					Fraction: &gatewayv1.Fraction{
						Numerator:   1,
						Denominator: helpers.GetPointer(int32(2)),
					},
				},
			},
			expectErrCount: 0,
			name:           "valid HTTP mirror filter with fraction set",
		},
		{
			filter: Filter{
				RouteType:     RouteTypeHTTP,
				FilterType:    FilterRequestMirror,
				RequestMirror: nil,
			},
			expectErrCount: 1,
			name:           "invalid nil HTTP mirror filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
					Percent: helpers.GetPointer(int32(5)),
					Fraction: &gatewayv1.Fraction{
						Numerator:   1,
						Denominator: helpers.GetPointer(int32(3)),
					},
				},
			},
			expectErrCount: 1,
			name:           "invalid HTTP mirror filter both percent and fraction set",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
					Fraction: &gatewayv1.Fraction{
						Numerator:   1,
						Denominator: helpers.GetPointer(int32(0)),
					},
				},
			},
			expectErrCount: 1,
			name:           "invalid HTTP mirror filter, fraction denominator value must be greater than 0",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
					Fraction: &gatewayv1.Fraction{
						Numerator:   -1,
						Denominator: helpers.GetPointer(int32(2)),
					},
				},
			},
			expectErrCount: 1,
			name:           "invalid HTTP mirror filter, fraction numerator value must be greater than or equal to 0",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
					Fraction: &gatewayv1.Fraction{
						Numerator:   5,
						Denominator: helpers.GetPointer(int32(2)),
					},
				},
			},
			expectErrCount: 1,
			name:           "invalid HTTP mirror filter, fraction numerator value must be less than denominator",
		},
		{
			filter: Filter{
				RouteType:     RouteTypeGRPC,
				FilterType:    FilterRequestMirror,
				RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{},
			},
			expectErrCount: 0,
			name:           "valid GRPC mirror filter",
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			allErrs := validateFilter(&validationfakes.FakeHTTPFieldsValidator{}, test.filter, filterPath)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestValidateFilterResponseHeaderModifier(t *testing.T) {
	t.Parallel()

	createAllValidValidator := func() *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}
		return v
	}

	tests := []struct {
		filter         gatewayv1.HTTPRouteFilter
		validator      *validationfakes.FakeHTTPFieldsValidator
		name           string
		expectErrCount int
	}{
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "MyBespokeHeader", Value: "my-value"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "Accept-Encoding", Value: "gzip"},
					},
					Remove: []string{"Cache-Control"},
				},
			},
			expectErrCount: 0,
			name:           "valid response header modifier filter",
		},
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type:                   gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: nil,
			},
			expectErrCount: 1,
			name:           "nil response header modifier filter",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderNameReturns(errors.New("Invalid header"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Add: []gatewayv1.HTTPHeader{
						{Name: "$var_name", Value: "gzip"},
					},
				},
			},
			expectErrCount: 1,
			name:           "response header modifier filter with invalid add",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderNameReturns(errors.New("Invalid header"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Remove: []string{"$var-name"},
				},
			},
			expectErrCount: 1,
			name:           "response header modifier filter with invalid remove",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderValueReturns(errors.New("Invalid header value"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Add: []gatewayv1.HTTPHeader{
						{Name: "Accept-Encoding", Value: "yhu$"},
					},
				},
			},
			expectErrCount: 1,
			name:           "response header modifier filter with invalid header value",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderValueReturns(errors.New("Invalid header value"))
				v.ValidateFilterHeaderNameReturns(errors.New("Invalid header"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "Host", Value: "my_host"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "}90yh&$", Value: "gzip$"},
						{Name: "}67yh&$", Value: "compress$"},
					},
					Remove: []string{"Cache-Control$}"},
				},
			},
			expectErrCount: 7,
			name:           "response header modifier filter all fields invalid",
		},
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "MyBespokeHeader", Value: "my-value"},
						{Name: "mYbespokeHEader", Value: "duplicate"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "Accept-Encoding", Value: "gzip"},
						{Name: "accept-encodING", Value: "gzip"},
					},
					Remove: []string{"Cache-Control", "cache-control"},
				},
			},
			expectErrCount: 3,
			name:           "response header modifier filter not unique names",
		},
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "Content-Length", Value: "163"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "Content-Type", Value: "text/plain"},
					},
					Remove: []string{"X-Pad"},
				},
			},
			expectErrCount: 3,
			name:           "response header modifier filter with disallowed header name",
		},
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "X-Accel-Redirect", Value: "/protected/iso.img"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "X-Accel-Limit-Rate", Value: "1024"},
					},
					Remove: []string{"X-Accel-Charset"},
				},
			},
			expectErrCount: 3,
			name:           "response header modifier filter with disallowed header name prefix",
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateFilterResponseHeaderModifier(
				test.validator, test.filter.ResponseHeaderModifier, filterPath,
			)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestValidateFilterRequestHeaderModifier(t *testing.T) {
	t.Parallel()

	createAllValidValidator := func() *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}
		return v
	}

	tests := []struct {
		filter         gatewayv1.HTTPRouteFilter
		validator      *validationfakes.FakeHTTPFieldsValidator
		name           string
		expectErrCount int
	}{
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "MyBespokeHeader", Value: "my-value"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "Accept-Encoding", Value: "gzip"},
					},
					Remove: []string{"Cache-Control"},
				},
			},
			expectErrCount: 0,
			name:           "valid request header modifier filter",
		},
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type:                  gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: nil,
			},
			expectErrCount: 1,
			name:           "nil request header modifier filter",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderNameReturns(errors.New("Invalid header"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Add: []gatewayv1.HTTPHeader{
						{Name: "$var_name", Value: "gzip"},
					},
				},
			},
			expectErrCount: 1,
			name:           "request header modifier filter with invalid add",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderNameReturns(errors.New("Invalid header"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Remove: []string{"$var-name"},
				},
			},
			expectErrCount: 1,
			name:           "request header modifier filter with invalid remove",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderValueReturns(errors.New("Invalid header value"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Add: []gatewayv1.HTTPHeader{
						{Name: "Accept-Encoding", Value: "yhu$"},
					},
				},
			},
			expectErrCount: 1,
			name:           "request header modifier filter with invalid header value",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				v := createAllValidValidator()
				v.ValidateFilterHeaderValueReturns(errors.New("Invalid header value"))
				v.ValidateFilterHeaderNameReturns(errors.New("Invalid header"))
				return v
			}(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "Host", Value: "my_host"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "}90yh&$", Value: "gzip$"},
						{Name: "}67yh&$", Value: "compress$"},
					},
					Remove: []string{"Cache-Control$}"},
				},
			},
			expectErrCount: 7,
			name:           "request header modifier filter all fields invalid",
		},
		{
			validator: createAllValidValidator(),
			filter: gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{
						{Name: "MyBespokeHeader", Value: "my-value"},
						{Name: "mYbespokeHEader", Value: "duplicate"},
					},
					Add: []gatewayv1.HTTPHeader{
						{Name: "Accept-Encoding", Value: "gzip"},
						{Name: "accept-encodING", Value: "gzip"},
					},
					Remove: []string{"Cache-Control", "cache-control"},
				},
			},
			expectErrCount: 3,
			name:           "request header modifier filter not unique names",
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateFilterHeaderModifier(
				test.validator, test.filter.RequestHeaderModifier, filterPath,
			)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestValidateFilterExternalAuth(t *testing.T) {
	t.Parallel()

	port := gatewayv1.PortNumber(80)

	invalidHeaderValidator := &validationfakes.FakeHTTPFieldsValidator{}
	invalidHeaderValidator.ValidateFilterHeaderNameCalls(func(name string) error {
		if name == "invalid header" {
			return errors.New("invalid header name")
		}
		return nil
	})

	invalidPathValidator := &validationfakes.FakeHTTPFieldsValidator{}
	invalidPathValidator.ValidatePathCalls(func(path string) error {
		if path == "/bad path" {
			return errors.New("invalid path")
		}
		return nil
	})

	invalidHeaderAndPathValidator := &validationfakes.FakeHTTPFieldsValidator{}
	invalidHeaderAndPathValidator.ValidateFilterHeaderNameCalls(func(name string) error {
		if name == "invalid header" {
			return errors.New("invalid header name")
		}
		return nil
	})
	invalidHeaderAndPathValidator.ValidatePathCalls(func(path string) error {
		if path == "/bad path" {
			return errors.New("invalid path")
		}
		return nil
	})

	tests := []struct {
		validator      validation.HTTPFieldsValidator
		filter         *gatewayv1.HTTPExternalAuthFilter
		name           string
		expectErrCount int
	}{
		{
			name:      "valid HTTP external auth filter with no httpAuthConfig",
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
			},
			expectErrCount: 0,
		},
		{
			name:      "valid HTTP external auth filter with both request and response headers",
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
				HTTPAuthConfig: &gatewayv1.HTTPAuthConfig{
					AllowedRequestHeaders:  []string{"X-Custom-Token"},
					AllowedResponseHeaders: []string{"X-Auth-Status", "X-Forwarded-Groups"},
				},
			},
			expectErrCount: 0,
		},
		{
			name:           "nil filter",
			validator:      &validationfakes.FakeHTTPFieldsValidator{},
			filter:         nil,
			expectErrCount: 1,
		},
		{
			name:      "GRPC protocol is not supported",
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthGRPCProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
			},
			expectErrCount: 1,
		},
		{
			name:      "invalid header name in allowedResponseHeaders",
			validator: invalidHeaderValidator,
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
				HTTPAuthConfig: &gatewayv1.HTTPAuthConfig{
					AllowedResponseHeaders: []string{"valid-header", "invalid header"},
				},
			},
			expectErrCount: 1,
		},
		{
			name:      "invalid header name in allowedRequestHeaders",
			validator: invalidHeaderValidator,
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
				HTTPAuthConfig: &gatewayv1.HTTPAuthConfig{
					AllowedRequestHeaders: []string{"invalid header"},
				},
			},
			expectErrCount: 1,
		},
		{
			name:      "valid non-empty path in httpAuthConfig passes validation",
			validator: invalidPathValidator,
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
				HTTPAuthConfig: &gatewayv1.HTTPAuthConfig{
					Path: "/check",
				},
			},
			expectErrCount: 0,
		},
		{
			name:      "invalid path in httpAuthConfig returns one error",
			validator: invalidPathValidator,
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
				HTTPAuthConfig: &gatewayv1.HTTPAuthConfig{
					Path: "/bad path",
				},
			},
			expectErrCount: 1,
		},
		{
			name:      "invalid path and invalid request and response headers accumulate three errors",
			validator: invalidHeaderAndPathValidator,
			filter: &gatewayv1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: gatewayv1.BackendObjectReference{
					Name: "auth-svc",
					Port: &port,
				},
				HTTPAuthConfig: &gatewayv1.HTTPAuthConfig{
					Path:                   "/bad path",
					AllowedRequestHeaders:  []string{"invalid header"},
					AllowedResponseHeaders: []string{"invalid header"},
				},
			},
			expectErrCount: 3,
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			allErrs := validateFilterExternalAuth(test.validator, test.filter, filterPath)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestProcessRouteRuleFiltersDuplicateExternalAuth(t *testing.T) {
	t.Parallel()

	port := gatewayv1.PortNumber(80)

	validExternalAuthFilter := Filter{
		FilterType: FilterExternalAuth,
		ExternalAuth: &gatewayv1.HTTPExternalAuthFilter{
			ExternalAuthProtocol: gatewayv1.HTTPRouteExternalAuthHTTPProtocol,
			BackendRef: gatewayv1.BackendObjectReference{
				Name: "auth-svc",
				Port: &port,
			},
		},
	}

	tests := []struct {
		name            string
		filters         []Filter
		expectValid     bool
		expectWarnCount int
	}{
		{
			name:            "single external auth filter is accepted with no warnings",
			filters:         []Filter{validExternalAuthFilter},
			expectValid:     true,
			expectWarnCount: 0,
		},
		{
			name:            "duplicate external auth filters produce a warning for the second filter",
			filters:         []Filter{validExternalAuthFilter, validExternalAuthFilter},
			expectValid:     true,
			expectWarnCount: 1,
		},
		{
			name:            "three external auth filters produce two warnings for the second and third filters",
			filters:         []Filter{validExternalAuthFilter, validExternalAuthFilter, validExternalAuthFilter},
			expectValid:     true,
			expectWarnCount: 2,
		},
	}

	path := field.NewPath("test")
	validator := &validationfakes.FakeHTTPFieldsValidator{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result, errs := processRouteRuleFilters(test.filters, path, validator, nil)
			g.Expect(result.Valid).To(Equal(test.expectValid))
			g.Expect(errs.warn).To(HaveLen(test.expectWarnCount))
		})
	}
}

func TestConvertGRPCFilters(t *testing.T) {
	t.Parallel()

	requestHeaderFilter1 := &gatewayv1.HTTPHeaderFilter{
		Remove: []string{"request-1"},
	}
	requestHeaderFilter2 := &gatewayv1.HTTPHeaderFilter{
		Remove: []string{"request-2"},
	}

	tests := []struct {
		name        string
		grpcFilters []gatewayv1.GRPCRouteFilter
		expFilters  []Filter
	}{
		{
			name:        "nil filters",
			grpcFilters: nil,
			expFilters:  []Filter{},
		},
		{
			name:        "empty filters",
			grpcFilters: []gatewayv1.GRPCRouteFilter{},
			expFilters:  []Filter{},
		},
		{
			name: "all filter types",
			grpcFilters: []gatewayv1.GRPCRouteFilter{
				{
					Type:                  gatewayv1.GRPCRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter1,
				},
				{
					Type:                  gatewayv1.GRPCRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter2, // duplicates are added
				},
				{
					Type:                   gatewayv1.GRPCRouteFilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
				},
				{
					Type:          gatewayv1.GRPCRouteFilterRequestMirror,
					RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{},
				},
				{
					Type:         gatewayv1.GRPCRouteFilterExtensionRef,
					ExtensionRef: &gatewayv1.LocalObjectReference{},
				},
			},
			expFilters: []Filter{
				{
					RouteType:             RouteTypeGRPC,
					FilterType:            FilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter1,
				},
				{
					RouteType:             RouteTypeGRPC,
					FilterType:            FilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter2,
				},
				{
					RouteType:              RouteTypeGRPC,
					FilterType:             FilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
				},
				{
					RouteType:     RouteTypeGRPC,
					FilterType:    FilterRequestMirror,
					RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{},
				},
				{
					RouteType:    RouteTypeGRPC,
					FilterType:   FilterExtensionRef,
					ExtensionRef: &gatewayv1.LocalObjectReference{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			convertedFilters := convertGRPCRouteFilters(test.grpcFilters)
			g.Expect(convertedFilters).To(Equal(test.expFilters))
		})
	}
}

func TestConvertHTTPFilters(t *testing.T) {
	t.Parallel()

	requestHeaderFilter1 := &gatewayv1.HTTPHeaderFilter{
		Remove: []string{"request-1"},
	}
	requestHeaderFilter2 := &gatewayv1.HTTPHeaderFilter{
		Remove: []string{"request-2"},
	}

	tests := []struct {
		name        string
		httpFilters []gatewayv1.HTTPRouteFilter
		expFilters  []Filter
	}{
		{
			name:        "nil filters",
			httpFilters: nil,
			expFilters:  []Filter{},
		},
		{
			name:        "empty filters",
			httpFilters: []gatewayv1.HTTPRouteFilter{},
			expFilters:  []Filter{},
		},
		{
			name: "all filter types",
			httpFilters: []gatewayv1.HTTPRouteFilter{
				{
					Type:                  gatewayv1.HTTPRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter1,
				},
				{
					Type:                  gatewayv1.HTTPRouteFilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter2, // duplicates are added
				},
				{
					Type:                   gatewayv1.HTTPRouteFilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
				},
				{
					Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
					RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
				},
				{
					Type:       gatewayv1.HTTPRouteFilterURLRewrite,
					URLRewrite: &gatewayv1.HTTPURLRewriteFilter{},
				},
				{
					Type:          gatewayv1.HTTPRouteFilterRequestMirror,
					RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{},
				},
				{
					Type:         gatewayv1.HTTPRouteFilterExtensionRef,
					ExtensionRef: &gatewayv1.LocalObjectReference{},
				},
			},
			expFilters: []Filter{
				{
					RouteType:             RouteTypeHTTP,
					FilterType:            FilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter1,
				},
				{
					RouteType:             RouteTypeHTTP,
					FilterType:            FilterRequestHeaderModifier,
					RequestHeaderModifier: requestHeaderFilter2,
				},
				{
					RouteType:              RouteTypeHTTP,
					FilterType:             FilterResponseHeaderModifier,
					ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{},
				},
				{
					RouteType:       RouteTypeHTTP,
					FilterType:      FilterRequestRedirect,
					RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
				},
				{
					RouteType:  RouteTypeHTTP,
					FilterType: FilterURLRewrite,
					URLRewrite: &gatewayv1.HTTPURLRewriteFilter{},
				},
				{
					RouteType:     RouteTypeHTTP,
					FilterType:    FilterRequestMirror,
					RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{},
				},
				{
					RouteType:    RouteTypeHTTP,
					FilterType:   FilterExtensionRef,
					ExtensionRef: &gatewayv1.LocalObjectReference{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			convertedFilters := convertHTTPRouteFilters(test.httpFilters)
			g.Expect(convertedFilters).To(Equal(test.expFilters))
		})
	}
}
