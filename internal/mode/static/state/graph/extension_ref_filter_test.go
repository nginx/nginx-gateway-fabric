package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func TestValidateExtensionRefFilter(t *testing.T) {
	t.Parallel()
	testPath := field.NewPath("test")

	tests := []struct {
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
				`test.extensionRef: Required value: extensionRef cannot be nil`,
			},
		},
		{
			name:        "empty ref",
			ref:         &v1.LocalObjectReference{},
			expErrCount: 3,
			errSubString: []string{
				`test.extensionRef: Required value: name cannot be empty`,
				`test.extensionRef: Unsupported value: "": supported values: "gateway.nginx.org"`,
				`test.extensionRef: Unsupported value: "": supported values: "SnippetsFilter"`,
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
				`test.extensionRef: Unsupported value: "unsupported": supported values: "gateway.nginx.org"`,
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
				`test.extensionRef: Unsupported value: "unsupported": supported values: "SnippetsFilter"`,
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

	for _, test := range tests {
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
