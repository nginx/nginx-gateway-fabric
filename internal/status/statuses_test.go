package status

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/helpers"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/conditions"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/graph"
)

var (
	gw = &v1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       "gateway",
			Generation: 2,
		},
	}

	ignoredGw = &v1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       "ignored-gateway",
			Generation: 1,
		},
	}
)

func TestBuildStatuses(t *testing.T) {
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
			Source: &v1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Spec: v1beta1.HTTPRouteSpec{
					CommonRouteSpec: v1beta1.CommonRouteSpec{
						ParentRefs: []v1beta1.ParentReference{
							{
								SectionName: helpers.GetPointer[v1beta1.SectionName]("listener-80-1"),
							},
							{
								SectionName: helpers.GetPointer[v1beta1.SectionName]("listener-80-2"),
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
			Source: &v1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Spec: v1beta1.HTTPRouteSpec{
					CommonRouteSpec: v1beta1.CommonRouteSpec{
						ParentRefs: []v1beta1.ParentReference{
							{
								SectionName: helpers.GetPointer[v1beta1.SectionName]("listener-80-1"),
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
			Source: &v1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			},
			Valid: true,
		},
		Gateway: &graph.Gateway{
			Source: gw,
			Listeners: map[string]*graph.Listener{
				"listener-80-1": {
					Valid: true,
					Routes: map[types.NamespacedName]*graph.Route{
						{Namespace: "test", Name: "hr-1"}: {},
					},
				},
			},
			Valid: true,
		},
		IgnoredGateways: map[types.NamespacedName]*v1beta1.Gateway{
			client.ObjectKeyFromObject(ignoredGw): ignoredGw,
		},
		Routes: routes,
	}

	expected := Statuses{
		GatewayClassStatuses: GatewayClassStatuses{
			{Name: ""}: {
				ObservedGeneration: 1,
				Conditions:         conditions.NewDefaultGatewayClassConditions(),
			},
		},
		GatewayStatuses: GatewayStatuses{
			{Namespace: "test", Name: "gateway"}: {
				Conditions: conditions.NewDefaultGatewayConditions(),
				ListenerStatuses: map[string]ListenerStatus{
					"listener-80-1": {
						AttachedRoutes: 1,
						Conditions:     conditions.NewDefaultListenerConditions(),
					},
				},
				ObservedGeneration: 2,
			},
			{Namespace: "test", Name: "ignored-gateway"}: {
				Conditions:         conditions.NewGatewayConflict(),
				ObservedGeneration: 1,
			},
		},
		HTTPRouteStatuses: HTTPRouteStatuses{
			{Namespace: "test", Name: "hr-valid"}: {
				ObservedGeneration: 3,
				ParentStatuses: []ParentStatus{
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1beta1.SectionName]("listener-80-1"),
						Conditions:    conditions.NewDefaultRouteConditions(),
					},
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1beta1.SectionName]("listener-80-2"),
						Conditions: append(
							conditions.NewDefaultRouteConditions(),
							invalidAttachmentCondition,
						),
					},
				},
			},
			{Namespace: "test", Name: "hr-invalid"}: {
				ObservedGeneration: 3,
				ParentStatuses: []ParentStatus{
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1beta1.SectionName]("listener-80-1"),
						Conditions: append(
							conditions.NewDefaultRouteConditions(),
							invalidRouteCondition,
						),
					},
				},
			},
		},
	}

	g := NewGomegaWithT(t)

	var nginxReloadRes NginxReloadResult
	result := BuildStatuses(graph, nginxReloadRes)
	g.Expect(helpers.Diff(expected, result)).To(BeEmpty())
}

