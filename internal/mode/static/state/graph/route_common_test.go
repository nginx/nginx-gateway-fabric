package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

func TestBuildSectionNameRefs(t *testing.T) {
	t.Parallel()
	const routeNamespace = "test"

	gwNsName1 := types.NamespacedName{Namespace: routeNamespace, Name: "gateway-1"}
	gwNsName2 := types.NamespacedName{Namespace: routeNamespace, Name: "gateway-2"}

	parentRefs := []gatewayv1.ParentReference{
		{
			Name:        gatewayv1.ObjectName(gwNsName1.Name),
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("one"),
		},
		{
			Name:        gatewayv1.ObjectName("some-other-gateway"),
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("two"),
		},
		{
			Name:        gatewayv1.ObjectName(gwNsName2.Name),
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("three"),
		},
		{
			Name:        gatewayv1.ObjectName(gwNsName1.Name),
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("same-name"),
		},
		{
			Name:        gatewayv1.ObjectName(gwNsName2.Name),
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("same-name"),
		},
		{
			Name:        gatewayv1.ObjectName("some-other-gateway"),
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("same-name"),
		},
	}

	gws := map[types.NamespacedName]*Gateway{
		gwNsName1: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwNsName1.Name,
					Namespace: gwNsName1.Namespace,
				},
			},
		},
		gwNsName2: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwNsName2.Name,
					Namespace: gwNsName2.Namespace,
				},
			},
		},
	}

	expected := []ParentRef{
		{
			Idx:         0,
			Gateway:     CreateParentRefGateway(gws[gwNsName1]),
			SectionName: parentRefs[0].SectionName,
		},
		{
			Idx:         2,
			Gateway:     CreateParentRefGateway(gws[gwNsName2]),
			SectionName: parentRefs[2].SectionName,
		},
		{
			Idx:         3,
			Gateway:     CreateParentRefGateway(gws[gwNsName1]),
			SectionName: parentRefs[3].SectionName,
		},
		{
			Idx:         4,
			Gateway:     CreateParentRefGateway(gws[gwNsName2]),
			SectionName: parentRefs[4].SectionName,
		},
	}

	tests := []struct {
		expectedError error
		name          string
		parentRefs    []gatewayv1.ParentReference
		expectedRefs  []ParentRef
	}{
		{
			name:          "normal case",
			parentRefs:    parentRefs,
			expectedRefs:  expected,
			expectedError: nil,
		},
		{
			parentRefs: []gatewayv1.ParentReference{
				{
					Name:        gatewayv1.ObjectName(gwNsName1.Name),
					SectionName: helpers.GetPointer[gatewayv1.SectionName]("http"),
				},
				{
					Name:        gatewayv1.ObjectName(gwNsName1.Name),
					SectionName: helpers.GetPointer[gatewayv1.SectionName]("http"),
				},
			},
			name:          "duplicate sectionNames",
			expectedError: errors.New("duplicate section name \"http\" for Gateway test/gateway-1"),
		},
		{
			parentRefs: []gatewayv1.ParentReference{
				{
					Name:        gatewayv1.ObjectName(gwNsName1.Name),
					SectionName: nil,
				},
				{
					Name:        gatewayv1.ObjectName(gwNsName1.Name),
					SectionName: nil,
				},
			},
			name:          "nil sectionNames",
			expectedError: errors.New("duplicate section name \"\" for Gateway test/gateway-1"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result, err := buildSectionNameRefs(test.parentRefs, routeNamespace, gws)
			g.Expect(result).To(Equal(test.expectedRefs))
			if test.expectedError != nil {
				g.Expect(err).To(Equal(test.expectedError))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestFindGatewayForParentRef(t *testing.T) {
	t.Parallel()
	gwNsName1 := types.NamespacedName{Namespace: "test-1", Name: "gateway-1"}
	gwNsName2 := types.NamespacedName{Namespace: "test-2", Name: "gateway-2"}

	tests := []struct {
		ref              gatewayv1.ParentReference
		expectedGwNsName types.NamespacedName
		name             string
		expectedFound    bool
	}{
		{
			ref: gatewayv1.ParentReference{
				Namespace: helpers.GetPointer(gatewayv1.Namespace(gwNsName1.Namespace)),
				Name:      gatewayv1.ObjectName(gwNsName1.Name),
			},
			expectedFound:    true,
			expectedGwNsName: gwNsName1,
			name:             "found",
		},
		{
			ref: gatewayv1.ParentReference{
				Group:     helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
				Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
				Namespace: helpers.GetPointer(gatewayv1.Namespace(gwNsName1.Namespace)),
				Name:      gatewayv1.ObjectName(gwNsName1.Name),
			},
			expectedFound:    true,
			expectedGwNsName: gwNsName1,
			name:             "found with explicit group and kind",
		},
		{
			ref: gatewayv1.ParentReference{
				Name: gatewayv1.ObjectName(gwNsName2.Name),
			},
			expectedFound:    true,
			expectedGwNsName: gwNsName2,
			name:             "found with implicit namespace",
		},
		{
			ref: gatewayv1.ParentReference{
				Kind: helpers.GetPointer[gatewayv1.Kind]("NotGateway"),
				Name: gatewayv1.ObjectName(gwNsName2.Name),
			},
			expectedFound: false,
			name:          "wrong kind",
		},
		{
			ref: gatewayv1.ParentReference{
				Group: helpers.GetPointer[gatewayv1.Group]("wrong-group"),
				Name:  gatewayv1.ObjectName(gwNsName2.Name),
			},
			expectedFound: false,
			name:          "wrong group",
		},
		{
			ref: gatewayv1.ParentReference{
				Namespace: helpers.GetPointer(gatewayv1.Namespace(gwNsName1.Namespace)),
				Name:      "some-gateway",
			},
			expectedFound: false,
			name:          "not found",
		},
	}

	routeNamespace := "test-2"

	gws := map[types.NamespacedName]*Gateway{
		gwNsName1: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwNsName1.Name,
					Namespace: gwNsName1.Namespace,
				},
			},
		},
		gwNsName2: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwNsName2.Name,
					Namespace: gwNsName2.Namespace,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gw := findGatewayForParentRef(test.ref, routeNamespace, gws)
			if test.expectedFound {
				g.Expect(gw).ToNot(BeNil())
				g.Expect(client.ObjectKeyFromObject(gw.Source)).To(Equal(test.expectedGwNsName))
			} else {
				g.Expect(gw).To(BeNil())
			}
		})
	}
}

