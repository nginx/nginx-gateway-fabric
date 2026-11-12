package graph

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	apisx "sigs.k8s.io/gateway-api/apisx/v1alpha1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

var backendNsName = types.NamespacedName{Namespace: "test", Name: "xbackend"}

func createBackend(
	nsname types.NamespacedName,
	backendType apisx.BackendType,
	matchLabels map[gatewayv1.LabelKey]gatewayv1.LabelValue,
) *apisx.XBackend {
	backend := &apisx.XBackend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  nsname.Namespace,
			Name:       nsname.Name,
			Generation: 1,
		},
		Spec: apisx.BackendSpec{
			Type: backendType,
		},
	}

	if matchLabels != nil {
		backend.Spec.EndpointSelector = &apisx.EndpointSelectorBackend{
			LabelSelector: apisx.LabelSelector{
				MatchLabels: matchLabels,
			},
		}
	}

	return backend
}

func createBackendResourceRef(
	namespace *gatewayv1.Namespace,
	group *string,
	kind *string,
) RouteBackendRef {
	ref := RouteBackendRef{}
	ref.Name = gatewayv1.ObjectName(backendNsName.Name)
	ref.Namespace = namespace

	if group != nil {
		ref.Group = helpers.GetPointer(gatewayv1.Group(*group))
	}

	if kind != nil {
		ref.Kind = helpers.GetPointer(gatewayv1.Kind(*kind))
	}

	return ref
}

func createBackendHTTPRoute(
	nsname types.NamespacedName,
	gwNsName types.NamespacedName,
	refs ...RouteBackendRef,
) map[RouteKey]*L7Route {
	return map[RouteKey]*L7Route{
		{
			NamespacedName: nsname,
			RouteType:      RouteTypeHTTP,
		}: {
			Source: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsname.Namespace,
					Name:      nsname.Name,
				},
			},
			RouteType: RouteTypeHTTP,
			ParentRefs: []ParentRef{
				{
					Kind:           kinds.Gateway,
					NamespacedName: gwNsName,
					GatewayNsName:  gwNsName,
				},
			},
			Valid: true,
			Spec: L7RouteSpec{
				Rules: []RouteRule{
					{
						RouteBackendRefs: refs,
					},
				},
			},
		},
	}
}

func createBackendTestGateway(nsname types.NamespacedName) map[types.NamespacedName]*Gateway {
	return map[types.NamespacedName]*Gateway{
		nsname: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsname.Namespace,
					Name:      nsname.Name,
				},
			},
		},
	}
}

