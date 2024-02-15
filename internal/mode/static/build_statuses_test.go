package static

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/status"
	staticConds "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/graph"
)

var (
	gw = &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       "gateway",
			Generation: 2,
		},
	}

	ignoredGw = &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       "ignored-gateway",
			Generation: 1,
		},
	}
)

func TestBuildStatuses(t *testing.T) {
	addr := []v1.GatewayStatusAddress{
		{
			Type:  helpers.GetPointer(v1.IPAddressType),
			Value: "1.2.3.4",
		},
	}

	invalidRouteCondition := conditions.Condition{
		Type:   "TestInvalidRoute",
		Status: metav1.ConditionTrue,
	}
	invalidAttachmentCondition := conditions.Condition{
		Type:   "TestInvalidAttachment",
		Status: metav1.ConditionTrue,
	}

	routes := map[types.NamespacedName]*graph.Route{
		{Namespace: "test", Name: "hr-valid"}: {
			Valid: true,
			Source: &v1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Spec: v1.HTTPRouteSpec{
					CommonRouteSpec: v1.CommonRouteSpec{
						ParentRefs: []v1.ParentReference{
							{
								SectionName: helpers.GetPointer[v1.SectionName]("listener-80-1"),
							},
							{
								SectionName: helpers.GetPointer[v1.SectionName]("listener-80-2"),
							},
						},
					},
				},
			},
			ParentRefs: []graph.ParentRef{
				{
					Idx:     0,
					Gateway: client.ObjectKeyFromObject(gw),
					Attachment: &graph.ParentRefAttachmentStatus{
						Attached: true,
					},
				},
				{
					Idx:     1,
					Gateway: client.ObjectKeyFromObject(gw),
					Attachment: &graph.ParentRefAttachmentStatus{
						Attached:        false,
						FailedCondition: invalidAttachmentCondition,
					},
				},
			},
		},
		{Namespace: "test", Name: "hr-invalid"}: {
			Valid:      false,
			Conditions: []conditions.Condition{invalidRouteCondition},
			Source: &v1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Spec: v1.HTTPRouteSpec{
					CommonRouteSpec: v1.CommonRouteSpec{
						ParentRefs: []v1.ParentReference{
							{
								SectionName: helpers.GetPointer[v1.SectionName]("listener-80-1"),
							},
						},
					},
				},
			},
			ParentRefs: []graph.ParentRef{
				{
					Idx:        0,
					Gateway:    client.ObjectKeyFromObject(gw),
					Attachment: nil,
				},
			},
		},
	}

	graph := &graph.Graph{
		GatewayClass: &graph.GatewayClass{
			Source: &v1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			},
			Valid: true,
		},
		Gateway: &graph.Gateway{
			Source: gw,
			Listeners: []*graph.Listener{
				{
					Name:  "listener-80-1",
					Valid: true,
					Routes: map[types.NamespacedName]*graph.Route{
						{Namespace: "test", Name: "hr-1"}: {},
					},
				},
			},
			Valid: true,
		},
		IgnoredGateways: map[types.NamespacedName]*v1.Gateway{
			client.ObjectKeyFromObject(ignoredGw): ignoredGw,
		},
		Routes: routes,
	}

	expected := status.GatewayAPIStatuses{
		GatewayClassStatuses: status.GatewayClassStatuses{
			{Name: ""}: {
				ObservedGeneration: 1,
				Conditions:         conditions.NewDefaultGatewayClassConditions(),
			},
		},
		GatewayStatuses: status.GatewayStatuses{
			{Namespace: "test", Name: "gateway"}: {
				Conditions: staticConds.NewDefaultGatewayConditions(),
				ListenerStatuses: []status.ListenerStatus{
					{
						Name:           "listener-80-1",
						AttachedRoutes: 1,
						Conditions:     staticConds.NewDefaultListenerConditions(),
					},
				},
				Addresses:          addr,
				ObservedGeneration: 2,
			},
			{Namespace: "test", Name: "ignored-gateway"}: {
				Conditions:         staticConds.NewGatewayConflict(),
				ObservedGeneration: 1,
				Ignored:            true,
			},
		},
		HTTPRouteStatuses: status.HTTPRouteStatuses{
			{Namespace: "test", Name: "hr-valid"}: {
				ObservedGeneration: 3,
				ParentStatuses: []status.ParentStatus{
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1.SectionName]("listener-80-1"),
						Conditions:    staticConds.NewDefaultRouteConditions(),
					},
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1.SectionName]("listener-80-2"),
						Conditions: append(
							staticConds.NewDefaultRouteConditions(),
							invalidAttachmentCondition,
						),
					},
				},
			},
			{Namespace: "test", Name: "hr-invalid"}: {
				ObservedGeneration: 3,
				ParentStatuses: []status.ParentStatus{
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1.SectionName]("listener-80-1"),
						Conditions: append(
							staticConds.NewDefaultRouteConditions(),
							invalidRouteCondition,
						),
					},
				},
			},
		},
		BackendTLSPolicyStatuses: status.BackendTLSPolicyStatuses{},
	}

	g := NewWithT(t)

	var nginxReloadRes nginxReloadResult
	result := buildGatewayAPIStatuses(graph, addr, nginxReloadRes)
	g.Expect(helpers.Diff(expected, result)).To(BeEmpty())
}