func TestBindRouteToListeners(t *testing.T) {
	// we create a new listener each time because the function under test can modify it
	createListener := func(name string) *Listener {
		return &Listener{
			Name: name,
			GatewayName: types.NamespacedName{
				Namespace: "test",
				Name:      "gateway",
			},
			Source: gatewayv1.Listener{
				Name:     gatewayv1.SectionName(name),
				Hostname: (*gatewayv1.Hostname)(helpers.GetPointer("foo.example.com")),
				Protocol: gatewayv1.HTTPProtocolType,
			},
			Valid:      true,
			Attachable: true,
			Routes:     map[RouteKey]*L7Route{},
			SupportedKinds: []gatewayv1.RouteGroupKind{
				{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
				{Kind: gatewayv1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
			},
		}
	}
	createModifiedListener := func(name string, m func(*Listener)) *Listener {
		l := createListener(name)
		m(l)
		return l
	}

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway",
		},
	}
	gwDiffNamespace := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "diff-namespace",
			Name:      "gateway",
		},
	}

	createHTTPRouteWithSectionNameAndPort := func(
		sectionName *gatewayv1.SectionName,
		port *gatewayv1.PortNumber,
	) *gatewayv1.HTTPRoute {
		return &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "hr",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "HTTPRoute",
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{
							Name:        gatewayv1.ObjectName(gw.Name),
							SectionName: sectionName,
							Port:        port,
						},
					},
				},
				Hostnames: []gatewayv1.Hostname{
					"foo.example.com",
				},
			},
		}
	}

	hr := createHTTPRouteWithSectionNameAndPort(helpers.GetPointer[gatewayv1.SectionName]("listener-80-1"), nil)
	hrWithNilSectionName := createHTTPRouteWithSectionNameAndPort(nil, nil)
	hrWithEmptySectionName := createHTTPRouteWithSectionNameAndPort(helpers.GetPointer[gatewayv1.SectionName](""), nil)
	hrWithPort := createHTTPRouteWithSectionNameAndPort(
		helpers.GetPointer[gatewayv1.SectionName]("listener-80-1"),
		helpers.GetPointer[gatewayv1.PortNumber](80),
	)
	hrWithNonExistingListener := createHTTPRouteWithSectionNameAndPort(
		helpers.GetPointer[gatewayv1.SectionName]("listener-80-2"),
		nil,
	)

	var normalHTTPRoute *L7Route
	createNormalHTTPRoute := func(gateway *gatewayv1.Gateway) *L7Route {
		normalHTTPRoute = &L7Route{
			RouteType: RouteTypeHTTP,
			Source:    hr,
			Spec: L7RouteSpec{
				Hostnames: hr.Spec.Hostnames,
			},
			Valid:      true,
			Attachable: true,
			ParentRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gateway)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
				},
			},
		}
		return normalHTTPRoute
	}

	getLastNormalHTTPRoute := func() *L7Route {
		return normalHTTPRoute
	}

	invalidAttachableRoute1 := &L7Route{
		RouteType:  RouteTypeHTTP,
		Source:     hr,
		Valid:      false,
		Attachable: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr.Spec.ParentRefs[0].SectionName,
			},
		},
	}
	invalidAttachableRoute2 := &L7Route{
		RouteType:  RouteTypeHTTP,
		Source:     hr,
		Valid:      false,
		Attachable: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr.Spec.ParentRefs[0].SectionName,
			},
		},
	}

	routeWithMissingSectionName := &L7Route{
		RouteType:  RouteTypeHTTP,
		Source:     hrWithNilSectionName,
		Valid:      true,
		Attachable: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hrWithNilSectionName.Spec.ParentRefs[0].SectionName,
			},
		},
	}
	routeWithEmptySectionName := &L7Route{
		RouteType:  RouteTypeHTTP,
		Source:     hrWithEmptySectionName,
		Valid:      true,
		Attachable: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hrWithEmptySectionName.Spec.ParentRefs[0].SectionName,
			},
		},
	}
	routeWithNonExistingListener := &L7Route{
		RouteType:  RouteTypeHTTP,
		Source:     hrWithNonExistingListener,
		Valid:      true,
		Attachable: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hrWithNonExistingListener.Spec.ParentRefs[0].SectionName,
			},
		},
	}
	routeWithPort := &L7Route{
		RouteType:  RouteTypeHTTP,
		Source:     hrWithPort,
		Valid:      true,
		Attachable: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hrWithPort.Spec.ParentRefs[0].SectionName,
				Port:        hrWithPort.Spec.ParentRefs[0].Port,
			},
		},
	}
	invalidRoute := &L7Route{
		RouteType: RouteTypeHTTP,
		Valid:     false,
		Source:    hr,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr.Spec.ParentRefs[0].SectionName,
			},
		},
	}

	invalidNotAttachableListener := createModifiedListener("listener-80-1", func(l *Listener) {
		l.Valid = false
		l.Attachable = false
	})
	nonMatchingHostnameListener := createModifiedListener("listener-80-1", func(l *Listener) {
		l.Source.Hostname = helpers.GetPointer[gatewayv1.Hostname]("bar.example.com")
	})

	routeWithInvalidBackendRefs := createNormalHTTPRoute(gw)
	routeWithInvalidBackendRefs.Spec.Rules = []RouteRule{
		{
			BackendRefs: []BackendRef{
				{
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{
						client.ObjectKeyFromObject(gw): {Message: "invalid backend"},
					},
				},
			},
		},
	}

	createGRPCRouteWithSectionNameAndPort := func(
		sectionName *gatewayv1.SectionName,
		port *gatewayv1.PortNumber,
	) *gatewayv1.GRPCRoute {
		return &gatewayv1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "hr",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "GRPCRoute",
			},
			Spec: gatewayv1.GRPCRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{
							Name:        gatewayv1.ObjectName(gw.Name),
							SectionName: sectionName,
							Port:        port,
						},
					},
				},
				Hostnames: []gatewayv1.Hostname{
					"foo.example.com",
				},
			},
		}
	}

	gr := createGRPCRouteWithSectionNameAndPort(helpers.GetPointer[gatewayv1.SectionName]("listener-80-1"), nil)

	var normalGRPCRoute *L7Route
	createNormalGRPCRoute := func(gateway *gatewayv1.Gateway) *L7Route {
		normalGRPCRoute = &L7Route{
			RouteType: RouteTypeGRPC,
			Source:    gr,
			Spec: L7RouteSpec{
				Hostnames: gr.Spec.Hostnames,
			},
			Valid:      true,
			Attachable: true,
			ParentRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gateway)},
					SectionName: gr.Spec.ParentRefs[0].SectionName,
				},
			},
		}
		return normalGRPCRoute
	}

	getLastNormalGRPCRoute := func() *L7Route {
		return normalGRPCRoute
	}

	tests := []struct {
		route                    *L7Route
		gateway                  *Gateway
		expectedGatewayListeners []*Listener
		name                     string
		expectedSectionNameRefs  []ParentRef
		expectedConditions       []conditions.Condition
	}{
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): getLastNormalHTTPRoute(),
					}
				}),
			},
			name: "normal case",
		},
		{
			route: routeWithMissingSectionName,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hrWithNilSectionName.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): routeWithMissingSectionName,
					}
				}),
			},
			name: "section name is nil",
		},
		{
			route: routeWithEmptySectionName,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80"),
					createListener("listener-8080"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hrWithEmptySectionName.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80",
							): {"foo.example.com"},
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-8080",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80", func(l *Listener) {
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): routeWithEmptySectionName,
					}
				}),
				createModifiedListener("listener-8080", func(l *Listener) {
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): routeWithEmptySectionName,
					}
				}),
			},
			name: "section name is empty; bind to multiple listeners",
		},
		{
			route: routeWithEmptySectionName,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					invalidNotAttachableListener,
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hrWithEmptySectionName.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteInvalidListener()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				invalidNotAttachableListener,
			},
			name: "empty section name with no valid and attachable listeners",
		},
		{
			route: routeWithPort,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hrWithPort.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: false,
						FailedConditions: []conditions.Condition{
							staticConds.NewRouteUnsupportedValue(
								`spec.parentRefs[0].port: Forbidden: cannot be set`,
							),
						},
						AcceptedHostnames: map[string][]string{},
					},
					Port: hrWithPort.Spec.ParentRefs[0].Port,
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-80-1"),
			},
			name: "port is configured",
		},
		{
			route: routeWithNonExistingListener,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hrWithNonExistingListener.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNoMatchingParent()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-80-1"),
			},
			name: "listener doesn't exist",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					invalidNotAttachableListener,
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteInvalidListener()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				invalidNotAttachableListener,
			},
			name: "listener isn't valid and attachable",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					nonMatchingHostnameListener,
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNoMatchingListenerHostname()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				nonMatchingHostnameListener,
			},
			name: "no matching listener hostname",
		},
		{
			route: invalidRoute,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Attachment:  nil,
					SectionName: hr.Spec.ParentRefs[0].SectionName,
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-80-1"),
			},
			name: "route isn't valid",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  false,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteInvalidGateway()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-80-1"),
			},
			name: "invalid gateway",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Valid = false
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Valid = false
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): getLastNormalHTTPRoute(),
					}
				}),
			},
			expectedConditions: []conditions.Condition{staticConds.NewRouteInvalidListener()},
			name:               "invalid attachable listener",
		},
		{
			route: invalidAttachableRoute1,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): invalidAttachableRoute1,
					}
				}),
			},
			name: "invalid attachable route",
		},
		{
			route: invalidAttachableRoute2,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Valid = false
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Valid = false
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): invalidAttachableRoute2,
					}
				}),
			},
			expectedConditions: []conditions.Condition{staticConds.NewRouteInvalidListener()},
			name:               "invalid attachable listener with invalid attachable route",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{
								From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
							},
						}
						allowedLabels := map[string]string{"app": "not-allowed"}
						l.AllowedRouteLabelSelector = labels.SelectorFromSet(allowedLabels)
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNotAllowedByListeners()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
						},
					}
					allowedLabels := map[string]string{"app": "not-allowed"}
					l.AllowedRouteLabelSelector = labels.SelectorFromSet(allowedLabels)
				}),
			},
			name: "route not allowed via labels",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{
								From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
							},
						}
						allowedLabels := map[string]string{"app": "allowed"}
						l.AllowedRouteLabelSelector = labels.SelectorFromSet(allowedLabels)
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					allowedLabels := map[string]string{"app": "allowed"}
					l.AllowedRouteLabelSelector = labels.SelectorFromSet(allowedLabels)
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
						},
					}
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): getLastNormalHTTPRoute(),
					}
				}),
			},
			name: "route allowed via labels",
		},
		{
			route: createNormalHTTPRoute(gwDiffNamespace),
			gateway: &Gateway{
				Source: gwDiffNamespace,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{
								From: helpers.GetPointer(gatewayv1.NamespacesFromSame),
							},
						}
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gwDiffNamespace)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNotAllowedByListeners()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: helpers.GetPointer(gatewayv1.NamespacesFromSame),
						},
					}
				}),
			},
			name: "route not allowed via same namespace",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{
								From: helpers.GetPointer(gatewayv1.NamespacesFromSame),
							},
						}
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: helpers.GetPointer(gatewayv1.NamespacesFromSame),
						},
					}
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): getLastNormalHTTPRoute(),
					}
				}),
			},
			name: "route allowed via same namespace",
		},
		{
			route: createNormalHTTPRoute(gwDiffNamespace),
			gateway: &Gateway{
				Source: gwDiffNamespace,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{
								From: helpers.GetPointer(gatewayv1.NamespacesFromAll),
							},
						}
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gwDiffNamespace)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: helpers.GetPointer(gatewayv1.NamespacesFromAll),
						},
					}
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): getLastNormalHTTPRoute(),
					}
				}),
			},
			name: "route allowed via all namespaces",
		},
		{
			route: createNormalGRPCRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.SupportedKinds = []gatewayv1.RouteGroupKind{
							{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
						}
						l.Routes = map[RouteKey]*L7Route{
							CreateRouteKey(gr): getLastNormalGRPCRoute(),
						}
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: gr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached:          false,
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNotAllowedByListeners()},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.SupportedKinds = []gatewayv1.RouteGroupKind{
						{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
					}
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(gr): getLastNormalGRPCRoute(),
					}
				}),
			},
			name: "grpc route not allowed when listener kind is HTTPRoute",
		},
		{
			route: createNormalGRPCRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.SupportedKinds = []gatewayv1.RouteGroupKind{
							{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
						}
						l.Routes = map[RouteKey]*L7Route{
							CreateRouteKey(gr): getLastNormalGRPCRoute(),
						}
					}),
				},
				EffectiveNginxProxy: &EffectiveNginxProxy{
					DisableHTTP2: helpers.GetPointer(true),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx: 0,
					Gateway: &ParentRefGateway{
						NamespacedName: client.ObjectKeyFromObject(gw),
					},
					SectionName: gr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: false,
						FailedConditions: []conditions.Condition{
							staticConds.NewRouteUnsupportedConfiguration(
								`HTTP2 is disabled - cannot configure GRPCRoutes`,
							),
						},
						AcceptedHostnames: map[string][]string{},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.SupportedKinds = []gatewayv1.RouteGroupKind{
						{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
					}
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(gr): getLastNormalGRPCRoute(),
					}
				}),
			},
			name: "grpc route not allowed when HTTP2 is disabled",
		},
		{
			route: createNormalHTTPRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createModifiedListener("listener-80-1", func(l *Listener) {
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Kinds: []gatewayv1.RouteGroupKind{
								{Kind: "HTTPRoute"},
							},
						}
						l.Routes = map[RouteKey]*L7Route{
							CreateRouteKey(hr): getLastNormalHTTPRoute(),
						}
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Kinds: []gatewayv1.RouteGroupKind{
							{Kind: "HTTPRoute"},
						},
					}
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): getLastNormalHTTPRoute(),
					}
				}),
			},
			name: "http route allowed when listener kind is HTTPRoute",
		},
		{
			route: routeWithInvalidBackendRefs,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				Listeners: []*Listener{
					createListener("listener-80-1"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: hr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						FailedConditions: []conditions.Condition{
							{Message: "invalid backend"},
						},
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-80-1",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-80-1", func(l *Listener) {
					l.Routes = map[RouteKey]*L7Route{
						CreateRouteKey(hr): routeWithInvalidBackendRefs,
					}
				}),
			},
			name: "route still allowed if backendRef failure conditions exist",
		},
	}

	namespaces := map[types.NamespacedName]*v1.Namespace{
		{Name: "test"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"app": "allowed"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			bindL7RouteToListeners(
				test.route,
				test.gateway,
				namespaces,
			)

			g.Expect(test.route.ParentRefs).To(Equal(test.expectedSectionNameRefs))
			g.Expect(helpers.Diff(test.gateway.Listeners, test.expectedGatewayListeners)).To(BeEmpty())
			g.Expect(helpers.Diff(test.route.Conditions, test.expectedConditions)).To(BeEmpty())
		})
	}
}