func TestBuildReferencedBackends(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gw"}
	routeNsName := types.NamespacedName{Namespace: "test", Name: "route"}

	appLabel := map[gatewayv1.LabelKey]gatewayv1.LabelValue{
		"app": "app1",
	}

	backendRef := createBackendResourceRef(
		nil,
		helpers.GetPointer(apisx.GroupName),
		helpers.GetPointer(kinds.Backend),
	)

	otherNamespaceBackendName := types.NamespacedName{Namespace: "other", Name: "xbackend"}

	tests := []struct {
		backends       map[types.NamespacedName]*apisx.XBackend
		expectSelector map[string]string
		name           string
		routeRefs      []RouteBackendRef
		expectNilMap   bool
		expectSource   bool
		expectValid    bool
	}{
		{
			name:      "existing endpoint selector XBackend",
			routeRefs: []RouteBackendRef{backendRef},
			backends: map[types.NamespacedName]*apisx.XBackend{
				backendNsName: createBackend(backendNsName, apisx.BackendTypeEndpointSelector, appLabel),
			},
			expectSource:   true,
			expectValid:    true,
			expectSelector: map[string]string{"app": "app1"},
		},
		{
			name:         "nonexistent XBackend",
			routeRefs:    []RouteBackendRef{backendRef},
			backends:     nil,
			expectSource: false,
		},
		{
			name:      "endpoint selector XBackend without match labels",
			routeRefs: []RouteBackendRef{backendRef},
			backends: map[types.NamespacedName]*apisx.XBackend{
				backendNsName: createBackend(backendNsName, apisx.BackendTypeEndpointSelector, nil),
			},
			expectSource: true,
		},
		{
			name:      "external hostname XBackend",
			routeRefs: []RouteBackendRef{backendRef},
			backends: map[types.NamespacedName]*apisx.XBackend{
				backendNsName: createBackend(backendNsName, apisx.BackendTypeExternalHostname, appLabel),
			},
			expectSource: true,
		},
		{
			name:      "reference without group is not an XBackend",
			routeRefs: []RouteBackendRef{createBackendResourceRef(nil, nil, helpers.GetPointer(kinds.Backend))},
			backends: map[types.NamespacedName]*apisx.XBackend{
				backendNsName: createBackend(backendNsName, apisx.BackendTypeEndpointSelector, appLabel),
			},
			expectNilMap: true,
		},
		{
			name:      "reference without kind is not an XBackend",
			routeRefs: []RouteBackendRef{createBackendResourceRef(nil, helpers.GetPointer(apisx.GroupName), nil)},
			backends: map[types.NamespacedName]*apisx.XBackend{
				backendNsName: createBackend(backendNsName, apisx.BackendTypeEndpointSelector, appLabel),
			},
			expectNilMap: true,
		},
		{
			name: "reference to an XBackend in another namespace",
			routeRefs: []RouteBackendRef{
				createBackendResourceRef(
					helpers.GetPointer(gatewayv1.Namespace("other")),
					helpers.GetPointer(apisx.GroupName),
					helpers.GetPointer(kinds.Backend),
				),
			},
			backends: map[types.NamespacedName]*apisx.XBackend{
				otherNamespaceBackendName: createBackend(
					otherNamespaceBackendName,
					apisx.BackendTypeEndpointSelector,
					appLabel,
				),
			},
			expectSource:   true,
			expectValid:    true,
			expectSelector: map[string]string{"app": "app1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			routes := createBackendHTTPRoute(routeNsName, gwNsName, test.routeRefs...)
			gws := createBackendTestGateway(gwNsName)

			referenced := buildReferencedBackends(routes, gws, test.backends, nil, controllerName, logr.Discard())

			if test.expectNilMap {
				g.Expect(referenced).To(BeNil())
				return
			}

			nsname := backendNsName
			if ref := test.routeRefs[0]; ref.Namespace != nil {
				nsname.Namespace = string(*ref.Namespace)
			}

			g.Expect(referenced).To(HaveKey(nsname))

			refBackend := referenced[nsname]

			if test.expectSource {
				g.Expect(refBackend.Source).ToNot(BeNil())
				g.Expect(refBackend.Gateways).To(ConsistOf(gwNsName))
				g.Expect(refBackend.Routes).To(HaveLen(1))
			} else {
				g.Expect(refBackend.Source).To(BeNil())
				g.Expect(refBackend.Gateways).To(BeEmpty())
				g.Expect(refBackend.Routes).To(BeEmpty())
			}

			g.Expect(refBackend.Valid).To(Equal(test.expectValid))
			g.Expect(refBackend.PodSelector).To(Equal(test.expectSelector))

			if !test.expectSource {
				g.Expect(refBackend.Conditions).To(BeNil())
			} else {
				g.Expect(refBackend.Conditions).To(HaveLen(1))

				if test.expectValid {
					g.Expect(refBackend.Conditions[0].Type).To(Equal(string(apisx.BackendConditionAccepted)))
					g.Expect(refBackend.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
					g.Expect(refBackend.Conditions[0].Reason).To(Equal(string(apisx.BackendReasonAccepted)))
				} else {
					g.Expect(refBackend.Conditions[0].Type).To(Equal(string(apisx.BackendConditionAccepted)))
					g.Expect(refBackend.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					g.Expect(refBackend.Conditions[0].Reason).To(
						Equal(string(conditions.BackendReasonUnsupportedValue)),
					)
					g.Expect(refBackend.Conditions[0].Message).ToNot(BeEmpty())
				}
			}
		})
	}
}

func TestBuildReferencedBackendsWithoutGatewaysAndRoutes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gw"}
	routeNsName := types.NamespacedName{Namespace: "test", Name: "route"}

	routes := createBackendHTTPRoute(
		routeNsName,
		gwNsName,
		createBackendResourceRef(nil, helpers.GetPointer(apisx.GroupName), helpers.GetPointer(kinds.Backend)),
	)

	g.Expect(buildReferencedBackends(routes, nil, nil, nil, controllerName, logr.Discard())).To(BeNil())
	g.Expect(
		buildReferencedBackends(nil, createBackendTestGateway(gwNsName), nil, nil, controllerName, logr.Discard()),
	).To(BeNil())
}

func TestBuildReferencedBackendsRouteNotBelongingToGateway(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	routeNsName := types.NamespacedName{Namespace: "test", Name: "route"}

	routes := createBackendHTTPRoute(
		routeNsName,
		types.NamespacedName{Namespace: "test", Name: "another-gw"},
		createBackendResourceRef(nil, helpers.GetPointer(apisx.GroupName), helpers.GetPointer(kinds.Backend)),
	)

	referenced := buildReferencedBackends(
		routes,
		createBackendTestGateway(types.NamespacedName{Namespace: "test", Name: "gw"}),
		map[types.NamespacedName]*apisx.XBackend{
			backendNsName: createBackend(
				backendNsName,
				apisx.BackendTypeEndpointSelector,
				map[gatewayv1.LabelKey]gatewayv1.LabelValue{"app": "app1"},
			),
		},
		nil,
		controllerName,
		logr.Discard(),
	)

	g.Expect(referenced).To(BeNil())
}

func TestUniqueGatewayAncestors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	baseTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gw"}

	createGateway := func(name string, offset time.Duration) *Gateway {
		return &Gateway{
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         backendNsName.Namespace,
					Name:              name,
					CreationTimestamp: metav1.NewTime(baseTime.Add(offset)),
				},
			},
		}
	}

	gws := map[types.NamespacedName]*Gateway{
		gwNsName: createGateway(gwNsName.Name, time.Minute),
	}
	newGwNsName := types.NamespacedName{Namespace: "test", Name: "new-gw"}
	gws[newGwNsName] = createGateway(newGwNsName.Name, 2*time.Minute)

	ancestors := uniqueGatewayAncestors(
		[]types.NamespacedName{newGwNsName, gwNsName, newGwNsName, gwNsName},
		gws,
		backendNsName,
		0,
		logr.Discard(),
	)
	g.Expect(ancestors).To(Equal([]types.NamespacedName{gwNsName, newGwNsName}))

	unknownGwNsName := types.NamespacedName{Namespace: "test", Name: "unknown-gw"}
	ancestors = uniqueGatewayAncestors(
		[]types.NamespacedName{newGwNsName, unknownGwNsName},
		gws,
		backendNsName,
		0,
		logr.Discard(),
	)
	g.Expect(ancestors).To(HaveLen(2))

	manyGws := make(map[types.NamespacedName]*Gateway, MaxBackendAncestors+3)
	manyAncestors := make([]types.NamespacedName, 0, MaxBackendAncestors+3)

	for i := range MaxBackendAncestors + 3 {
		nsname := types.NamespacedName{Namespace: "test", Name: fmt.Sprintf("gw-%02d", i)}
		manyGws[nsname] = createGateway(nsname.Name, time.Duration(i)*time.Minute)
		manyAncestors = append(manyAncestors, nsname)
	}

	slices.Reverse(manyAncestors)

	ancestors = uniqueGatewayAncestors(manyAncestors, manyGws, backendNsName, 0, logr.Discard())
	g.Expect(ancestors).To(HaveLen(MaxBackendAncestors))
	g.Expect(ancestors[0]).To(Equal(types.NamespacedName{Namespace: "test", Name: "gw-00"}))
	g.Expect(ancestors[MaxBackendAncestors-1]).To(Equal(
		types.NamespacedName{Namespace: "test", Name: fmt.Sprintf("gw-%02d", MaxBackendAncestors-1)},
	))
	g.Expect(ancestors).ToNot(ContainElement(
		types.NamespacedName{Namespace: "test", Name: fmt.Sprintf("gw-%02d", MaxBackendAncestors+2)},
	))

	ancestors = uniqueGatewayAncestors(manyAncestors, manyGws, backendNsName, 2, logr.Discard())
	g.Expect(ancestors).To(HaveLen(MaxBackendAncestors - 2))
	g.Expect(ancestors).ToNot(ContainElement(
		types.NamespacedName{Namespace: "test", Name: fmt.Sprintf("gw-%02d", MaxBackendAncestors-2)},
	))

	someAncestors := manyAncestors[:30]
	ancestors = uniqueGatewayAncestors(someAncestors, manyGws, backendNsName, 5, logr.Discard())
	g.Expect(ancestors).To(HaveLen(27))
	g.Expect(ancestors[0]).To(Equal(types.NamespacedName{Namespace: "test", Name: "gw-05"}))
	g.Expect(ancestors).ToNot(ContainElement(
		types.NamespacedName{Namespace: "test", Name: fmt.Sprintf("gw-%02d", MaxBackendAncestors+2)},
	))

	for _, otherControllers := range []int{MaxBackendAncestors, MaxBackendAncestors + 5} {
		ancestors = uniqueGatewayAncestors(manyAncestors, manyGws, backendNsName, otherControllers, logr.Discard())
		g.Expect(ancestors).To(BeEmpty())
	}
}

