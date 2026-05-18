package v1_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	wafv1 "github.com/nginx/nginx-gateway-fabric/v2/apis/waf/v1"
)

// parseBundleStatusFunc is the common signature for ParseAPPolicyStatus and ParseAPLogConfStatus,
// projected to only the Bundle field we care about.
type parseBundleStatusFunc func(obj *unstructured.Unstructured) (*wafv1.BundleStatus, error)

func wrapParseAPPolicyStatus(obj *unstructured.Unstructured) (*wafv1.BundleStatus, error) {
	s, err := wafv1.ParseAPPolicyStatus(obj)
	if err != nil {
		return nil, err
	}
	return s.Bundle, nil
}

func wrapParseAPLogConfStatus(obj *unstructured.Unstructured) (*wafv1.BundleStatus, error) {
	s, err := wafv1.ParseAPLogConfStatus(obj)
	if err != nil {
		return nil, err
	}
	return s.Bundle, nil
}

// makeAPObject builds a minimal unstructured object with optional status.
func makeAPObject(kind string, status map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "appprotect.f5.com/v1",
		"kind":       kind,
		"metadata": map[string]any{
			"name":      "test-resource",
			"namespace": "default",
		},
	}
	if status != nil {
		obj["status"] = status
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestParseAPPolicyStatus(t *testing.T) {
	t.Parallel()
	runParseBundleStatusTests(t, "APPolicy", wrapParseAPPolicyStatus)
}

func TestParseAPLogConfStatus(t *testing.T) {
	t.Parallel()
	runParseBundleStatusTests(t, "APLogConf", wrapParseAPLogConfStatus)
}

// runParseBundleStatusTests runs a shared set of table-driven tests for any
// PLM status parser that extracts a BundleStatus.
func runParseBundleStatusTests(t *testing.T, kind string, parse parseBundleStatusFunc) {
	t.Helper()

	tests := []struct {
		obj       *unstructured.Unstructured
		expBundle *wafv1.BundleStatus
		name      string
		expErrMsg string
		expectErr bool
	}{
		{
			name: "valid ready status with all fields",
			obj: makeAPObject(kind, map[string]any{
				"bundle": map[string]any{
					"state":              "ready",
					"location":           "s3://bundles/test.tgz",
					"sha256":             "abc123",
					"compilerVersion":    "1.0.0",
					"observedGeneration": int64(3),
				},
			}),
			expBundle: &wafv1.BundleStatus{
				State:              "ready",
				Location:           "s3://bundles/test.tgz",
				SHA256:             "abc123",
				CompilerVersion:    "1.0.0",
				ObservedGeneration: 3,
			},
		},
		{
			name: "pending status with no location",
			obj: makeAPObject(kind, map[string]any{
				"bundle": map[string]any{
					"state": "pending",
				},
			}),
			expBundle: &wafv1.BundleStatus{
				State: "pending",
			},
		},
		{
			name:      "status with no bundle field returns nil bundle",
			obj:       makeAPObject(kind, map[string]any{}),
			expBundle: nil,
		},
		{
			name:      "no status field returns error",
			obj:       makeAPObject(kind, nil),
			expectErr: true,
			expErrMsg: "has no status",
		},
		{
			name: "wrong kind returns error",
			obj: makeAPObject(otherKind(kind), map[string]any{
				"bundle": map[string]any{"state": "ready"},
			}),
			expectErr: true,
			expErrMsg: "expected appprotect.f5.com/v1",
		},
		{
			name: "wrong apiVersion returns error",
			obj: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "appprotect.f5.com/v1alpha1",
				"kind":       kind,
				"metadata": map[string]any{
					"name":      "test-resource",
					"namespace": "default",
				},
				"status": map[string]any{
					"bundle": map[string]any{"state": "ready"},
				},
			}},
			expectErr: true,
			expErrMsg: "expected appprotect.f5.com/v1",
		},
		{
			name: "unknown fields are ignored",
			obj: makeAPObject(kind, map[string]any{
				"bundle": map[string]any{
					"state":        "ready",
					"unknownField": "ignored",
				},
				"anotherField": "also-ignored",
			}),
			expBundle: &wafv1.BundleStatus{
				State: "ready",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			bundle, err := parse(tt.obj)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expErrMsg))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(bundle).To(Equal(tt.expBundle))
		})
	}
}

func otherKind(kind string) string {
	if kind == "APPolicy" {
		return "APLogConf"
	}

	return "APPolicy"
}