func TestFindAcceptedHostnames(t *testing.T) {
	t.Parallel()
	var listenerHostnameFoo gatewayv1.Hostname = "foo.example.com"
	var listenerHostnameCafe gatewayv1.Hostname = "cafe.example.com"
	var listenerHostnameWildcard gatewayv1.Hostname = "*.example.com"
	routeHostnames := []gatewayv1.Hostname{"foo.example.com", "bar.example.com"}

	tests := []struct {
		listenerHostname *gatewayv1.Hostname
		msg              string
		routeHostnames   []gatewayv1.Hostname
		expected         []string
	}{
		{
			listenerHostname: &listenerHostnameFoo,
			routeHostnames:   routeHostnames,
			expected:         []string{"foo.example.com"},
			msg:              "one match",
		},
		{
			listenerHostname: &listenerHostnameCafe,
			routeHostnames:   routeHostnames,
			expected:         nil,
			msg:              "no match",
		},
		{
			listenerHostname: nil,
			routeHostnames:   routeHostnames,
			expected:         []string{"foo.example.com", "bar.example.com"},
			msg:              "nil listener hostname",
		},
		{
			listenerHostname: &listenerHostnameFoo,
			routeHostnames:   nil,
			expected:         []string{"foo.example.com"},
			msg:              "route has empty hostnames",
		},
		{
			listenerHostname: nil,
			routeHostnames:   nil,
			expected:         []string{wildcardHostname},
			msg:              "both listener and route have empty hostnames",
		},
		{
			listenerHostname: &listenerHostnameWildcard,
			routeHostnames:   routeHostnames,
			expected:         []string{"foo.example.com", "bar.example.com"},
			msg:              "listener wildcard hostname",
		},
		{
			listenerHostname: &listenerHostnameFoo,
			routeHostnames:   []gatewayv1.Hostname{"*.example.com"},
			expected:         []string{"foo.example.com"},
			msg:              "route wildcard hostname; specific listener hostname",
		},
		{
			listenerHostname: &listenerHostnameWildcard,
			routeHostnames:   nil,
			expected:         []string{"*.example.com"},
			msg:              "listener wildcard hostname; nil route hostname",
		},
		{
			listenerHostname: nil,
			routeHostnames:   []gatewayv1.Hostname{"*.example.com"},
			expected:         []string{"*.example.com"},
			msg:              "route wildcard hostname; nil listener hostname",
		},
		{
			listenerHostname: &listenerHostnameWildcard,
			routeHostnames:   []gatewayv1.Hostname{"*.bar.example.com"},
			expected:         []string{"*.bar.example.com"},
			msg:              "route and listener wildcard hostnames",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := findAcceptedHostnames(test.listenerHostname, test.routeHostnames)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestGetHostname(t *testing.T) {
	t.Parallel()
	var emptyHostname gatewayv1.Hostname
	var hostname gatewayv1.Hostname = "example.com"

	tests := []struct {
		h        *gatewayv1.Hostname
		expected string
		msg      string
	}{
		{
			h:        nil,
			expected: "",
			msg:      "nil hostname",
		},
		{
			h:        &emptyHostname,
			expected: "",
			msg:      "empty hostname",
		},
		{
			h:        &hostname,
			expected: string(hostname),
			msg:      "normal hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := getHostname(test.h)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestValidateHostnames(t *testing.T) {
	t.Parallel()
	const validHostname = "example.com"

	tests := []struct {
		name      string
		hostnames []gatewayv1.Hostname
		expectErr bool
	}{
		{
			hostnames: []gatewayv1.Hostname{
				validHostname,
				"example.org",
				"foo.example.net",
			},
			expectErr: false,
			name:      "multiple valid",
		},
		{
			hostnames: []gatewayv1.Hostname{
				validHostname,
				"",
			},
			expectErr: true,
			name:      "valid and invalid",
		},
	}

	path := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			err := validateHostnames(test.hostnames, path)

			if test.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestRouteKeyForKind(t *testing.T) {
	t.Parallel()
	nsname := types.NamespacedName{Namespace: testNs, Name: "route"}

	g := NewWithT(t)

	key := routeKeyForKind(kinds.HTTPRoute, nsname)
	g.Expect(key).To(Equal(RouteKey{RouteType: RouteTypeHTTP, NamespacedName: nsname}))

	key = routeKeyForKind(kinds.GRPCRoute, nsname)
	g.Expect(key).To(Equal(RouteKey{RouteType: RouteTypeGRPC, NamespacedName: nsname}))

	rk := func() {
		_ = routeKeyForKind(kinds.Gateway, nsname)
	}

	g.Expect(rk).To(Panic())
}

func TestAllowedRouteType(t *testing.T) {
	t.Parallel()
	test := []struct {
		listener  *Listener
		name      string
		routeType RouteType
		expResult bool
	}{
		{
			name:      "grpcRoute is allowed when listener supports grpcRoute kind",
			routeType: RouteTypeGRPC,
			listener: &Listener{
				SupportedKinds: []gatewayv1.RouteGroupKind{
					{Kind: kinds.GRPCRoute},
				},
			},
			expResult: true,
		},
		{
			name:      "grpcRoute is allowed when listener supports grpcRoute and httpRoute kind",
			routeType: RouteTypeGRPC,
			listener: &Listener{
				SupportedKinds: []gatewayv1.RouteGroupKind{
					{Kind: kinds.HTTPRoute},
					{Kind: kinds.GRPCRoute},
				},
			},
			expResult: true,
		},
		{
			name:      "grpcRoute is allowed when listener supports httpRoute kind",
			routeType: RouteTypeGRPC,
			listener: &Listener{
				SupportedKinds: []gatewayv1.RouteGroupKind{
					{Kind: kinds.HTTPRoute},
				},
			},
			expResult: false,
		},
		{
			name:      "httpRoute not allowed when listener supports grpcRoute kind",
			routeType: RouteTypeHTTP,
			listener: &Listener{
				SupportedKinds: []gatewayv1.RouteGroupKind{
					{Kind: kinds.GRPCRoute},
				},
			},
			expResult: false,
		},
	}

	for _, test := range test {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(isRouteTypeAllowedByListener(test.listener, convertRouteType(test.routeType))).To(Equal(test.expResult))
		})
	}
}

func TestBindL4RouteToListeners(t *testing.T) {
	t.Parallel()
	// we create a new listener each time because the function under test can modify it
	createListener := func(name string) *Listener {
		return &Listener{
			Name: name,
			GatewayName: types.NamespacedName{
				Namespace: "test",
				Name:      "gateway",
			},
			Source: gatewayv1.Listener{
				Name:     gatewayv1.SectionName(name),
				Hostname: (*gatewayv1.Hostname)(helpers.GetPointer("foo.example.com")),
				Protocol: gatewayv1.TLSProtocolType,
				TLS: helpers.GetPointer(gatewayv1.GatewayTLSConfig{
					Mode: helpers.GetPointer(gatewayv1.TLSModeTerminate),
				}),
			},
			SupportedKinds: []gatewayv1.RouteGroupKind{
				{Kind: kinds.TLSRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
			},
			Valid:      true,
			Attachable: true,
			Routes:     map[RouteKey]*L7Route{},
			L4Routes:   map[L4RouteKey]*L4Route{},
		}
	}
	createModifiedListener := func(name string, m func(*Listener)) *Listener {
		l := createListener(name)
		m(l)
		return l
	}

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway",
		},
	}

	gwWrongNamespace := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "wrong",
			Name:      "gateway",
		},
	}

	createTLSRouteWithSectionNameAndPort := func(
		sectionName *gatewayv1.SectionName,
		port *gatewayv1.PortNumber,
		ns string,
	) *v1alpha2.TLSRoute {
		return &v1alpha2.TLSRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "hr",
			},
			Spec: v1alpha2.TLSRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{
							Name:        gatewayv1.ObjectName(gw.Name),
							SectionName: sectionName,
							Port:        port,
						},
					},
				},
				Hostnames: []gatewayv1.Hostname{
					"foo.example.com",
				},
			},
		}
	}

	tr := createTLSRouteWithSectionNameAndPort(helpers.GetPointer[gatewayv1.SectionName]("listener-443"), nil, "test")

	var normalRoute *L4Route
	createNormalRoute := func(gateway *gatewayv1.Gateway) *L4Route {
		normalRoute = &L4Route{
			Source: tr,
			Spec: L4RouteSpec{
				Hostnames: tr.Spec.Hostnames,
			},
			Valid:      true,
			Attachable: true,
			ParentRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gateway)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
				},
			},
		}
		return normalRoute
	}

	makeModifiedRoute := func(gateway *gatewayv1.Gateway, m func(r *L4Route)) *L4Route {
		normalRoute = createNormalRoute(gateway)
		m(normalRoute)
		return normalRoute
	}
	getLastNormalRoute := func() *L4Route {
		return normalRoute
	}

	noMatchingParentAttachment := ParentRefAttachmentStatus{
		AcceptedHostnames: map[string][]string{},
		FailedConditions:  []conditions.Condition{staticConds.NewRouteNoMatchingParent()},
	}

	notAttachableRoute := &L4Route{
		Source: tr,
		Spec: L4RouteSpec{
			Hostnames: tr.Spec.Hostnames,
		},
		Valid: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr.Spec.ParentRefs[0].SectionName,
			},
		},
	}
	notAttachableRoutePort := &L4Route{
		Source: tr,
		Spec: L4RouteSpec{
			Hostnames: tr.Spec.Hostnames,
		},
		Valid: true,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr.Spec.ParentRefs[0].SectionName,
				Port:        helpers.GetPointer[gatewayv1.PortNumber](80),
			},
		},
		Attachable: true,
	}

	routeWithInvalidBackendRefs := createNormalRoute(gw)
	routeWithInvalidBackendRefs.Spec.BackendRef = BackendRef{
		InvalidForGateways: map[types.NamespacedName]conditions.Condition{
			client.ObjectKeyFromObject(gw): {Message: "invalid backend"},
		},
	}

	tests := []struct {
		route                    *L4Route
		gateway                  *Gateway
		expectedGatewayListeners []*Listener
		name                     string
		expectedSectionNameRefs  []ParentRef
		expectedConditions       []conditions.Condition
	}{
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-443",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.L4Routes = map[L4RouteKey]*L4Route{
						CreateRouteKeyL4(tr): getLastNormalRoute(),
					}
				}),
			},
			name: "normal case",
		},
		{
			route: notAttachableRoute,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-443"),
			},
			name: "route is not attachable",
		},
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-444"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Attachment:  &noMatchingParentAttachment,
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Idx:         0,
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-444"),
			},
			name: "section name is wrong",
		},
		{
			route: notAttachableRoutePort,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{},
						FailedConditions: []conditions.Condition{
							staticConds.NewRouteUnsupportedValue(
								`spec.parentRefs[0].port: Forbidden: cannot be set`,
							),
						},
						Attached: false,
					},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Idx:         0,
					Port:        helpers.GetPointer[gatewayv1.PortNumber](80),
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-443"),
			},
			name: "port is not nil",
		},
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  false,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{},
						FailedConditions:  []conditions.Condition{staticConds.NewRouteInvalidGateway()},
						Attached:          false,
					},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Idx:         0,
				},
			},
			expectedGatewayListeners: []*Listener{
				createListener("listener-443"),
			},
			name: "invalid gateway",
		},
		{
			route: createNormalRoute(gwWrongNamespace),
			gateway: &Gateway{
				Source: gwWrongNamespace,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "wrong",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createModifiedListener("listener-443", func(l *Listener) {
						l.GatewayName = client.ObjectKeyFromObject(gwWrongNamespace)
						l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{From: helpers.GetPointer(
								gatewayv1.FromNamespaces("Same"),
							)},
						}
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gwWrongNamespace)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{},
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNotAllowedByListeners()},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.GatewayName = client.ObjectKeyFromObject(gwWrongNamespace)
					l.Source.AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{From: helpers.GetPointer(
							gatewayv1.FromNamespaces("Same"),
						)},
					}
				}),
			},
			name: "route not allowed by listener; in different namespace",
		},
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createModifiedListener("listener-443", func(l *Listener) {
						l.Valid = false
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-443",
							): {"foo.example.com"},
						},
						Attached: true,
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.Valid = false
					r := createNormalRoute(gw)
					r.Conditions = append(r.Conditions, staticConds.NewRouteInvalidListener())
					r.ParentRefs = []ParentRef{
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: tr.Spec.ParentRefs[0].SectionName,
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw),
										"listener-443",
									): {"foo.example.com"},
								},
								Attached: true,
							},
						},
					}
					l.L4Routes = map[L4RouteKey]*L4Route{
						CreateRouteKeyL4(tr): r,
					}
				}),
			},
			expectedConditions: []conditions.Condition{staticConds.NewRouteInvalidListener()},
			name:               "invalid attachable listener",
		},
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Valid: true,
				Listeners: []*Listener{
					createModifiedListener("listener-443", func(l *Listener) {
						l.Source.Hostname = (*gatewayv1.Hostname)(helpers.GetPointer("*.example.org"))
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{},
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNoMatchingListenerHostname()},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.Source.Hostname = (*gatewayv1.Hostname)(helpers.GetPointer("*.example.org"))
				}),
			},
			name: "route hostname does not match any listener",
		},
		{
			route: makeModifiedRoute(gw, func(r *L4Route) {
				r.ParentRefs[0].SectionName = nil
			}),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:     0,
					Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-443",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.L4Routes = map[L4RouteKey]*L4Route{
						CreateRouteKeyL4(tr): getLastNormalRoute(),
					}
				}),
			},
			name: "nil section name",
		},
		{
			route: makeModifiedRoute(gw, func(r *L4Route) {
				r.ParentRefs[0].SectionName = helpers.GetPointer[gatewayv1.SectionName]("")
			}),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:     0,
					Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-443",
							): {"foo.example.com"},
						},
					},
					SectionName: helpers.GetPointer[gatewayv1.SectionName](""),
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.L4Routes = map[L4RouteKey]*L4Route{
						CreateRouteKeyL4(tr): getLastNormalRoute(),
					}
				}),
			},
			name: "empty section name",
		},
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Valid:     true,
				Listeners: []*Listener{},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Attachment:  &noMatchingParentAttachment,
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Idx:         0,
				},
			},
			expectedGatewayListeners: []*Listener{},
			name:                     "listener does not exist",
		},
		{
			route: makeModifiedRoute(gw, func(r *L4Route) {
				r.Valid = false
			}),
			gateway: &Gateway{
				Source: gw,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Valid: true,
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:     0,
					Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-443",
							): {"foo.example.com"},
						},
					},
					SectionName: helpers.GetPointer[gatewayv1.SectionName]("listener-443"),
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.L4Routes = map[L4RouteKey]*L4Route{
						CreateRouteKeyL4(tr): getLastNormalRoute(),
					}
				}),
			},
			name: "invalid attachable route",
		},
		{
			route: createNormalRoute(gw),
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createModifiedListener("listener-443", func(l *Listener) {
						l.SupportedKinds = nil
					}),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:     0,
					Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{},
						FailedConditions:  []conditions.Condition{staticConds.NewRouteNotAllowedByListeners()},
					},
					SectionName: helpers.GetPointer[gatewayv1.SectionName]("listener-443"),
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.SupportedKinds = nil
				}),
			},
			name: "route kind not allowed",
		},
		{
			route: routeWithInvalidBackendRefs,
			gateway: &Gateway{
				Source: gw,
				Valid:  true,
				DeploymentName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Listeners: []*Listener{
					createListener("listener-443"),
				},
			},
			expectedSectionNameRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: tr.Spec.ParentRefs[0].SectionName,
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
						FailedConditions: []conditions.Condition{
							{Message: "invalid backend"},
						},
						AcceptedHostnames: map[string][]string{
							CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw),
								"listener-443",
							): {"foo.example.com"},
						},
					},
				},
			},
			expectedGatewayListeners: []*Listener{
				createModifiedListener("listener-443", func(l *Listener) {
					l.L4Routes = map[L4RouteKey]*L4Route{
						CreateRouteKeyL4(tr): routeWithInvalidBackendRefs,
					}
				}),
			},
			name: "route still allowed if backendRef failure conditions exist",
		},
	}

	namespaces := map[types.NamespacedName]*v1.Namespace{
		{Name: "test"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"app": "allowed"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			bindL4RouteToListeners(
				test.route,
				test.gateway,
				namespaces,
				map[string]struct{}{},
			)

			g.Expect(test.route.ParentRefs).To(Equal(test.expectedSectionNameRefs))
			g.Expect(helpers.Diff(test.gateway.Listeners, test.expectedGatewayListeners)).To(BeEmpty())
			g.Expect(helpers.Diff(test.route.Conditions, test.expectedConditions)).To(BeEmpty())
		})
	}
}