func TestBuildStatusesNginxErr(t *testing.T) {
	addr := []v1.GatewayStatusAddress{
		{
			Type:  helpers.GetPointer(v1.IPAddressType),
			Value: "1.2.3.4",
		},
	}

	routes := map[types.NamespacedName]*graph.Route{
		{Namespace: "test", Name: "hr-valid"}: {
			Valid: true,
			Source: &v1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Spec: v1.HTTPRouteSpec{
					CommonRouteSpec: v1.CommonRouteSpec{
						ParentRefs: []v1.ParentReference{
							{
								SectionName: helpers.GetPointer[v1.SectionName]("listener-80-1"),
							},
						},
					},
				},
			},
			ParentRefs: []graph.ParentRef{
				{
					Idx:     0,
					Gateway: client.ObjectKeyFromObject(gw),
					Attachment: &graph.ParentRefAttachmentStatus{
						Attached: true,
					},
				},
			},
		},
	}

	graph := &graph.Graph{
		Gateway: &graph.Gateway{
			Source: gw,
			Listeners: []*graph.Listener{
				{
					Name:  "listener-80-1",
					Valid: true,
					Routes: map[types.NamespacedName]*graph.Route{
						{Namespace: "test", Name: "hr-1"}: {},
					},
				},
			},
			Valid: true,
		},
		Routes: routes,
	}

	expected := status.GatewayAPIStatuses{
		GatewayClassStatuses: status.GatewayClassStatuses{},
		GatewayStatuses: status.GatewayStatuses{
			{Namespace: "test", Name: "gateway"}: {
				Conditions: []conditions.Condition{
					staticConds.NewGatewayAccepted(),
					staticConds.NewGatewayNotProgrammedInvalid(staticConds.GatewayMessageFailedNginxReload),
				},
				ListenerStatuses: []status.ListenerStatus{
					{
						Name:           "listener-80-1",
						AttachedRoutes: 1,
						Conditions: []conditions.Condition{
							staticConds.NewListenerAccepted(),
							staticConds.NewListenerResolvedRefs(),
							staticConds.NewListenerNoConflicts(),
							staticConds.NewListenerNotProgrammedInvalid(staticConds.ListenerMessageFailedNginxReload),
						},
					},
				},
				Addresses:          addr,
				ObservedGeneration: 2,
			},
		},
		HTTPRouteStatuses: status.HTTPRouteStatuses{
			{Namespace: "test", Name: "hr-valid"}: {
				ObservedGeneration: 3,
				ParentStatuses: []status.ParentStatus{
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1.SectionName]("listener-80-1"),
						Conditions: []conditions.Condition{
							staticConds.NewRouteResolvedRefs(),
							staticConds.NewRouteGatewayNotProgrammed(staticConds.RouteMessageFailedNginxReload),
						},
					},
				},
			},
		},
		BackendTLSPolicyStatuses: status.BackendTLSPolicyStatuses{},
	}

	g := NewWithT(t)

	nginxReloadRes := nginxReloadResult{error: errors.New("test error")}
	result := buildGatewayAPIStatuses(graph, addr, nginxReloadRes)
	g.Expect(helpers.Diff(expected, result)).To(BeEmpty())
}