func TestOtherControllerAncestors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	createAncestor := func(ctlrName string) apisx.BackendAncestorStatus {
		return apisx.BackendAncestorStatus{
			AncestorRef: gatewayv1.ParentReference{
				Namespace: helpers.GetPointer(gatewayv1.Namespace(backendNsName.Namespace)),
				Name:      gatewayv1.ObjectName(backendNsName.Name),
			},
			ControllerName: gatewayv1.GatewayController(ctlrName),
		}
	}

	g.Expect(otherControllerAncestors(&ReferencedBackend{}, controllerName)).To(BeZero())

	refBackend := &ReferencedBackend{
		Source: &apisx.XBackend{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backendNsName.Namespace,
				Name:      backendNsName.Name,
			},
		},
	}
	g.Expect(otherControllerAncestors(refBackend, controllerName)).To(BeZero())

	refBackend.Source.Status.Ancestors = []apisx.BackendAncestorStatus{
		createAncestor(controllerName),
		createAncestor("other.controller"),
		createAncestor("another.controller"),
	}

	g.Expect(otherControllerAncestors(refBackend, controllerName)).To(Equal(2))
}

func TestReferencedBackendMatchesPod(t *testing.T) {
	t.Parallel()

	backend := createBackend(
		backendNsName,
		apisx.BackendTypeEndpointSelector,
		map[gatewayv1.LabelKey]gatewayv1.LabelValue{
			"app":     "app1",
			"version": "v1",
		},
	)

	createPod := func(namespace string, labels map[string]string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "pod",
				Labels:    labels,
			},
		}
	}

	tests := []struct {
		source   *apisx.XBackend
		selector map[string]string
		pod      *v1.Pod
		name     string
		expect   bool
	}{
		{
			name:     "pod matches",
			source:   backend,
			selector: map[string]string{"app": "app1"},
			pod:      createPod("test", map[string]string{"app": "app1"}),
			expect:   true,
		},
		{
			name:     "pod with extra labels matches",
			source:   backend,
			selector: map[string]string{"app": "app1", "version": "v1"},
			pod:      createPod("test", map[string]string{"app": "app1", "version": "v1", "extra": "label"}),
			expect:   true,
		},
		{
			name:     "pod with some of the labels does not match",
			source:   backend,
			selector: map[string]string{"app": "app1", "version": "v1"},
			pod:      createPod("test", map[string]string{"app": "app1"}),
			expect:   false,
		},
		{
			name:     "pod with other labels does not match",
			source:   backend,
			selector: map[string]string{"app": "app2"},
			pod:      createPod("test", map[string]string{"app": "app1"}),
			expect:   false,
		},
		{
			name:     "pod in another namespace does not match",
			source:   backend,
			selector: map[string]string{"app": "app1"},
			pod:      createPod("other", map[string]string{"app": "app1"}),
			expect:   false,
		},
		{
			name:     "pod without labels does not match",
			source:   backend,
			selector: map[string]string{"app": "app1"},
			pod:      createPod("test", nil),
			expect:   false,
		},
		{
			name:     "empty selector matches nothing",
			source:   backend,
			selector: nil,
			pod:      createPod("test", map[string]string{"app": "app1"}),
			expect:   false,
		},
		{
			name:     "nonexistent XBackend matches nothing",
			source:   nil,
			selector: map[string]string{"app": "app1"},
			pod:      createPod("test", map[string]string{"app": "app1"}),
			expect:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			refBackend := &ReferencedBackend{
				Source:      test.source,
				PodSelector: test.selector,
			}

			g.Expect(refBackend.matchesPod(test.pod)).To(Equal(test.expect))
		})
	}
}