func TestBuildL4RoutesForGateways_NoGateways(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	nsName := types.NamespacedName{Namespace: testNs, Name: "hi"}

	tlsRoutes := map[types.NamespacedName]*v1alpha2.TLSRoute{
		nsName: {
			Spec: v1alpha2.TLSRouteSpec{
				Hostnames: []v1alpha2.Hostname{"app.example.com"},
			},
		},
	}

	services := map[types.NamespacedName]*v1.Service{
		nsName: {
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{},
			},
		},
	}

	refGrantResolver := newReferenceGrantResolver(nil)

	g.Expect(buildL4RoutesForGateways(
		tlsRoutes,
		services,
		nil,
		refGrantResolver,
	)).To(BeNil())
}

func TestTryToAttachL4RouteToListeners_NoAttachableListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	route := &L4Route{
		Spec: L4RouteSpec{
			Hostnames: []v1alpha2.Hostname{"app.example.com"},
		},
		Valid:      true,
		Attachable: true,
	}

	gw := &Gateway{
		Valid: true,
		Listeners: []*Listener{
			{
				Name: "listener1",
			},
			{
				Name: "listener2",
			},
		},
	}

	cond, attachable := tryToAttachL4RouteToListeners(
		nil,
		nil,
		route,
		gw,
		nil,
		map[string]struct{}{},
	)
	g.Expect(cond).To(Equal(staticConds.NewRouteInvalidListener()))
	g.Expect(attachable).To(BeFalse())
}