func TestBuildGatewayClassStatuses(t *testing.T) {
	tests := []struct {
		gc             *graph.GatewayClass
		ignoredClasses map[types.NamespacedName]*v1.GatewayClass
		expected       status.GatewayClassStatuses
		name           string
	}{
		{
			name:     "nil gatewayclass and no ignored gatewayclasses",
			expected: status.GatewayClassStatuses{},
		},
		{
			name: "nil gatewayclass and ignored gatewayclasses",
			ignoredClasses: map[types.NamespacedName]*v1.GatewayClass{
				{Name: "ignored-1"}: {
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
				{Name: "ignored-2"}: {
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2,
					},
				},
			},
			expected: status.GatewayClassStatuses{
				{Name: "ignored-1"}: {
					Conditions:         []conditions.Condition{conditions.NewGatewayClassConflict()},
					ObservedGeneration: 1,
				},
				{Name: "ignored-2"}: {
					Conditions:         []conditions.Condition{conditions.NewGatewayClassConflict()},
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "valid gatewayclass",
			gc: &graph.GatewayClass{
				Source: &v1.GatewayClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "valid-gc",
						Generation: 1,
					},
				},
			},
			expected: status.GatewayClassStatuses{
				{Name: "valid-gc"}: {
					Conditions:         conditions.NewDefaultGatewayClassConditions(),
					ObservedGeneration: 1,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			result := buildGatewayClassStatuses(test.gc, test.ignoredClasses)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestBuildGatewayStatuses(t *testing.T) {
	addr := []v1.GatewayStatusAddress{
		{
			Type:  helpers.GetPointer(v1.IPAddressType),
			Value: "1.2.3.4",
		},
	}

	tests := []struct {
		nginxReloadRes  nginxReloadResult
		gateway         *graph.Gateway
		ignoredGateways map[types.NamespacedName]*v1.Gateway
		expected        status.GatewayStatuses
		name            string
	}{
		{
			name:     "nil gateway and no ignored gateways",
			expected: status.GatewayStatuses{},
		},
		{
			name: "nil gateway and ignored gateways",
			ignoredGateways: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "ignored-1"}: {
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
				{Namespace: "test", Name: "ignored-2"}: {
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2,
					},
				},
			},
			expected: status.GatewayStatuses{
				{Namespace: "test", Name: "ignored-1"}: {
					Conditions:         staticConds.NewGatewayConflict(),
					ObservedGeneration: 1,
					Ignored:            true,
				},
				{Namespace: "test", Name: "ignored-2"}: {
					Conditions:         staticConds.NewGatewayConflict(),
					ObservedGeneration: 2,
					Ignored:            true,
				},
			},
		},
		{
			name: "valid gateway; all valid listeners",
			gateway: &graph.Gateway{
				Source: gw,
				Listeners: []*graph.Listener{
					{
						Name:  "listener-valid-1",
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
					{
						Name:  "listener-valid-2",
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
				},
				Valid: true,
			},
			expected: status.GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: staticConds.NewDefaultGatewayConditions(),
					ListenerStatuses: []status.ListenerStatus{
						{
							Name:           "listener-valid-1",
							AttachedRoutes: 1,
							Conditions:     staticConds.NewDefaultListenerConditions(),
						},
						{
							Name:           "listener-valid-2",
							AttachedRoutes: 1,
							Conditions:     staticConds.NewDefaultListenerConditions(),
						},
					},
					Addresses:          addr,
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "valid gateway; some valid listeners",
			gateway: &graph.Gateway{
				Source: gw,
				Listeners: []*graph.Listener{
					{
						Name:  "listener-valid",
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
					{
						Name:       "listener-invalid",
						Valid:      false,
						Conditions: staticConds.NewListenerUnsupportedValue("unsupported value"),
					},
				},
				Valid: true,
			},
			expected: status.GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: []conditions.Condition{
						staticConds.NewGatewayProgrammed(),
						staticConds.NewGatewayAcceptedListenersNotValid(),
					},
					ListenerStatuses: []status.ListenerStatus{
						{
							Name:           "listener-valid",
							AttachedRoutes: 1,
							Conditions:     staticConds.NewDefaultListenerConditions(),
						},
						{
							Name:       "listener-invalid",
							Conditions: staticConds.NewListenerUnsupportedValue("unsupported value"),
						},
					},
					Addresses:          addr,
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "valid gateway; no valid listeners",
			gateway: &graph.Gateway{
				Source: gw,
				Listeners: []*graph.Listener{
					{
						Name:       "listener-invalid-1",
						Valid:      false,
						Conditions: staticConds.NewListenerUnsupportedProtocol("unsupported protocol"),
					},
					{
						Name:       "listener-invalid-2",
						Valid:      false,
						Conditions: staticConds.NewListenerUnsupportedValue("unsupported value"),
					},
				},
				Valid: true,
			},
			expected: status.GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: staticConds.NewGatewayNotAcceptedListenersNotValid(),
					ListenerStatuses: []status.ListenerStatus{
						{
							Name:       "listener-invalid-1",
							Conditions: staticConds.NewListenerUnsupportedProtocol("unsupported protocol"),
						},
						{
							Name:       "listener-invalid-2",
							Conditions: staticConds.NewListenerUnsupportedValue("unsupported value"),
						},
					},
					Addresses:          addr,
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "invalid gateway",
			gateway: &graph.Gateway{
				Source:     gw,
				Valid:      false,
				Conditions: staticConds.NewGatewayInvalid("no gateway class"),
			},
			expected: status.GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions:         staticConds.NewGatewayInvalid("no gateway class"),
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "error reloading nginx; gateway/listener not programmed",
			gateway: &graph.Gateway{
				Source:     gw,
				Valid:      true,
				Conditions: staticConds.NewDefaultGatewayConditions(),
				Listeners: []*graph.Listener{
					{
						Name:  "listener-valid",
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
				},
			},
			expected: status.GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: []conditions.Condition{
						staticConds.NewGatewayAccepted(),
						staticConds.NewGatewayNotProgrammedInvalid(staticConds.GatewayMessageFailedNginxReload),
					},
					ListenerStatuses: []status.ListenerStatus{
						{
							Name:           "listener-valid",
							AttachedRoutes: 1,
							Conditions: []conditions.Condition{
								staticConds.NewListenerAccepted(),
								staticConds.NewListenerResolvedRefs(),
								staticConds.NewListenerNoConflicts(),
								staticConds.NewListenerNotProgrammedInvalid(
									staticConds.ListenerMessageFailedNginxReload,
								),
							},
						},
					},
					Addresses:          addr,
					ObservedGeneration: 2,
				},
			},
			nginxReloadRes: nginxReloadResult{error: errors.New("test error")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			result := buildGatewayStatuses(test.gateway, test.ignoredGateways, addr, test.nginxReloadRes)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestBuildBackendTLSPolicyStatuses(t *testing.T) {
	type policyCfg struct {
		Name         string
		Conditions   []conditions.Condition
		Valid        bool
		Ignored      bool
		IsReferenced bool
	}

	getBackendTLSPolicy := func(policyCfg policyCfg) *graph.BackendTLSPolicy {
		return &graph.BackendTLSPolicy{
			Source: &v1alpha2.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "test",
					Name:       policyCfg.Name,
					Generation: 1,
				},
			},
			Valid:        policyCfg.Valid,
			Ignored:      policyCfg.Ignored,
			IsReferenced: policyCfg.IsReferenced,
			Conditions:   policyCfg.Conditions,
			Gateway:      types.NamespacedName{Name: "gateway", Namespace: "test"},
		}
	}

	attachedConds := []conditions.Condition{staticConds.NewBackendTLSPolicyAccepted()}
	invalidConds := []conditions.Condition{staticConds.NewBackendTLSPolicyInvalid("invalid backendTLSPolicy")}

	validPolicyCfg := policyCfg{
		Name:         "valid-bt",
		Valid:        true,
		IsReferenced: true,
		Conditions:   attachedConds,
	}

	invalidPolicyCfg := policyCfg{
		Name:         "invalid-bt",
		IsReferenced: true,
		Conditions:   invalidConds,
	}

	ignoredPolicyCfg := policyCfg{
		Name:         "ignored-bt",
		Ignored:      true,
		IsReferenced: true,
	}

	notReferencedPolicyCfg := policyCfg{
		Name:  "not-referenced",
		Valid: true,
	}

	tests := []struct {
		backendTLSPolicies map[types.NamespacedName]*graph.BackendTLSPolicy
		expected           status.BackendTLSPolicyStatuses
		name               string
	}{
		{
			name:     "nil backendTLSPolicies",
			expected: status.BackendTLSPolicyStatuses{},
		},
		{
			name: "valid backendTLSPolicy",
			backendTLSPolicies: map[types.NamespacedName]*graph.BackendTLSPolicy{
				{Namespace: "test", Name: "valid-bt"}: getBackendTLSPolicy(validPolicyCfg),
			},
			expected: status.BackendTLSPolicyStatuses{
				{Namespace: "test", Name: "valid-bt"}: {
					AncestorStatuses: []status.AncestorStatus{
						{
							Conditions:    attachedConds,
							GatewayNsName: types.NamespacedName{Name: "gateway", Namespace: "test"},
						},
					},
				},
			},
		},
		{
			name: "invalid backendTLSPolicy",
			backendTLSPolicies: map[types.NamespacedName]*graph.BackendTLSPolicy{
				{Namespace: "test", Name: "invalid-bt"}: getBackendTLSPolicy(invalidPolicyCfg),
			},
			expected: status.BackendTLSPolicyStatuses{
				{Namespace: "test", Name: "invalid-bt"}: {
					AncestorStatuses: []status.AncestorStatus{
						{
							Conditions:    invalidConds,
							GatewayNsName: types.NamespacedName{Name: "gateway", Namespace: "test"},
						},
					},
				},
			},
		},
		{
			name: "ignored or not referenced backendTLSPolicies",
			backendTLSPolicies: map[types.NamespacedName]*graph.BackendTLSPolicy{
				{Namespace: "test", Name: "ignored-bt"}:     getBackendTLSPolicy(ignoredPolicyCfg),
				{Namespace: "test", Name: "not-referenced"}: getBackendTLSPolicy(notReferencedPolicyCfg),
			},
			expected: status.BackendTLSPolicyStatuses{},
		},
		{
			name: "mix valid and ignored backendTLSPolicies",
			backendTLSPolicies: map[types.NamespacedName]*graph.BackendTLSPolicy{
				{Namespace: "test", Name: "ignored-bt"}: getBackendTLSPolicy(ignoredPolicyCfg),
				{Namespace: "test", Name: "valid-bt"}:   getBackendTLSPolicy(validPolicyCfg),
			},
			expected: status.BackendTLSPolicyStatuses{
				{Namespace: "test", Name: "valid-bt"}: {
					AncestorStatuses: []status.AncestorStatus{
						{
							Conditions:    attachedConds,
							GatewayNsName: types.NamespacedName{Name: "gateway", Namespace: "test"},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			result := buildBackendTLSPolicyStatuses(test.backendTLSPolicies)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}
