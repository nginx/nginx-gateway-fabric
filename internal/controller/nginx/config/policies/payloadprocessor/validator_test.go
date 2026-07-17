package payloadprocessor_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/payloadprocessor"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func createValidPolicy() *ngfAPI.PayloadProcessor {
	return &ngfAPI.PayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.PayloadProcessorSpec{
			TargetRef: v1.LocalPolicyTargetReference{
				Group: v1.GroupName,
				Kind:  kinds.Gateway,
				Name:  "gateway",
			},
			Processors: []ngfAPI.PayloadProcessorEntry{
				{
					ExtProcess: &ngfAPI.ExtProcessConfig{
						BackendRef: v1.BackendObjectReference{
							Name: "ext-proc-service",
							Port: helpers.GetPointer[v1.PortNumber](9000),
						},
					},
				},
			},
		},
	}
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		policy        *ngfAPI.PayloadProcessor
		name          string
		expConditions []conditions.Condition
	}{
		{
			name:          "valid policy",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "invalid target ref kind",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.TargetRef.Kind = "Unsupported"
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"Gateway\", \"HTTPRoute\""),
			},
		},
		{
			name: "no processors",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors = nil
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("at least one processor must be specified"),
			},
		},
		{
			name: "missing extProcess configuration",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors[0].ExtProcess = nil
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("processor missing extProcess configuration"),
			},
		},
		{
			name: "missing extProcess backendRef name",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors[0].ExtProcess.BackendRef.Name = ""
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("processor extProcess.backendRef.name must be set"),
			},
		},
		{
			name: "invalid extProcess backendRef group",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors[0].ExtProcess.BackendRef.Group = helpers.GetPointer[v1.Group]("invalid.group")
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("processor extProcess.backendRef.group must be core"),
			},
		},
		{
			name: "invalid extProcess backendRef kind",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors[0].ExtProcess.BackendRef.Kind = helpers.GetPointer[v1.Kind]("Secret")
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("processor extProcess.backendRef.kind must be Service"),
			},
		},
		{
			name: "missing extProcess backendRef port",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors[0].ExtProcess.BackendRef.Port = nil
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("processor extProcess.backendRef.port must be set"),
			},
		},
		{
			name: "invalid extProcess backendRef port",
			policy: func() *ngfAPI.PayloadProcessor {
				p := createValidPolicy()
				p.Spec.Processors[0].ExtProcess.BackendRef.Port = helpers.GetPointer[v1.PortNumber](0)
				return p
			}(),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("processor extProcess.backendRef.port must be a valid TCP port"),
			},
		},
	}

	validator := payloadprocessor.NewValidator()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			conds := validator.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	validator := payloadprocessor.NewValidator()
	pol := createValidPolicy()

	// PayloadProcessor has no global settings gating.
	g.Expect(validator.ValidateGlobalSettings(pol, nil)).To(BeNil())
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	validator := payloadprocessor.NewValidator()
	pol1 := createValidPolicy()
	pol2 := createValidPolicy()

	// PayloadProcessors occupy a single phase, so any two targeting the same object conflict.
	// The graph layer keeps the oldest and marks newer ones Conflicted.
	g.Expect(validator.Conflicts(pol1, pol2)).To(BeTrue())
}