func TestGraphPodIsReferenced(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	backend := createBackend(
		backendNsName,
		apisx.BackendTypeEndpointSelector,
		map[gatewayv1.LabelKey]gatewayv1.LabelValue{"app": "app1"},
	)

	graph := &Graph{
		ReferencedBackends: map[types.NamespacedName]*ReferencedBackend{
			backendNsName: {
				Source:      backend,
				PodSelector: map[string]string{"app": "app1"},
				Valid:       true,
			},
		},
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "pod",
			Labels:    map[string]string{"app": "app1"},
		},
	}

	g.Expect(graph.IsReferenced(pod, types.NamespacedName{Namespace: "test", Name: "pod"})).To(
		BeTrue(),
		"pod selected by a referenced XBackend should be referenced",
	)

	unselectedPod := pod.DeepCopy()
	unselectedPod.Labels = map[string]string{"app": "app2"}

	g.Expect(graph.IsReferenced(unselectedPod, types.NamespacedName{Namespace: "test", Name: "pod"})).To(
		BeFalse(),
		"pod that the XBackend does not select should not be referenced",
	)

	g.Expect(graph.IsReferenced(backend, backendNsName)).To(
		BeTrue(),
		"XBackend that a route references should be referenced",
	)

	g.Expect(
		graph.IsReferenced(
			&apisx.XBackend{},
			types.NamespacedName{Namespace: "test", Name: "another-xbackend"},
		),
	).To(BeFalse(), "XBackend that no route references should not be referenced")

	ownership := resolver.NewPodOwnership()
	ownership.Replace(backendNsName, []v1.Pod{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "pod"},
	}})
	graph.PodOwnership = ownership

	g.Expect(graph.IsReferenced(&v1.Pod{}, types.NamespacedName{Namespace: "test", Name: "pod"})).To(
		BeTrue(),
		"deleted pod that a referenced XBackend was routing to should be referenced",
	)

	g.Expect(graph.IsReferenced(&v1.Pod{}, types.NamespacedName{Namespace: "other", Name: "pod"})).To(
		BeFalse(),
		"deleted pod that no referenced XBackend was routing to should not be referenced",
	)

	graph.PodOwnership = nil

	g.Expect(graph.IsReferenced(&v1.Pod{}, types.NamespacedName{Namespace: "test", Name: "pod"})).To(
		BeFalse(),
		"deleted pod with no ownership records should not be referenced",
	)
}