type parentRef struct {
	sectionName *gatewayv1.SectionName
	gw          types.NamespacedName
}

func TestIsolateL4Listeners(t *testing.T) {
	t.Parallel()
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway",
		},
	}

	gw1 := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway1",
		},
	}

	createTLSRouteWithSectionNameAndPort := func(
		name string,
		parentRef []parentRef,
		ns string,
		hostnames ...gatewayv1.Hostname,
	) *v1alpha2.TLSRoute {
		var parentRefs []gatewayv1.ParentReference
		for _, p := range parentRef {
			parentRefs = append(parentRefs, gatewayv1.ParentReference{
				Name:        gatewayv1.ObjectName(p.gw.Name),
				SectionName: p.sectionName,
			})
		}
		return &v1alpha2.TLSRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Spec: v1alpha2.TLSRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: parentRefs,
				},
				Hostnames: hostnames,
			},
		}
	}

	routeHostnames := []gatewayv1.Hostname{"bar.com", "*.example.com", "*.foo.example.com", "abc.foo.example.com"}
	tr1 := createTLSRouteWithSectionNameAndPort(
		"tr1",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("empty-hostname"),
			},
		},
		"test",
		routeHostnames...,
	)
	tr2 := createTLSRouteWithSectionNameAndPort(
		"tr2",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
		},
		"test",
		routeHostnames...,
	)
	tr3 := createTLSRouteWithSectionNameAndPort(
		"tr3",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("foo-wildcard-example-com"),
			},
		},
		"test",
		routeHostnames...,
	)
	tr4 := createTLSRouteWithSectionNameAndPort(
		"tr4",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("abc-com"),
			},
		},
		"test",
		routeHostnames...,
	)
	tr5 := createTLSRouteWithSectionNameAndPort(
		"tr5",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("no-match"),
			},
		},
		"test",
		routeHostnames...,
	)

	createL4RoutewithAcceptedHostnames := func(
		source *v1alpha2.TLSRoute,
		acceptedHostnames map[string][]string,
		hostnames []gatewayv1.Hostname,
		sectionName *gatewayv1.SectionName,
		listenerPort int32,
	) *L4Route {
		return &L4Route{
			Source: source,
			Spec: L4RouteSpec{
				Hostnames: hostnames,
			},
			ParentRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: sectionName,
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: acceptedHostnames,
						Attached:          true,
						ListenerPort:      gatewayv1.PortNumber(listenerPort),
					},
				},
			},
		}
	}

	acceptedHostnamesEmptyHostname := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "empty-hostname"): {
			"bar.com", "*.example.com", "*.foo.example.com", "abc.foo.example.com",
		},
	}
	acceptedHostnamesWildcardExample := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "wildcard-example-com"): {
			"*.example.com", "*.foo.example.com", "abc.foo.example.com",
		},
	}

	acceptedHostnamesFooWildcardExample := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "foo-wildcard-example-com"): {
			"*.foo.example.com", "abc.foo.example.com",
		},
	}

	acceptedHostnamesAbcCom := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "abc-com"): {
			"abc.foo.example.com",
		},
	}
	acceptedHostnamesNoMatch := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "no-match"): {},
	}

	routesHostnameIntersection := []*L4Route{
		createL4RoutewithAcceptedHostnames(
			tr1, acceptedHostnamesEmptyHostname,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("empty-hostname"),
			80,
		),
		createL4RoutewithAcceptedHostnames(
			tr2,
			acceptedHostnamesWildcardExample,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			80,
		),
		createL4RoutewithAcceptedHostnames(
			tr3,
			acceptedHostnamesFooWildcardExample,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("foo-wildcard-example-com"),
			80,
		),
		createL4RoutewithAcceptedHostnames(
			tr4,
			acceptedHostnamesAbcCom,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("abc-com"),
			80,
		),
		createL4RoutewithAcceptedHostnames(
			tr5,
			acceptedHostnamesNoMatch,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("no-match"),
			80,
		),
	}

	listenerMapHostnameIntersection := map[string]hostPort{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "empty-hostname"): {
			hostname: "",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "wildcard-example-com"): {
			hostname: "*.example.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "foo-wildcard-example-com"): {
			hostname: "*.foo.example.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "abc-com"): {
			hostname: "abc.foo.example.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "no-match"): {
			hostname: "no-match.cafe.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
	}

	expectedResultHostnameIntersection := map[string][]ParentRef{
		"tr1": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr1.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "empty-hostname"): {"bar.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"tr2": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr2.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(
							client.ObjectKeyFromObject(gw),
							"wildcard-example-com",
						): {"*.example.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"tr3": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr3.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(
							client.ObjectKeyFromObject(gw),
							"foo-wildcard-example-com",
						): {"*.foo.example.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"tr4": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr4.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "abc-com"): {"abc.foo.example.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"tr5": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: tr5.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "no-match"): {},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
	}

	routeHostnameNoSectionName := []gatewayv1.Hostname{"tea.example.com", "coffee.example.com", "flavor.example.com"}
	tlsCoffeeRoute := createTLSRouteWithSectionNameAndPort(
		"tls_coffee",
		nil,
		"test",
		routeHostnameNoSectionName...,
	)

	tlsTeaRoute := createTLSRouteWithSectionNameAndPort(
		"tls_tea",
		nil,
		"test",
		routeHostnameNoSectionName...,
	)

	tlsFlavorRoute := createTLSRouteWithSectionNameAndPort(
		"tls_flavor",
		nil,
		"test",
		routeHostnameNoSectionName...,
	)

	acceptedHostnamesNoSectionName := map[string][]string{
		"tls_coffee": {"coffee.example.com"},
		"tls_tea":    {"tea.example.com"},
		"tls_flavor": {"flavor.example.com"},
	}

	routeHostname := []gatewayv1.Hostname{"coffee.example.com", "flavor.example.com"}
	acceptedHostanamesMultipleGateways := map[string][]string{
		"tls_coffee": {"coffee.example.com", "flavor.example.com"},
		"tls_flavor": {"coffee.example.com", "flavor.example.com"},
	}
	tlsCoffeeRoute1 := createTLSRouteWithSectionNameAndPort(
		"tls_coffee",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
			{
				gw:          client.ObjectKeyFromObject(gw1),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
		},
		"test",
		routeHostname...,
	)

	tlsFlavorRoute1 := createTLSRouteWithSectionNameAndPort(
		"tls_flavor",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
			{
				gw:          client.ObjectKeyFromObject(gw1),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
		},
		"test",
		routeHostname...,
	)

	tests := []struct {
		expectedResult map[string][]ParentRef
		listenerMap    map[string]hostPort
		name           string
		routes         []*L4Route
	}{
		{
			name:           "isolate listeners based on hostname intersection for different routes",
			routes:         routesHostnameIntersection,
			listenerMap:    listenerMapHostnameIntersection,
			expectedResult: expectedResultHostnameIntersection,
		},
		{
			name: "no listener isolation for routes with no section name, attaches to all listeners",
			routes: []*L4Route{
				createL4RoutewithAcceptedHostnames(
					tlsCoffeeRoute,
					acceptedHostnamesNoSectionName,
					routeHostnameNoSectionName,
					nil, // no section name
					443,
				),
				createL4RoutewithAcceptedHostnames(
					tlsTeaRoute,
					acceptedHostnamesNoSectionName,
					routeHostnameNoSectionName,
					nil, // no section name
					443,
				),
				createL4RoutewithAcceptedHostnames(
					tlsFlavorRoute,
					acceptedHostnamesNoSectionName,
					routeHostnameNoSectionName,
					nil, // no section name
					443,
				),
			},
			listenerMap: map[string]hostPort{
				"tls_coffee,test,gateway": {hostname: "coffee.example.com", port: 443},
				"tls_tea,test,gateway":    {hostname: "tea.example.com", port: 443},
				"tls_flavor,test,gateway": {hostname: "flavor.example.com", port: 443},
			},
			expectedResult: map[string][]ParentRef{
				"tls_coffee": {
					{
						Idx:     0,
						Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com"},
								"tls_tea":    {"tea.example.com"},
								"tls_flavor": {"flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
				},
				"tls_tea": {
					{
						Idx:     0,
						Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com"},
								"tls_tea":    {"tea.example.com"},
								"tls_flavor": {"flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
				},
				"tls_flavor": {
					{
						Idx:     0,
						Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com"},
								"tls_tea":    {"tea.example.com"},
								"tls_flavor": {"flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
				},
			},
		},
		{
			name: "no listener isolation for routes with overlapping hostnames but different gateways",
			routes: []*L4Route{
				{
					Source: tlsCoffeeRoute1,
					Spec: L4RouteSpec{
						Hostnames: routeHostname,
					},
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostanamesMultipleGateways,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(443),
							},
						},
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostanamesMultipleGateways,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(443),
							},
						},
					},
				},
				{
					Source: tlsFlavorRoute1,
					Spec: L4RouteSpec{
						Hostnames: routeHostname,
					},
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostanamesMultipleGateways,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(443),
							},
						},
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostanamesMultipleGateways,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(443),
							},
						},
					},
				},
			},
			listenerMap: map[string]hostPort{
				"wildcard-example-com,test,gateway": {
					hostname: "*.example.com",
					port:     443,
					gwNsName: client.ObjectKeyFromObject(gw),
				},
				"wildcard-example-com,test,gateway1": {
					hostname: "*.example.com",
					port:     443,
					gwNsName: client.ObjectKeyFromObject(gw),
				},
			},
			expectedResult: map[string][]ParentRef{
				"tls_coffee": {
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: tlsCoffeeRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com", "flavor.example.com"},
								"tls_flavor": {"coffee.example.com", "flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: tlsCoffeeRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com", "flavor.example.com"},
								"tls_flavor": {"coffee.example.com", "flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
				},
				"tls_flavor": {
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: tlsFlavorRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com", "flavor.example.com"},
								"tls_flavor": {"coffee.example.com", "flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: tlsCoffeeRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"tls_coffee": {"coffee.example.com", "flavor.example.com"},
								"tls_flavor": {"coffee.example.com", "flavor.example.com"},
							},
							ListenerPort: 443,
							Attached:     true,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			isolateL4RouteListeners(test.routes, test.listenerMap)

			result := map[string][]ParentRef{}
			for _, route := range test.routes {
				result[route.Source.GetName()] = route.ParentRefs
			}
			g.Expect(helpers.Diff(result, test.expectedResult)).To(BeEmpty())
		})
	}
}

