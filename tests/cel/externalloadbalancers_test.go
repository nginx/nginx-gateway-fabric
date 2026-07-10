package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func gatewayTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return []gatewayv1.LocalPolicyTargetReference{
		{
			Kind:  gatewayKind,
			Group: gatewayGroup,
			Name:  gatewayv1.ObjectName(uniqueResourceName(testTargetRefName)),
		},
	}
}

func TestExternalLoadBalancersTargetRefs(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	validGatewayLink := &ngfAPIv1alpha1.GatewayLinkConfig{
		VirtualServerAddress: helpers.GetPointer("10.8.3.101"),
	}

	tests := []struct {
		gatewayLink *ngfAPIv1alpha1.GatewayLinkConfig
		name        string
		targetRefs  []gatewayv1.LocalPolicyTargetReference
		wantErrors  []string
	}{
		{
			name:        "a single targetRef of kind Gateway in group gateway.networking.k8s.io is allowed",
			gatewayLink: validGatewayLink,
			targetRefs:  gatewayTargetRefs(),
		},
		{
			name:        "a targetRef of kind Service is rejected because only Gateway is supported",
			gatewayLink: validGatewayLink,
			wantErrors:  []string{expectedELBTargetRefKindGatewayError},
			targetRefs: []gatewayv1.LocalPolicyTargetReference{
				{
					Kind:  serviceKind,
					Group: gatewayGroup,
					Name:  gatewayv1.ObjectName(uniqueResourceName(testTargetRefName)),
				},
			},
		},
		{
			name: "a targetRef in group invalid.networking.k8s.io is rejected " +
				"because only gateway.networking.k8s.io is supported",
			gatewayLink: validGatewayLink,
			wantErrors:  []string{expectedTargetRefGroupError},
			targetRefs: []gatewayv1.LocalPolicyTargetReference{
				{
					Kind:  gatewayKind,
					Group: invalidGroup,
					Name:  gatewayv1.ObjectName(uniqueResourceName(testTargetRefName)),
				},
			},
		},
		{
			name:        "an empty targetRefs list is rejected because at least one Gateway is required",
			gatewayLink: validGatewayLink,
			wantErrors:  []string{expectedELBTargetRefsMinItemsError},
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{},
		},
		{
			name:        "two targetRefs are rejected because at most one Gateway is supported",
			gatewayLink: validGatewayLink,
			wantErrors:  []string{expectedELBTargetRefsMaxItemsError},
			targetRefs: []gatewayv1.LocalPolicyTargetReference{
				{
					Kind:  gatewayKind,
					Group: gatewayGroup,
					Name:  gatewayv1.ObjectName(uniqueResourceName(testTargetRefName)),
				},
				{
					Kind:  gatewayKind,
					Group: gatewayGroup,
					Name:  gatewayv1.ObjectName(uniqueResourceName(testTargetRefName)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			elb := &ngfAPIv1alpha1.ExternalLoadBalancers{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: ngfAPIv1alpha1.ExternalLoadBalancersSpec{
					TargetRefs:  tt.targetRefs,
					GatewayLink: tt.gatewayLink,
				},
			}
			validateCrd(t, tt.wantErrors, elb, k8sClient)
		})
	}
}

func TestExternalLoadBalancersBackend(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		gatewayLink *ngfAPIv1alpha1.GatewayLinkConfig
		name        string
		wantErrors  []string
	}{
		{
			name:        "gatewayLink unset is rejected because exactly one backend must be set",
			gatewayLink: nil,
			wantErrors:  []string{expectedELBBackendRequiredError},
		},
		{
			name:        "gatewayLink with virtualServerAddress alone is allowed",
			gatewayLink: &ngfAPIv1alpha1.GatewayLinkConfig{VirtualServerAddress: helpers.GetPointer("10.8.3.101")},
		},
		{
			name:        "gatewayLink with ipamLabel alone is allowed",
			gatewayLink: &ngfAPIv1alpha1.GatewayLinkConfig{IPAMLabel: helpers.GetPointer("bigip-pool")},
		},
		{
			name: "gatewayLink with virtualServerAddress and ipamLabel " +
				"together is rejected because they are mutually exclusive",
			gatewayLink: &ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.8.3.101"),
				IPAMLabel:            helpers.GetPointer("bigip-pool"),
			},
			wantErrors: []string{expectedELBVirtualServerAddressIPAMLabelExclusiveError},
		},
		{
			name: "gatewayLink with neither virtualServerAddress nor ipamLabel " +
				"is rejected because one of them must be set",
			gatewayLink: &ngfAPIv1alpha1.GatewayLinkConfig{Partition: helpers.GetPointer("k8s")},
			wantErrors:  []string{expectedELBVirtualServerAddressOrIPAMLabelRequiredError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			elb := &ngfAPIv1alpha1.ExternalLoadBalancers{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: ngfAPIv1alpha1.ExternalLoadBalancersSpec{
					TargetRefs:  gatewayTargetRefs(),
					GatewayLink: tt.gatewayLink,
				},
			}
			validateCrd(t, tt.wantErrors, elb, k8sClient)
		})
	}
}

func TestExternalLoadBalancersPartition(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		partition  string
		wantErrors []string
	}{
		{
			name:      "a non-Common partition is allowed",
			partition: "k8s",
		},
		{
			name:       "the Common partition is rejected because it is reserved",
			partition:  "Common",
			wantErrors: []string{expectedELBPartitionCommonError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			elb := &ngfAPIv1alpha1.ExternalLoadBalancers{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: ngfAPIv1alpha1.ExternalLoadBalancersSpec{
					TargetRefs: gatewayTargetRefs(),
					GatewayLink: &ngfAPIv1alpha1.GatewayLinkConfig{
						VirtualServerAddress: helpers.GetPointer("10.8.3.101"),
						Partition:            helpers.GetPointer(tt.partition),
					},
				},
			}
			validateCrd(t, tt.wantErrors, elb, k8sClient)
		})
	}
}