func TestValidateRouteBackendRefForBackend(t *testing.T) {
	t.Parallel()

	alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }
	backendGroupAndKind := helpers.GetPointer(apisx.GroupName)
	backendKind := helpers.GetPointer(kinds.Backend)

	tests := []struct {
		expectedCondition conditions.Condition
		name              string
		routeType         RouteType
		ref               RouteBackendRef
		expectedValid     bool
	}{
		{
			name:          "http route accepts an XBackend reference without a port",
			routeType:     RouteTypeHTTP,
			ref:           createBackendResourceRef(nil, backendGroupAndKind, backendKind),
			expectedValid: true,
		},
		{
			name:          "grpc route accepts an XBackend reference without a port",
			routeType:     RouteTypeGRPC,
			ref:           createBackendResourceRef(nil, backendGroupAndKind, backendKind),
			expectedValid: true,
		},
		{
			name:      "group is rejected when kind is not XBackend",
			routeType: RouteTypeHTTP,
			ref:       createBackendResourceRef(nil, backendGroupAndKind, helpers.GetPointer(kinds.Service)),
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.kind: Invalid value: "Service": kind must be XBackend when group is gateway.networking.x-k8s.io`,
			),
		},
		{
			name:      "kind is rejected when group is not the x-k8s API group",
			routeType: RouteTypeHTTP,
			ref:       createBackendResourceRef(nil, helpers.GetPointer("core"), backendKind),
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.group: Invalid value: "core": group must be gateway.networking.x-k8s.io when kind is XBackend`,
			),
		},
		{
			name:      "cross-namespace reference is not permitted",
			routeType: RouteTypeHTTP,
			ref: createBackendResourceRef(
				helpers.GetPointer(gatewayv1.Namespace("other")),
				backendGroupAndKind,
				backendKind,
			),
			expectedCondition: conditions.NewRouteBackendRefRefNotPermitted(
				`test.namespace: Forbidden: Cross-namespace backend ref to XBackend other/xbackend is not supported`,
			),
		},
		{
			name:      "cross-namespace reference is not permitted on a grpc route either",
			routeType: RouteTypeGRPC,
			ref: createBackendResourceRef(
				helpers.GetPointer(gatewayv1.Namespace("other")),
				backendGroupAndKind,
				backendKind,
			),
			expectedCondition: conditions.NewRouteBackendRefRefNotPermitted(
				`test.namespace: Forbidden: Cross-namespace backend ref to XBackend other/xbackend is not supported`,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			valid, cond := validateRouteBackendRef(
				test.routeType,
				test.ref,
				backendNsName.Namespace,
				alwaysTrueRefGrantResolver,
				field.NewPath("test"),
				FeatureFlags{Experimental: true},
			)

			g.Expect(valid).To(Equal(test.expectedValid))
			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestValidateRouteBackendRefForBackendWithoutExperimentalFeatures(t *testing.T) {
	t.Parallel()

	alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }
	backendGroupAndKind := helpers.GetPointer(apisx.GroupName)
	backendKind := helpers.GetPointer(kinds.Backend)

	tests := []struct {
		name      string
		routeType RouteType
	}{
		{
			name:      "http route rejects an XBackend reference",
			routeType: RouteTypeHTTP,
		},
		{
			name:      "grpc route rejects an XBackend reference",
			routeType: RouteTypeGRPC,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			valid, cond := validateRouteBackendRef(
				test.routeType,
				createBackendResourceRef(nil, backendGroupAndKind, backendKind),
				backendNsName.Namespace,
				alwaysTrueRefGrantResolver,
				field.NewPath("test"),
				FeatureFlags{},
			)

			g.Expect(valid).To(BeFalse())
			g.Expect(cond.Type).To(Equal(string(gatewayv1.RouteConditionResolvedRefs)))
			g.Expect(cond.Reason).To(Equal(string(gatewayv1.RouteReasonInvalidKind)))
		})
	}
}