func TestIsolateL7Listeners(t *testing.T) {
	t.Parallel()
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway",
		},
	}

	gw1 := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway1",
		},
	}

	createHTTPRouteWithSectionNameAndPort := func(
		name string,
		parentRef []parentRef,
		ns string,
		hostnames ...gatewayv1.Hostname,
	) *gatewayv1.HTTPRoute {
		var parentRefs []gatewayv1.ParentReference
		for _, p := range parentRef {
			parentRefs = append(parentRefs, gatewayv1.ParentReference{
				Name:        gatewayv1.ObjectName(p.gw.Name),
				SectionName: p.sectionName,
			})
		}
		return &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: parentRefs,
				},
				Hostnames: hostnames,
			},
		}
	}

	createL7RoutewithAcceptedHostnames := func(
		source *gatewayv1.HTTPRoute,
		acceptedHostnames map[string][]string,
		hostnames []gatewayv1.Hostname,
		sectionName *gatewayv1.SectionName,
		listenerPort int32,
	) *L7Route {
		return &L7Route{
			Source: source,
			Spec: L7RouteSpec{
				Hostnames: hostnames,
			},
			ParentRefs: []ParentRef{
				{
					Idx:         0,
					Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
					SectionName: sectionName,
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: acceptedHostnames,
						Attached:          true,
						ListenerPort:      gatewayv1.PortNumber(listenerPort),
					},
				},
			},
		}
	}

	routeHostnames := []gatewayv1.Hostname{"bar.com", "*.example.com", "*.foo.example.com", "abc.foo.example.com"}
	hr1 := createHTTPRouteWithSectionNameAndPort(
		"hr1",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("empty-hostname"),
			},
		},
		"test",
		routeHostnames...,
	)
	hr2 := createHTTPRouteWithSectionNameAndPort(
		"hr2",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
		},
		"test",
		routeHostnames...,
	)
	hr3 := createHTTPRouteWithSectionNameAndPort(
		"hr3",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("foo-wildcard-example-com"),
			},
		},
		"test",
		routeHostnames...,
	)
	hr4 := createHTTPRouteWithSectionNameAndPort(
		"hr4",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("abc-com"),
			},
		},
		"test",
		routeHostnames...,
	)
	hr5 := createHTTPRouteWithSectionNameAndPort(
		"hr5",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("no-match"),
			},
		},
		"test",
		routeHostnames..., // no matching hostname
	)

	acceptedHostnamesEmptyHostname := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "empty-hostname"): {
			"bar.com", "*.example.com", "*.foo.example.com", "abc.foo.example.com",
		},
	}
	acceptedHostnamesWildcardExample := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "wildcard-example-com"): {
			"*.example.com", "*.foo.example.com", "abc.foo.example.com",
		},
	}

	acceptedHostnamesFooWildcardExample := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "foo-wildcard-example-com"): {
			"*.foo.example.com", "abc.foo.example.com",
		},
	}

	acceptedHostnamesAbcCom := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "abc-com"): {
			"abc.foo.example.com",
		},
	}
	acceptedHostnamesNoMatch := map[string][]string{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "no-match"): {},
	}

	routesHostnameIntersection := []*L7Route{
		createL7RoutewithAcceptedHostnames(
			hr1,
			acceptedHostnamesEmptyHostname,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("empty-hostname"),
			80,
		),
		createL7RoutewithAcceptedHostnames(
			hr2,
			acceptedHostnamesWildcardExample,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			80,
		),
		createL7RoutewithAcceptedHostnames(
			hr3,
			acceptedHostnamesFooWildcardExample,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("foo-wildcard-example-com"),
			80,
		),
		createL7RoutewithAcceptedHostnames(
			hr4,
			acceptedHostnamesAbcCom,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("abc-com"),
			80,
		),
		createL7RoutewithAcceptedHostnames(
			hr5,
			acceptedHostnamesNoMatch,
			routeHostnames,
			helpers.GetPointer[gatewayv1.SectionName]("no-match"),
			80,
		),
	}

	listenerMapHostnameIntersection := map[string]hostPort{
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "empty-hostname"): {
			hostname: "",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "wildcard-example-com"): {
			hostname: "*.example.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "foo-wildcard-example-com"): {
			hostname: "*.foo.example.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "abc-com"): {
			hostname: "abc.foo.example.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
		CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "no-match"): {
			hostname: "no-match.cafe.com",
			port:     80,
			gwNsName: client.ObjectKeyFromObject(gw),
		},
	}

	expectedResultHostnameIntersection := map[string][]ParentRef{
		"hr1": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr1.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "empty-hostname"): {"bar.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"hr2": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr2.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(
							client.ObjectKeyFromObject(gw),
							"wildcard-example-com",
						): {"*.example.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"hr3": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr3.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(
							client.ObjectKeyFromObject(gw),
							"foo-wildcard-example-com",
						): {"*.foo.example.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"hr4": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr4.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "abc-com"): {"abc.foo.example.com"},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
		"hr5": {
			{
				Idx:         0,
				Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
				SectionName: hr5.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						CreateGatewayListenerKey(client.ObjectKeyFromObject(gw), "no-match"): {},
					},
					Attached:     true,
					ListenerPort: 80,
				},
			},
		},
	}

	routeHostnameCafeExample := []gatewayv1.Hostname{"cafe.example.com"}
	httpListenerRoute := createHTTPRouteWithSectionNameAndPort(
		"hr_cafe",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("http"),
			},
		},
		"test",
		routeHostnameCafeExample...,
	)

	acceptedHostnamesHTTP := map[string][]string{
		"http": {
			"cafe.example.com",
		},
	}

	routeHostnameNoSectionName := []gatewayv1.Hostname{"tea.example.com", "coffee.example.com", "flavor.example.com"}
	hrCoffeeRoute := createHTTPRouteWithSectionNameAndPort(
		"hr_coffee",
		nil,
		"test",
		routeHostnameNoSectionName...,
	)

	hrTeaRoute := createHTTPRouteWithSectionNameAndPort(
		"hr_tea",
		nil,
		"test",
		routeHostnameNoSectionName...,
	)

	hrFlavorRoute := createHTTPRouteWithSectionNameAndPort(
		"hr_flavor",
		nil,
		"test",
		routeHostnameNoSectionName...,
	)

	acceptedHostnamesNoSectionName := map[string][]string{
		"hr_coffee": {"coffee.example.com"},
		"hr_tea":    {"tea.example.com"},
		"hr_flavor": {"flavor.example.com"},
	}

	routeHostname := []gatewayv1.Hostname{"cafe.example.com", "flavor.example.com"}

	acceptedHostNamesMultipleGateway := map[string][]string{
		"hr_cafe":   {"cafe.example.com", "flavor.example.com"},
		"hr_flavor": {"cafe.example.com", "flavor.example.com"},
	}

	hrCoffeeRoute1 := createHTTPRouteWithSectionNameAndPort(
		"hr_coffee",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
			{
				gw:          client.ObjectKeyFromObject(gw1),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
		},
		"test",
		routeHostname...,
	)

	hrFlavorRoute1 := createHTTPRouteWithSectionNameAndPort(
		"hr_flavor",
		[]parentRef{
			{
				gw:          client.ObjectKeyFromObject(gw),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
			{
				gw:          client.ObjectKeyFromObject(gw1),
				sectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
			},
		},
		"test",
		routeHostname...,
	)

	tests := []struct {
		expectedResult map[string][]ParentRef
		listenersMap   map[string]hostPort
		name           string
		routes         []*L7Route
	}{
		{
			name:           "isolate listeners based on hostname intersection for different routes",
			routes:         routesHostnameIntersection,
			listenersMap:   listenerMapHostnameIntersection,
			expectedResult: expectedResultHostnameIntersection,
		},
		{
			name: "no isolation for listeners with same hostname, different ports",
			routes: []*L7Route{
				createL7RoutewithAcceptedHostnames(
					httpListenerRoute,
					acceptedHostnamesHTTP,
					[]gatewayv1.Hostname{"cafe.example.com"},
					helpers.GetPointer[gatewayv1.SectionName]("http"),
					80,
				),
			},
			listenersMap: map[string]hostPort{
				"http,test,gateway":           {hostname: "cafe.example.com", port: 80},
				"http-different,test,gateway": {hostname: "cafe.example.com", port: 8080},
			},
			expectedResult: map[string][]ParentRef{
				"hr_cafe": {
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: httpListenerRoute.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"http": {"cafe.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
				},
			},
		},
		{
			name: "no listener isolation for routes with no section name, attaches to all listeners",
			routes: []*L7Route{
				createL7RoutewithAcceptedHostnames(
					hrCoffeeRoute,
					acceptedHostnamesNoSectionName,
					routeHostnameNoSectionName,
					nil, // no section name
					80,
				),
				createL7RoutewithAcceptedHostnames(
					hrTeaRoute,
					acceptedHostnamesNoSectionName,
					routeHostnameNoSectionName,
					nil, // no section name
					80,
				),
				createL7RoutewithAcceptedHostnames(
					hrFlavorRoute,
					acceptedHostnamesNoSectionName,
					routeHostnameNoSectionName,
					nil, // no section name
					80,
				),
			},
			listenersMap: map[string]hostPort{
				"hr_coffee,test,gateway": {hostname: "coffee.example.com", port: 80},
				"hr_tea,test,gateway":    {hostname: "tea.example.com", port: 80},
				"hr_flavor,test,gateway": {hostname: "flavor.example.com", port: 80},
			},
			expectedResult: map[string][]ParentRef{
				"hr_coffee": {
					{
						Idx:     0,
						Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_coffee": {"coffee.example.com"},
								"hr_tea":    {"tea.example.com"},
								"hr_flavor": {"flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
				},
				"hr_tea": {
					{
						Idx:     0,
						Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_coffee": {"coffee.example.com"},
								"hr_tea":    {"tea.example.com"},
								"hr_flavor": {"flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
				},
				"hr_flavor": {
					{
						Idx:     0,
						Gateway: &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_coffee": {"coffee.example.com"},
								"hr_tea":    {"tea.example.com"},
								"hr_flavor": {"flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
				},
			},
		},
		{
			name: "no listener isolation for routes with same hostname, associated with different gateways",
			routes: []*L7Route{
				{
					Source: hrCoffeeRoute1,
					Spec: L7RouteSpec{
						Hostnames: routeHostname,
					},
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostNamesMultipleGateway,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(80),
							},
						},
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostNamesMultipleGateway,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(80),
							},
						},
					},
				},
				{
					Source: hrFlavorRoute1,
					Spec: L7RouteSpec{
						Hostnames: routeHostname,
					},
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostNamesMultipleGateway,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(80),
							},
						},
						{
							Idx:         0,
							Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("wildcard-example-com"),
							Attachment: &ParentRefAttachmentStatus{
								AcceptedHostnames: acceptedHostNamesMultipleGateway,
								Attached:          true,
								ListenerPort:      gatewayv1.PortNumber(80),
							},
						},
					},
				},
			},
			listenersMap: map[string]hostPort{
				"wildcard-example-com,test,gateway": {
					hostname: "*.example.com",
					port:     80,
					gwNsName: client.ObjectKeyFromObject(gw),
				},
				"wildcard-example-com,test,gateway1": {
					hostname: "*.example.com",
					port:     80,
					gwNsName: client.ObjectKeyFromObject(gw1),
				},
			},
			expectedResult: map[string][]ParentRef{
				"hr_coffee": {
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: hrCoffeeRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_cafe":   {"cafe.example.com", "flavor.example.com"},
								"hr_flavor": {"cafe.example.com", "flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: hrCoffeeRoute1.Spec.ParentRefs[1].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_cafe":   {"cafe.example.com", "flavor.example.com"},
								"hr_flavor": {"cafe.example.com", "flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
				},
				"hr_flavor": {
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: hrFlavorRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_cafe":   {"cafe.example.com", "flavor.example.com"},
								"hr_flavor": {"cafe.example.com", "flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
					{
						Idx:         0,
						Gateway:     &ParentRefGateway{NamespacedName: client.ObjectKeyFromObject(gw)},
						SectionName: hrFlavorRoute1.Spec.ParentRefs[0].SectionName,
						Attachment: &ParentRefAttachmentStatus{
							AcceptedHostnames: map[string][]string{
								"hr_cafe":   {"cafe.example.com", "flavor.example.com"},
								"hr_flavor": {"cafe.example.com", "flavor.example.com"},
							},
							ListenerPort: 80,
							Attached:     true,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			isolateL7RouteListeners(test.routes, test.listenersMap)

			result := map[string][]ParentRef{}
			for _, route := range test.routes {
				result[route.Source.GetName()] = route.ParentRefs
			}
			g.Expect(helpers.Diff(result, test.expectedResult)).To(BeEmpty())
		})
	}
}

func TestRemoveHostnames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		hostnames         []string
		removeHostnames   map[string]struct{}
		expectedHostnames []string
	}{
		{
			name:      "remove multiple hostnames",
			hostnames: []string{"foo.example.com", "bar.example.com", "bar.com", "*.wildcard.com"},
			removeHostnames: map[string]struct{}{
				"foo.example.com": {},
				"bar.example.com": {},
			},
			expectedHostnames: []string{"bar.com", "*.wildcard.com"},
		},
		{
			name:      "remove all hostnames",
			hostnames: []string{"foo.example.com", "bar.example.com", "bar.com", "*.wildcard.com"},
			removeHostnames: map[string]struct{}{
				"foo.example.com": {},
				"bar.example.com": {},
				"bar.com":         {},
				"*.wildcard.com":  {},
			},
			expectedHostnames: []string{},
		},
		{
			name:              "remove no hostnames",
			hostnames:         []string{"foo.example.com", "bar.example.com", "bar.com", "*.wildcard.com"},
			removeHostnames:   map[string]struct{}{},
			expectedHostnames: []string{"foo.example.com", "bar.example.com", "bar.com", "*.wildcard.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := removeHostnames(tt.hostnames, tt.removeHostnames)
			g.Expect(result).To(Equal(tt.expectedHostnames))
		})
	}
}

func TestBindRoutesToListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(func() {
		bindRoutesToListeners(nil, nil, nil, nil)
	}).ToNot(Panic())
}
