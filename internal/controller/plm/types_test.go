package plm

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestExtractAPResourceStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		obj      *unstructured.Unstructured
		expected *APResourceStatus
		name     string
	}{
		{
			name: "full status with all fields (APPolicy example)",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"bundle": map[string]any{
							"state":    "ready",
							"location": "s3://default/bundles/policy.tgz",
							"sha256":   "abc123",
						},
						"processing": map[string]any{
							"datetime":   "2026-02-04T21:13:54Z",
							"isCompiled": true,
						},
						"observedGeneration": int64(2),
					},
				},
			},
			expected: &APResourceStatus{
				Bundle: BundleStatus{
					State:    StateReady,
					Location: "s3://default/bundles/policy.tgz",
					Sha256:   "abc123",
				},
				Processing: ProcessingStatus{
					Datetime:   "2026-02-04T21:13:54Z",
					IsCompiled: true,
				},
				ObservedGeneration: 2,
			},
		},
		{
			name: "full status (APLogConf example)",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"bundle": map[string]any{
							"state":    "ready",
							"location": "s3://default/bundles/logconf.tgz",
							"sha256":   "def456",
						},
						"processing": map[string]any{
							"datetime":   "2026-02-04T22:00:00Z",
							"isCompiled": true,
						},
						"observedGeneration": int64(3),
					},
				},
			},
			expected: &APResourceStatus{
				Bundle: BundleStatus{
					State:    StateReady,
					Location: "s3://default/bundles/logconf.tgz",
					Sha256:   "def456",
				},
				Processing: ProcessingStatus{
					Datetime:   "2026-02-04T22:00:00Z",
					IsCompiled: true,
				},
				ObservedGeneration: 3,
			},
		},
		{
			name: "no status field",
			obj: &unstructured.Unstructured{
				Object: map[string]any{},
			},
			expected: &APResourceStatus{},
		},
		{
			name: "empty status",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{},
				},
			},
			expected: &APResourceStatus{},
		},
		{
			name: "status with only bundle",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"bundle": map[string]any{
							"state": "pending",
						},
					},
				},
			},
			expected: &APResourceStatus{
				Bundle: BundleStatus{
					State: StatePending,
				},
			},
		},
		{
			name: "status with processing errors",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"bundle": map[string]any{
							"state": "invalid",
						},
						"processing": map[string]any{
							"datetime":   "2026-02-04T21:13:54Z",
							"errors":     []any{"syntax error in policy", "invalid directive"},
							"isCompiled": false,
						},
					},
				},
			},
			expected: &APResourceStatus{
				Bundle: BundleStatus{
					State: StateInvalid,
				},
				Processing: ProcessingStatus{
					Datetime:   "2026-02-04T21:13:54Z",
					Errors:     []string{"syntax error in policy", "invalid directive"},
					IsCompiled: false,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result, err := ExtractAPResourceStatus(tc.obj)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestNewAPPolicyUnstructured(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	obj := NewAPPolicyUnstructured()
	g.Expect(obj).ToNot(BeNil())
	g.Expect(obj.GroupVersionKind()).To(Equal(kinds.APPolicyGVK))
}

func TestNewAPLogConfUnstructured(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	obj := NewAPLogConfUnstructured()
	g.Expect(obj).ToNot(BeNil())
	g.Expect(obj.GroupVersionKind()).To(Equal(kinds.APLogConfGVK))
}

func TestNewAPPolicyListUnstructured(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	list := NewAPPolicyListUnstructured()
	g.Expect(list).ToNot(BeNil())
	g.Expect(list.GroupVersionKind().Group).To(Equal(kinds.PLMGroup))
	g.Expect(list.GroupVersionKind().Version).To(Equal(kinds.PLMVersion))
	g.Expect(list.GetKind()).To(Equal(kinds.APPolicy + "List"))
}

func TestNewAPLogConfListUnstructured(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	list := NewAPLogConfListUnstructured()
	g.Expect(list).ToNot(BeNil())
	g.Expect(list.GroupVersionKind().Group).To(Equal(kinds.PLMGroup))
	g.Expect(list.GroupVersionKind().Version).To(Equal(kinds.PLMVersion))
	g.Expect(list.GetKind()).To(Equal(kinds.APLogConf + "List"))
}