func TestAddBackendRefsToRulesWithBackends(t *testing.T) {
	t.Parallel()

	appLabel := map[gatewayv1.LabelKey]gatewayv1.LabelValue{
		"app": "app1",
	}

	backend := createBackend(backendNsName, apisx.BackendTypeEndpointSelector, appLabel)
	backend.Spec.Port = apisx.BackendPort{Number: 8080}

	refGrantResolver := newReferenceGrantResolver(nil)

	backendGroupAndKind := helpers.GetPointer(apisx.GroupName)
	backendKind := helpers.GetPointer(kinds.Backend)

	validBackend := &ReferencedBackend{
		Source:      backend,
		PodSelector: map[string]string{"app": "app1"},
		Valid:       true,
	}

	backendRef := createBackendResourceRef(nil, backendGroupAndKind, backendKind)
	portConflictRef := createBackendResourceRef(nil, backendGroupAndKind, backendKind)
	portConflictRef.Port = helpers.GetPointer(gatewayv1.PortNumber(9000))

	tests := []struct {
		referencedBackends map[types.NamespacedName]*ReferencedBackend
		name               string
		expectedPortRef    string
		expectedConditions []conditions.Condition
		ref                RouteBackendRef
		expectedBackend    BackendRef
	}{
		{
			name:               "resolved XBackend carries the selector and the port from the XBackend",
			ref:                backendRef,
			referencedBackends: map[types.NamespacedName]*ReferencedBackend{backendNsName: validBackend},
			expectedBackend: BackendRef{
				BackendNsName: backendNsName,
				PodSelector:   map[string]string{"app": "app1"},
				BackendPort:   8080,
				Valid:         true,
				Weight:        1,
				IsBackend:     true,
			},
			expectedPortRef: "test_xbackend_8080_backend",
		},
		{
			name: "a deleted XBackend is reported as not found",
			ref:  backendRef,
			referencedBackends: map[types.NamespacedName]*ReferencedBackend{
				backendNsName: {Source: nil, Valid: false},
			},
			expectedBackend: BackendRef{
				BackendNsName:      backendNsName,
				Valid:              false,
				Weight:             1,
				IsBackend:          true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefRefBackendNotFound(
					`spec.rules[0].backendRefs[0].name: Not found: "Backend ref to XBackend test/xbackend not found"`,
				),
			},
		},
		{
			name:               "an XBackend reference with no entry in the Graph is reported as not found",
			ref:                backendRef,
			referencedBackends: map[types.NamespacedName]*ReferencedBackend{},
			expectedBackend: BackendRef{
				BackendNsName:      backendNsName,
				Valid:              false,
				Weight:             1,
				IsBackend:          true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefRefBackendNotFound(
					`spec.rules[0].backendRefs[0].name: Not found: "Backend ref to XBackend test/xbackend not found"`,
				),
			},
		},
		{
			name: "an XBackend of an unsupported type is reported as an unsupported value",
			ref:  backendRef,
			referencedBackends: map[types.NamespacedName]*ReferencedBackend{
				backendNsName: {
					Source: createBackend(backendNsName, apisx.BackendTypeExternalHostname, nil),
					Valid:  false,
				},
			},
			expectedBackend: BackendRef{
				BackendNsName:      backendNsName,
				Valid:              false,
				Weight:             1,
				IsBackend:          true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					"XBackend test/xbackend is invalid and cannot be used as a backendRef",
				),
			},
		},
		{
			name:               "a port on the backendRef does not override the XBackend port",
			ref:                portConflictRef,
			referencedBackends: map[types.NamespacedName]*ReferencedBackend{backendNsName: validBackend},
			expectedBackend: BackendRef{
				BackendNsName: backendNsName,
				PodSelector:   map[string]string{"app": "app1"},
				BackendPort:   8080,
				Valid:         true,
				Weight:        1,
				IsBackend:     true,
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					`spec.rules[0].backendRefs[0].port: Forbidden: Backend ref to XBackend test/xbackend port 9000` +
						" ignored, the XBackend selects port 8080",
				),
			},
			expectedPortRef: "test_xbackend_8080_backend",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			routes := createBackendHTTPRoute(
				types.NamespacedName{Namespace: "test", Name: "route"},
				types.NamespacedName{Namespace: "test", Name: "gw"},
				test.ref,
			)
			route := routes[RouteKey{
				NamespacedName: types.NamespacedName{Namespace: "test", Name: "route"},
				RouteType:      RouteTypeHTTP,
			}]
			route.Spec.Rules[0].ValidMatches = true
			route.Spec.Rules[0].Filters.Valid = true

			addBackendRefsToRules(route, refGrantResolver, map[types.NamespacedName]*v1.Service{}, nil,
				test.referencedBackends, nil, FeatureFlags{Experimental: true})

			g.Expect(route.Spec.Rules[0].BackendRefs).To(HaveLen(1))
			g.Expect(helpers.Diff(test.expectedBackend, route.Spec.Rules[0].BackendRefs[0])).To(BeEmpty())
			g.Expect(route.Conditions).To(Equal(test.expectedConditions))
			g.Expect(route.Spec.Rules[0].BackendRefs[0].ServicePortReference()).To(Equal(test.expectedPortRef))
		})
	}
}