func TestBuildStatusesNginxErr(t *testing.T) {
	routes := map[types.NamespacedName]*graph.Route{
		{Namespace: "test", Name: "hr-valid"}: {
			Valid: true,
			Source: &v1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Spec: v1beta1.HTTPRouteSpec{
					CommonRouteSpec: v1beta1.CommonRouteSpec{
						ParentRefs: []v1beta1.ParentReference{
							{
								SectionName: helpers.GetPointer[v1beta1.SectionName]("listener-80-1"),
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
			Listeners: map[string]*graph.Listener{
				"listener-80-1": {
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

	expected := Statuses{
		GatewayClassStatuses: GatewayClassStatuses{},
		GatewayStatuses: GatewayStatuses{
			{Namespace: "test", Name: "gateway"}: {
				Conditions: []conditions.Condition{
					conditions.NewGatewayAccepted(),
					conditions.NewGatewayNotProgrammedInvalid(conditions.GatewayMessageFailedNginxReload),
				},
				ListenerStatuses: map[string]ListenerStatus{
					"listener-80-1": {
						AttachedRoutes: 1,
						Conditions: []conditions.Condition{
							conditions.NewListenerAccepted(),
							conditions.NewListenerResolvedRefs(),
							conditions.NewListenerNoConflicts(),
							conditions.NewListenerNotProgrammedInvalid(conditions.ListenerMessageFailedNginxReload),
						},
					},
				},
				ObservedGeneration: 2,
			},
		},
		HTTPRouteStatuses: HTTPRouteStatuses{
			{Namespace: "test", Name: "hr-valid"}: {
				ObservedGeneration: 3,
				ParentStatuses: []ParentStatus{
					{
						GatewayNsName: client.ObjectKeyFromObject(gw),
						SectionName:   helpers.GetPointer[v1beta1.SectionName]("listener-80-1"),
						Conditions: []conditions.Condition{
							conditions.NewRouteResolvedRefs(),
							conditions.NewRouteGatewayNotProgrammed(conditions.RouteMessageFailedNginxReload),
						},
					},
				},
			},
		},
	}

	g := NewGomegaWithT(t)

	nginxReloadRes := NginxReloadResult{Error: errors.New("test error")}
	result := BuildStatuses(graph, nginxReloadRes)
	g.Expect(helpers.Diff(expected, result)).To(BeEmpty())
}

func TestBuildGatewayClassStatuses(t *testing.T) {
	tests := []struct {
		gc             *graph.GatewayClass
		ignoredClasses map[types.NamespacedName]*v1beta1.GatewayClass
		expected       GatewayClassStatuses
		name           string
	}{
		{
			name:     "nil gatewayclass and no ignored gatewayclasses",
			expected: GatewayClassStatuses{},
		},
		{
			name: "nil gatewayclass and ignored gatewayclasses",
			ignoredClasses: map[types.NamespacedName]*v1beta1.GatewayClass{
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
			expected: GatewayClassStatuses{
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
				Source: &v1beta1.GatewayClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "valid-gc",
						Generation: 1,
					},
				},
			},
			expected: GatewayClassStatuses{
				{Name: "valid-gc"}: {
					Conditions:         conditions.NewDefaultGatewayClassConditions(),
					ObservedGeneration: 1,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			result := buildGatewayClassStatuses(test.gc, test.ignoredClasses)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestBuildGatewayStatuses(t *testing.T) {
	tests := []struct {
		nginxReloadRes  NginxReloadResult
		gateway         *graph.Gateway
		ignoredGateways map[types.NamespacedName]*v1beta1.Gateway
		expected        GatewayStatuses
		name            string
	}{
		{
			name:     "nil gateway and no ignored gateways",
			expected: GatewayStatuses{},
		},
		{
			name: "nil gateway and ignored gateways",
			ignoredGateways: map[types.NamespacedName]*v1beta1.Gateway{
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
			expected: GatewayStatuses{
				{Namespace: "test", Name: "ignored-1"}: {
					Conditions:         conditions.NewGatewayConflict(),
					ObservedGeneration: 1,
				},
				{Namespace: "test", Name: "ignored-2"}: {
					Conditions:         conditions.NewGatewayConflict(),
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "valid gateway; all valid listeners",
			gateway: &graph.Gateway{
				Source: gw,
				Listeners: map[string]*graph.Listener{
					"listener-valid-1": {
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
					"listener-valid-2": {
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
				},
				Valid: true,
			},
			expected: GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: conditions.NewDefaultGatewayConditions(),
					ListenerStatuses: map[string]ListenerStatus{
						"listener-valid-1": {
							AttachedRoutes: 1,
							Conditions:     conditions.NewDefaultListenerConditions(),
						},
						"listener-valid-2": {
							AttachedRoutes: 1,
							Conditions:     conditions.NewDefaultListenerConditions(),
						},
					},
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "valid gateway; some valid listeners",
			gateway: &graph.Gateway{
				Source: gw,
				Listeners: map[string]*graph.Listener{
					"listener-valid": {
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
					"listener-invalid": {
						Valid:      false,
						Conditions: conditions.NewListenerUnsupportedValue("unsupported value"),
					},
				},
				Valid: true,
			},
			expected: GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: []conditions.Condition{
						conditions.NewGatewayProgrammed(),
						conditions.NewGatewayAcceptedListenersNotValid(),
					},
					ListenerStatuses: map[string]ListenerStatus{
						"listener-valid": {
							AttachedRoutes: 1,
							Conditions:     conditions.NewDefaultListenerConditions(),
						},
						"listener-invalid": {
							Conditions: conditions.NewListenerUnsupportedValue("unsupported value"),
						},
					},
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "valid gateway; no valid listeners",
			gateway: &graph.Gateway{
				Source: gw,
				Listeners: map[string]*graph.Listener{
					"listener-invalid-1": {
						Valid:      false,
						Conditions: conditions.NewListenerUnsupportedProtocol("unsupported protocol"),
					},
					"listener-invalid-2": {
						Valid:      false,
						Conditions: conditions.NewListenerUnsupportedValue("unsupported value"),
					},
				},
				Valid: true,
			},
			expected: GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: conditions.NewGatewayNotAcceptedListenersNotValid(),
					ListenerStatuses: map[string]ListenerStatus{
						"listener-invalid-1": {
							Conditions: conditions.NewListenerUnsupportedProtocol("unsupported protocol"),
						},
						"listener-invalid-2": {
							Conditions: conditions.NewListenerUnsupportedValue("unsupported value"),
						},
					},
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "invalid gateway",
			gateway: &graph.Gateway{
				Source:     gw,
				Valid:      false,
				Conditions: conditions.NewGatewayInvalid("no gateway class"),
			},
			expected: GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions:         conditions.NewGatewayInvalid("no gateway class"),
					ObservedGeneration: 2,
				},
			},
		},
		{
			name: "error reloading nginx; gateway/listener not programmed",
			gateway: &graph.Gateway{
				Source:     gw,
				Valid:      true,
				Conditions: conditions.NewDefaultGatewayConditions(),
				Listeners: map[string]*graph.Listener{
					"listener-valid": {
						Valid: true,
						Routes: map[types.NamespacedName]*graph.Route{
							{Namespace: "test", Name: "hr-1"}: {},
						},
					},
				},
			},
			expected: GatewayStatuses{
				{Namespace: "test", Name: "gateway"}: {
					Conditions: []conditions.Condition{
						conditions.NewGatewayAccepted(),
						conditions.NewGatewayNotProgrammedInvalid(conditions.GatewayMessageFailedNginxReload),
					},
					ListenerStatuses: map[string]ListenerStatus{
						"listener-valid": {
							AttachedRoutes: 1,
							Conditions: []conditions.Condition{
								conditions.NewListenerAccepted(),
								conditions.NewListenerResolvedRefs(),
								conditions.NewListenerNoConflicts(),
								conditions.NewListenerNotProgrammedInvalid(conditions.ListenerMessageFailedNginxReload),
							},
						},
					},
					ObservedGeneration: 2,
				},
			},
			nginxReloadRes: NginxReloadResult{Error: errors.New("test error")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			result := buildGatewayStatuses(test.gateway, test.ignoredGateways, test.nginxReloadRes)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}
