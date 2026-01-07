package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestValidateExtensionRefFilter(t *testing.T) {
	t.Parallel()
	testPath := field.NewPath("test")

	validTests := []struct {
		ref          *v1.LocalObjectReference
		name         string
		errSubString []string
		expErrCount  int
	}{
		{
			name:        "nil ref",
			ref:         nil,
			expErrCount: 1,
			errSubString: []string{
				`test.extensionRef: Required value: cannot be nil`,
			},
		},
		{
			name:        "empty ref",
			ref:         &v1.LocalObjectReference{},
			expErrCount: 3,
			errSubString: []string{
				`test.extensionRef: Required value: name cannot be empty`,
				`test.extensionRef: Unsupported value: ""`,
				`supported values: "gateway.nginx.org"`,
				`test.extensionRef: Unsupported value: ""`,
				`supported values: "SnippetsFilter", "AuthenticationFilter"`,
			},
		},
		{
			name: "ref missing name",
			ref: &v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
			},
			expErrCount: 1,
			errSubString: []string{
				`test.extensionRef: Required value: name cannot be empty`,
			},
		},
		{
			name: "ref unsupported group",
			ref: &v1.LocalObjectReference{
				Name:  v1.ObjectName("filter"),
				Group: "unsupported",
				Kind:  kinds.SnippetsFilter,
			},
			expErrCount: 1,
			errSubString: []string{
				`test.extensionRef: Unsupported value: "unsupported"`,
				`supported values: "gateway.nginx.org"`,
			},
		},
		{
			name: "ref unsupported kind",
			ref: &v1.LocalObjectReference{
				Name:  v1.ObjectName("filter"),
				Group: ngfAPI.GroupName,
				Kind:  "unsupported",
			},
			expErrCount: 1,
			errSubString: []string{
				`test.extensionRef: Unsupported value: "unsupported"`,
				`supported values: "SnippetsFilter", "AuthenticationFilter"`,
			},
		},
		{
			name: "valid ref",
			ref: &v1.LocalObjectReference{
				Name:  v1.ObjectName("filter"),
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
			},
			expErrCount: 0,
		},
	}

	for _, test := range validTests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			errs := validateExtensionRefFilter(test.ref, testPath)
			g.Expect(errs).To(HaveLen(test.expErrCount))

			if len(test.errSubString) > 0 {
				aggregateErrStr := errs.ToAggregate().Error()
				for _, ss := range test.errSubString {
					g.Expect(aggregateErrStr).To(ContainSubstring(ss))
				}
			}
		})
	}
}

func TestBuildExtRefFilterResolvers(t *testing.T) {
	t.Parallel()

	snippetsFilters := map[types.NamespacedName]*SnippetsFilter{
		{Namespace: "default", Name: "snippets1"}: {},
	}
	authenticationFilters := map[types.NamespacedName]*AuthenticationFilter{
		{Namespace: "default", Name: "auth1"}: {},
	}

	resolvers := buildExtRefFilterResolvers(
		"default",
		snippetsFilters,
		authenticationFilters,
	)

	tests := []struct {
		name string
		ref  v1.LocalObjectReference
	}{
		{
			name: "snippets filter resolver",
			ref: v1.LocalObjectReference{
				Name:  "snippets1",
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
			},
		},
		{
			name: "authentication filter resolver",
			ref: v1.LocalObjectReference{
				Name:  "auth1",
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resolver, exists := resolvers[string(test.ref.Kind)]
			if !exists {
				t.Fatalf("expected resolver for %s kind to exist", test.ref.Kind)
			}

			result := resolver(test.ref)
			if result == nil {
				t.Fatalf("expected non-nil ExtensionRefFilter")
			}

			switch test.ref.Kind {
			case kinds.SnippetsFilter:
				if result.SnippetsFilter == nil {
					t.Fatalf("expected non-nil SnippetsFilter in ExtensionRefFilter")
				}
			case kinds.AuthenticationFilter:
				if result.AuthenticationFilter == nil {
					t.Fatalf("expected non-nil AuthenticationFilter in ExtensionRefFilter")
				}
			}
		})
	}

	invalidTests := []struct {
		name string
		ref  v1.LocalObjectReference
	}{
		{
			name: "invalid kind resolver",
			ref: v1.LocalObjectReference{
				Name:  "invalid1",
				Group: ngfAPI.GroupName,
				Kind:  "InvalidKind",
			},
		},
		{
			name: "unsupported group resolver",
			ref: v1.LocalObjectReference{
				Name:  "invalid2",
				Group: "unsupported.group",
				Kind:  kinds.SnippetsFilter,
			},
		},
		{
			name: "empty kind resolver",
			ref: v1.LocalObjectReference{
				Name:  "invalid3",
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
			},
		},
	}

	for _, test := range invalidTests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			resolver := resolvers[string(test.ref.Kind)]
			if resolver != nil {
				results := resolver(test.ref)
				if results != nil {
					t.Fatalf("expected nil ExtensionRefFilter for invalid kind")
				}
			}
		})
	}
}
