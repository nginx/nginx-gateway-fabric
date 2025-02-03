package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation/validationfakes"
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
			name:           "valid HTTP extension ref filter",
		},
		{
			filter: Filter{
				RouteType:  RouteTypeHTTP,
				FilterType: FilterRequestMirror,
			},
			expectErrCount: 1,
			name:           "unsupported HTTP filter type",
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