func TestAddBackendRefsToRulesWithBackendsWithoutExperimentalFeatures(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	backendGroupAndKind := helpers.GetPointer(apisx.GroupName)
	backendKind := helpers.GetPointer(kinds.Backend)

	routes := createBackendHTTPRoute(
		types.NamespacedName{Namespace: "test", Name: "route"},
		types.NamespacedName{Namespace: "test", Name: "gw"},
		createBackendResourceRef(nil, backendGroupAndKind, backendKind),
	)
	route := routes[RouteKey{
		NamespacedName: types.NamespacedName{Namespace: "test", Name: "route"},
		RouteType:      RouteTypeHTTP,
	}]
	route.Spec.Rules[0].ValidMatches = true
	route.Spec.Rules[0].Filters.Valid = true

	addBackendRefsToRules(route, newReferenceGrantResolver(nil), map[types.NamespacedName]*v1.Service{}, nil,
		map[types.NamespacedName]*ReferencedBackend{}, nil, FeatureFlags{})

	backend := route.Spec.Rules[0].BackendRefs[0]
	g.Expect(backend.Valid).To(BeFalse())
	g.Expect(backend.IsBackend).To(BeTrue())
	g.Expect(backend.ServicePortReference()).To(BeEmpty())
	g.Expect(route.Conditions).To(HaveLen(1))
	g.Expect(route.Conditions[0].Type).To(Equal(string(gatewayv1.RouteConditionResolvedRefs)))
	g.Expect(route.Conditions[0].Reason).To(Equal(string(gatewayv1.RouteReasonInvalidKind)))
}

func TestBuildL4RoutesRejectBackendRefs(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}
	gws := map[types.NamespacedName]*Gateway{
		gwNsName: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "gateway"},
			},
			Valid: true,
		},
	}

	parentRefs := []gatewayv1.ParentReference{{
		Namespace:   helpers.GetPointer(gatewayv1.Namespace(gwNsName.Namespace)),
		Name:        gatewayv1.ObjectName(gwNsName.Name),
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
		Kind:        helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
	}}

	refGrantResolver := func(_ toResource) bool { return true }

	backendRef := gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Name:  gatewayv1.ObjectName(backendNsName.Name),
			Group: helpers.GetPointer[gatewayv1.Group](apisx.GroupName),
			Kind:  helpers.GetPointer[gatewayv1.Kind](kinds.Backend),
		},
	}

	services := map[types.NamespacedName]*v1.Service{
		backendNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: backendNsName.Namespace, Name: backendNsName.Name},
			Spec:       v1.ServiceSpec{Ports: []v1.ServicePort{{Port: 80}}},
		},
	}

	expectedBackendRef := BackendRef{
		Valid:              false,
		InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
	}
	expectedCond := conditions.NewRouteBackendRefInvalidKind(
		`spec.rules[0].backendRefs[0].group: Unsupported value: "gateway.networking.x-k8s.io": supported values: "core", ""`,
	)

	assertRejected := func(g *WithT, route *L4Route) {
		g.Expect(route).ToNot(BeNil())
		g.Expect(route.Spec.BackendRefs).To(HaveLen(1))
		g.Expect(route.Spec.BackendRefs[0]).To(Equal(expectedBackendRef))
		g.Expect(route.Conditions).To(ContainElement(expectedCond))
	}

	t.Run("TLSRoute", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		tr := &gatewayv1.TLSRoute{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "tlsroute"},
			Spec: gatewayv1.TLSRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: parentRefs},
				Rules: []gatewayv1.TLSRouteRule{
					{BackendRefs: []gatewayv1.BackendRef{backendRef}},
				},
			},
		}

		assertRejected(g, buildTLSRoute(tr, gws, services, nil, refGrantResolver, nil))
	})

	t.Run("TCPRoute", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		tr := &gatewayv1.TCPRoute{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "tcproute"},
			Spec: gatewayv1.TCPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: parentRefs},
				Rules: []gatewayv1.TCPRouteRule{
					{BackendRefs: []gatewayv1.BackendRef{backendRef}},
				},
			},
		}

		assertRejected(g, buildTCPRoute(tr, gws, services, refGrantResolver, nil))
	})

	t.Run("UDPRoute", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ur := &gatewayv1.UDPRoute{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "udproute"},
			Spec: gatewayv1.UDPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: parentRefs},
				Rules: []gatewayv1.UDPRouteRule{
					{BackendRefs: []gatewayv1.BackendRef{backendRef}},
				},
			},
		}

		assertRejected(g, buildUDPRoute(ur, gws, services, refGrantResolver, nil))
	})
}
