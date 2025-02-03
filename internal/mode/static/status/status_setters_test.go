package status

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/policiesfakes"
)

func TestNewNginxGatewayStatusSetter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		status, newStatus ngfAPI.NginxGatewayStatus
		expStatusSet      bool
	}{
		{
			name:         "NginxGateway has no status",
			expStatusSet: true,
			newStatus: ngfAPI.NginxGatewayStatus{
				Conditions: []metav1.Condition{{Message: "some condition"}},
			},
			status: ngfAPI.NginxGatewayStatus{},
		},
		{
			name:         "NginxGateway has old status",
			expStatusSet: true,
			newStatus: ngfAPI.NginxGatewayStatus{
				Conditions: []metav1.Condition{{Message: "new condition"}},
			},
			status: ngfAPI.NginxGatewayStatus{
				Conditions: []metav1.Condition{{Message: "old condition"}},
			},
		},
		{
			name:         "NginxGateway has same status",
			expStatusSet: false,
			newStatus: ngfAPI.NginxGatewayStatus{
				Conditions: []metav1.Condition{{Message: "same condition"}},
			},
			status: ngfAPI.NginxGatewayStatus{
				Conditions: []metav1.Condition{{Message: "same condition"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newNginxGatewayStatusSetter(test.newStatus)
			obj := &ngfAPI.NginxGateway{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.newStatus))
		})
	}
}

func TestNewGatewayStatusSetter(t *testing.T) {
	t.Parallel()
	expAddress := gatewayv1.GatewayStatusAddress{
		Type:  helpers.GetPointer(gatewayv1.IPAddressType),
		Value: "10.0.0.0",
	}

	tests := []struct {
		name              string
		status, newStatus gatewayv1.GatewayStatus
		expStatusSet      bool
	}{
		{
			name: "Gateway has no status",
			newStatus: gatewayv1.GatewayStatus{
				Conditions: []metav1.Condition{{Message: "new condition"}},
				Addresses:  []gatewayv1.GatewayStatusAddress{expAddress},
			},
			status:       gatewayv1.GatewayStatus{},
			expStatusSet: true,
		},
		{
			name: "Gateway has old status",
			newStatus: gatewayv1.GatewayStatus{
				Conditions: []metav1.Condition{{Message: "new condition"}},
				Addresses:  []gatewayv1.GatewayStatusAddress{expAddress},
			},
			status: gatewayv1.GatewayStatus{
				Conditions: []metav1.Condition{{Message: "old condition"}},
				Addresses:  []gatewayv1.GatewayStatusAddress{expAddress},
			},
			expStatusSet: true,
		},
		{
			name: "Gateway has same status",
			newStatus: gatewayv1.GatewayStatus{
				Conditions: []metav1.Condition{{Message: "same condition"}},
				Addresses:  []gatewayv1.GatewayStatusAddress{expAddress},
			},
			status: gatewayv1.GatewayStatus{
				Conditions: []metav1.Condition{{Message: "same condition"}},
				Addresses:  []gatewayv1.GatewayStatusAddress{expAddress},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newGatewayStatusSetter(test.newStatus)
			obj := &gatewayv1.Gateway{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.newStatus))
		})
	}
}

func TestNewHTTPRouteStatusSetter(t *testing.T) {
	t.Parallel()
	const (
		controllerName      = "controller"
		otherControllerName = "different"
	)

	tests := []struct {
		name                         string
		status, newStatus, expStatus gatewayv1.HTTPRouteStatus
		expStatusSet                 bool
	}{
		{
			name: "HTTPRoute has no status",
			newStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "HTTPRoute has old status",
			newStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			status: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "old condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "HTTPRoute has old status, keep other controller statuses",
			newStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			status: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(otherControllerName),
							Conditions:     []metav1.Condition{{Message: "some condition"}},
						},
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "old condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(otherControllerName),
							Conditions:     []metav1.Condition{{Message: "some condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "HTTPRoute has same status",
			newStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			status: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newHTTPRouteStatusSetter(test.newStatus, controllerName)
			obj := &gatewayv1.HTTPRoute{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.expStatus))
		})
	}
}

func TestNewGRPCRouteStatusSetter(t *testing.T) {
	t.Parallel()
	const (
		controllerName      = "controller"
		otherControllerName = "different"
	)

	tests := []struct {
		name                         string
		status, newStatus, expStatus gatewayv1.GRPCRouteStatus
		expStatusSet                 bool
	}{
		{
			name: "GRPCRoute has no status",
			newStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "GRPCRoute has old status",
			newStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			status: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "old condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "GRPCRoute has old status, keep other controller statuses",
			newStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			status: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(otherControllerName),
							Conditions:     []metav1.Condition{{Message: "some condition"}},
						},
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "old condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(otherControllerName),
							Conditions:     []metav1.Condition{{Message: "some condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "GRPCRoute has same status",
			newStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			status: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			expStatus: gatewayv1.GRPCRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newGRPCRouteStatusSetter(test.newStatus, controllerName)
			obj := &gatewayv1.GRPCRoute{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.expStatus))
		})
	}
}

func TestNewTLSRouteStatusSetter(t *testing.T) {
	t.Parallel()
	const (
		controllerName      = "controller"
		otherControllerName = "different"
	)

	tests := []struct {
		name                         string
		status, newStatus, expStatus v1alpha2.TLSRouteStatus
		expStatusSet                 bool
	}{
		{
			name: "TLSRoute has no status",
			newStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: v1alpha2.RouteStatus{
					Parents: []v1alpha2.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "TLSRoute has old status",
			newStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			status: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "old condition"}},
						},
					},
				},
			},
			expStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "TLSRoute has old status, keep other controller statuses",
			newStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
					},
				},
			},
			status: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(otherControllerName),
							Conditions:     []metav1.Condition{{Message: "some condition"}},
						},
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "old condition"}},
						},
					},
				},
			},
			expStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "new condition"}},
						},
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(otherControllerName),
							Conditions:     []metav1.Condition{{Message: "some condition"}},
						},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "TLSRoute has same status",
			newStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			status: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			expStatus: v1alpha2.TLSRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							ParentRef:      gatewayv1.ParentReference{},
							ControllerName: gatewayv1.GatewayController(controllerName),
							Conditions:     []metav1.Condition{{Message: "same condition"}},
						},
					},
				},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newTLSRouteStatusSetter(test.newStatus, controllerName)
			obj := &v1alpha2.TLSRoute{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.expStatus))
		})
	}
}

func TestNewGatewayClassStatusSetter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		status, newStatus gatewayv1.GatewayClassStatus
		expStatusSet      bool
	}{
		{
			name: "GatewayClass has no status",
			newStatus: gatewayv1.GatewayClassStatus{
				Conditions: []metav1.Condition{{Message: "new condition"}},
			},
			expStatusSet: true,
		},
		{
			name: "GatewayClass has old status",
			newStatus: gatewayv1.GatewayClassStatus{
				Conditions: []metav1.Condition{{Message: "new condition"}},
			},
			status: gatewayv1.GatewayClassStatus{
				Conditions: []metav1.Condition{{Message: "old condition"}},
			},
			expStatusSet: true,
		},
		{
			name: "GatewayClass has same status",
			newStatus: gatewayv1.GatewayClassStatus{
				Conditions: []metav1.Condition{{Message: "same condition"}},
			},
			status: gatewayv1.GatewayClassStatus{
				Conditions: []metav1.Condition{{Message: "same condition"}},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newGatewayClassStatusSetter(test.newStatus)
			obj := &gatewayv1.GatewayClass{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.newStatus))
		})
	}
}

func TestNewBackendTLSPolicyStatusSetter(t *testing.T) {
	t.Parallel()
	const (
		controllerName      = "controller"
		otherControllerName = "other-controller"
	)

	tests := []struct {
		name                         string
		status, newStatus, expStatus v1alpha2.PolicyStatus
		expStatusSet                 bool
	}{
		{
			name: "BackendTLSPolicy has no status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "BackendTLSPolicy has old status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "old condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "BackendTLSPolicy has old status and other controller status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "old condition"}},
					},
					{
						ControllerName: otherControllerName,
						Conditions:     []metav1.Condition{{Message: "some condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: otherControllerName,
						Conditions:     []metav1.Condition{{Message: "some condition"}},
					},
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "BackendTLSPolicy has same status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "same condition"}},
					},
				},
			},
			status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "same condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "same condition"}},
					},
				},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newBackendTLSPolicyStatusSetter(test.newStatus, controllerName)
			obj := &v1alpha3.BackendTLSPolicy{Status: test.status}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(obj.Status).To(Equal(test.expStatus))
		})
	}
}

func TestNewNGFPolicyStatusSetter(t *testing.T) {
	t.Parallel()
	const (
		controllerName      = "controller"
		otherControllerName = "other-controller"
	)

	tests := []struct {
		name                         string
		status, newStatus, expStatus v1alpha2.PolicyStatus
		expStatusSet                 bool
	}{
		{
			name: "Policy has no status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "Policy has old status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "old condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "Policy has old status and other controller status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "old condition"}},
					},
					{
						ControllerName: otherControllerName,
						Conditions:     []metav1.Condition{{Message: "some condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: otherControllerName,
						Conditions:     []metav1.Condition{{Message: "some condition"}},
					},
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "Policy has same status",
			newStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "same condition"}},
					},
				},
			},
			status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "same condition"}},
					},
				},
			},
			expStatus: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "same condition"}},
					},
				},
			},
			expStatusSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			setter := newNGFPolicyStatusSetter(test.newStatus, controllerName)
			obj := &policiesfakes.FakePolicy{
				GetPolicyStatusStub: func() v1alpha2.PolicyStatus {
					return test.status
				},
			}

			statusSet := setter(obj)

			g.Expect(statusSet).To(Equal(test.expStatusSet))

			if statusSet {
				g.Expect(obj.SetPolicyStatusArgsForCall(0)).To(Equal(test.expStatus))
			}
		})
	}
}

func TestGWStatusEqual(t *testing.T) {
	t.Parallel()
	getDefaultStatus := func() gatewayv1.GatewayStatus {
		return gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{
					Type:  helpers.GetPointer(gatewayv1.IPAddressType),
					Value: "10.0.0.0",
				},
				{
					Type:  helpers.GetPointer(gatewayv1.IPAddressType),
					Value: "11.0.0.0",
				},
			},
			Conditions: []metav1.Condition{
				{
					Type: "type", /* conditions are covered by another test*/
				},
			},
			Listeners: []gatewayv1.ListenerStatus{
				{
					Name: "listener1",
					SupportedKinds: []gatewayv1.RouteGroupKind{
						{
							Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
							Kind:  kinds.HTTPRoute,
						},
						{
							Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
							Kind:  "TCPRoute",
						},
					},
					AttachedRoutes: 1,
					Conditions: []metav1.Condition{
						{
							Type: "type", /* conditions are covered by another test*/
						},
					},
				},
				{
					Name: "listener2",
					SupportedKinds: []gatewayv1.RouteGroupKind{
						{
							Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
							Kind:  kinds.HTTPRoute,
						},
					},
					AttachedRoutes: 1,
					Conditions: []metav1.Condition{
						{
							Type: "type", /* conditions are covered by another test*/
						},
					},
				},
				{
					Name: "listener3",
					SupportedKinds: []gatewayv1.RouteGroupKind{
						{
							Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
							Kind:  kinds.HTTPRoute,
						},
					},
					AttachedRoutes: 1,
					Conditions: []metav1.Condition{
						{
							Type: "type", /* conditions are covered by another test*/
						},
					},
				},
			},
		}
	}

	getModifiedStatus := func(mod func(gatewayv1.GatewayStatus) gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
		return mod(getDefaultStatus())
	}

	tests := []struct {
		name       string
		prevStatus gatewayv1.GatewayStatus
		curStatus  gatewayv1.GatewayStatus
		expEqual   bool
	}{
		{
			name:       "different number of addresses",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Addresses = status.Addresses[:1]
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different address type",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Addresses[1].Type = helpers.GetPointer(gatewayv1.HostnameAddressType)
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different address value",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Addresses[0].Value = "12.0.0.0"
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different conditions",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Conditions[0].Type = "different"
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different number of listener statuses",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners = status.Listeners[:2]
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different listener status name",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners[2].Name = "different"
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different listener status attached routes",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners[1].AttachedRoutes++
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different listener status conditions",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners[0].Conditions[0].Type = "different"
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different listener status supported kinds (different number)",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners[0].SupportedKinds = status.Listeners[0].SupportedKinds[:1]
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different listener status supported kinds (different kind)",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners[1].SupportedKinds[0].Kind = "TCPRoute"
				return status
			}),
			expEqual: false,
		},
		{
			name:       "different listener status supported kinds (different group)",
			prevStatus: getDefaultStatus(),
			curStatus: getModifiedStatus(func(status gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
				status.Listeners[1].SupportedKinds[0].Group = helpers.GetPointer[gatewayv1.Group]("different")
				return status
			}),
			expEqual: false,
		},
		{
			name:       "equal",
			prevStatus: getDefaultStatus(),
			curStatus:  getDefaultStatus(),
			expEqual:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			equal := gwStatusEqual(test.prevStatus, test.curStatus)
			g.Expect(equal).To(Equal(test.expEqual))
		})
	}
}

func TestHRStatusEqual(t *testing.T) {
	t.Parallel()
	testConds := []metav1.Condition{
		{
			Type: "type", /* conditions are covered by another test*/
		},
	}

	previousStatus := gatewayv1.HTTPRouteStatus{
		RouteStatus: gatewayv1.RouteStatus{
			Parents: []gatewayv1.RouteParentStatus{
				{
					ParentRef: gatewayv1.ParentReference{
						Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
						Name:        "our-parent",
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("section1"),
					},
					ControllerName: "ours",
					Conditions:     testConds,
				},
				{
					ParentRef: gatewayv1.ParentReference{
						Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
						Name:        "not-our-parent",
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("section1"),
					},
					ControllerName: "not-ours",
					Conditions:     testConds,
				},
				{
					ParentRef: gatewayv1.ParentReference{
						Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
						Name:        "our-parent",
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("section2"),
					},
					ControllerName: "ours",
					Conditions:     testConds,
				},
				{
					ParentRef: gatewayv1.ParentReference{
						Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
						Name:        "not-our-parent",
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("section2"),
					},
					ControllerName: "not-ours",
					Conditions:     testConds,
				},
			},
		},
	}

	getDefaultStatus := func() gatewayv1.HTTPRouteStatus {
		return gatewayv1.HTTPRouteStatus{
			RouteStatus: gatewayv1.RouteStatus{
				Parents: []gatewayv1.RouteParentStatus{
					{
						ParentRef: gatewayv1.ParentReference{
							Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
							Name:        "our-parent",
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("section1"),
						},
						ControllerName: "ours",
						Conditions:     testConds,
					},
					{
						ParentRef: gatewayv1.ParentReference{
							Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
							Name:        "our-parent",
							SectionName: helpers.GetPointer[gatewayv1.SectionName]("section2"),
						},
						ControllerName: "ours",
						Conditions:     testConds,
					},
				},
			},
		}
	}

	newParentStatus := gatewayv1.RouteParentStatus{
		ParentRef: gatewayv1.ParentReference{
			Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
			Name:        "our-parent",
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("section3"),
		},
		ControllerName: "ours",
		Conditions:     testConds,
	}

	getModifiedStatus := func(
		mod func(status gatewayv1.HTTPRouteStatus) gatewayv1.HTTPRouteStatus,
	) gatewayv1.HTTPRouteStatus {
		return mod(getDefaultStatus())
	}

	tests := []struct {
		name       string
		prevStatus gatewayv1.HTTPRouteStatus
		curStatus  gatewayv1.HTTPRouteStatus
		expEqual   bool
	}{
		{
			name:       "stale status",
			prevStatus: previousStatus,
			curStatus: getModifiedStatus(func(status gatewayv1.HTTPRouteStatus) gatewayv1.HTTPRouteStatus {
				// remove last parent status
				status.Parents = status.Parents[:1]
				return status
			}),
			expEqual: false,
		},
		{
			name:       "new status",
			prevStatus: previousStatus,
			curStatus: getModifiedStatus(func(status gatewayv1.HTTPRouteStatus) gatewayv1.HTTPRouteStatus {
				// add another parent status
				status.Parents = append(status.Parents, newParentStatus)
				return status
			}),
			expEqual: false,
		},
		{
			name:       "equal",
			prevStatus: previousStatus,
			curStatus:  getDefaultStatus(),
			expEqual:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			equal := routeStatusEqual("ours", test.prevStatus.Parents, test.curStatus.Parents)
			g.Expect(equal).To(Equal(test.expEqual))
		})
	}
}

func TestRouteParentStatusEqual(t *testing.T) {
	t.Parallel()
	getDefaultStatus := func() gatewayv1.RouteParentStatus {
		return gatewayv1.RouteParentStatus{
			ParentRef: gatewayv1.ParentReference{
				Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
				Name:        "parent",
				SectionName: helpers.GetPointer[gatewayv1.SectionName]("section"),
			},
			ControllerName: "controller",
			Conditions: []metav1.Condition{
				{
					Type: "type", /* conditions are covered by another test*/
				},
			},
		}
	}

	getModifiedStatus := func(
		mod func(gatewayv1.RouteParentStatus) gatewayv1.RouteParentStatus,
	) gatewayv1.RouteParentStatus {
		return mod(getDefaultStatus())
	}

	tests := []struct {
		name     string
		p1       gatewayv1.RouteParentStatus
		p2       gatewayv1.RouteParentStatus
		expEqual bool
	}{
		{
			name: "different controller name",
			p1:   getDefaultStatus(),
			p2: getModifiedStatus(func(status gatewayv1.RouteParentStatus) gatewayv1.RouteParentStatus {
				status.ControllerName = "different"
				return status
			}),
			expEqual: false,
		},
		{
			name: "different parentRef name",
			p1:   getDefaultStatus(),
			p2: getModifiedStatus(func(status gatewayv1.RouteParentStatus) gatewayv1.RouteParentStatus {
				status.ParentRef.Name = "different"
				return status
			}),
			expEqual: false,
		},
		{
			name: "different parentRef namespace",
			p1:   getDefaultStatus(),
			p2: getModifiedStatus(func(status gatewayv1.RouteParentStatus) gatewayv1.RouteParentStatus {
				status.ParentRef.Namespace = helpers.GetPointer[gatewayv1.Namespace]("different")
				return status
			}),
			expEqual: false,
		},
		{
			name: "different parentRef section name",
			p1:   getDefaultStatus(),
			p2: getModifiedStatus(func(status gatewayv1.RouteParentStatus) gatewayv1.RouteParentStatus {
				status.ParentRef.SectionName = helpers.GetPointer[gatewayv1.SectionName]("different")
				return status
			}),
			expEqual: false,
		},
		{
			name: "different conditions",
			p1:   getDefaultStatus(),
			p2: getModifiedStatus(func(status gatewayv1.RouteParentStatus) gatewayv1.RouteParentStatus {
				status.Conditions[0].Type = "different"
				return status
			}),
			expEqual: false,
		},
		{
			name:     "equal",
			p1:       getDefaultStatus(),
			p2:       getDefaultStatus(),
			expEqual: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			equal := routeParentStatusEqual(test.p1, test.p2)
			g.Expect(equal).To(Equal(test.expEqual))
		})
	}
}

func TestPolicyStatusEqual(t *testing.T) {
	t.Parallel()
	getPolicyStatus := func() v1alpha2.PolicyStatus {
		return v1alpha2.PolicyStatus{
			Ancestors: []v1alpha2.PolicyAncestorStatus{
				{
					AncestorRef: gatewayv1.ParentReference{
						Namespace: helpers.GetPointer[gatewayv1.Namespace]("ns1"),
						Name:      "ancestor1",
						Group:     helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
						Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
					},
					ControllerName: "ctlr1",
					Conditions:     []metav1.Condition{{Type: "otherType", Status: "otherStatus"}},
				},
			},
		}
	}

	type modFunc func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus

	getModifiedPolicyStatus := func(mod modFunc) v1alpha2.PolicyStatus {
		return mod(getPolicyStatus())
	}

	prevMultiple := getPolicyStatus()
	prevMultiple.Ancestors = append(
		prevMultiple.Ancestors,
		getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
			ns := "ns2"
			s.Ancestors[0].AncestorRef.Name = "ancestor2"
			s.Ancestors[0].AncestorRef.Namespace = (*gatewayv1.Namespace)(&ns)
			s.Ancestors[0].ControllerName = "ctlr2"
			return s
		}).Ancestors...)

	currMultiple := getPolicyStatus()
	currMultiple.Ancestors = append(
		currMultiple.Ancestors,
		getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
			ns := "ns3"
			s.Ancestors[0].AncestorRef.Name = "ancestor3"
			s.Ancestors[0].AncestorRef.Namespace = (*gatewayv1.Namespace)(&ns)
			s.Ancestors[0].ControllerName = "ctlr3"
			return s
		}).Ancestors...)

	tests := []struct {
		name           string
		controllerName string
		previous       v1alpha2.PolicyStatus
		current        v1alpha2.PolicyStatus
		expEqual       bool
	}{
		{
			name:           "status equal",
			previous:       getPolicyStatus(),
			current:        getPolicyStatus(),
			controllerName: "ctlr1",
			expEqual:       true,
		},
		{
			name:     "status not equal, different ancestor name",
			previous: getPolicyStatus(),
			current: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				s.Ancestors[0].AncestorRef.Name = "diff"
				return s
			}),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name:     "status not equal, different ancestor namespace",
			previous: getPolicyStatus(),
			current: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				ns := "diff"
				s.Ancestors[0].AncestorRef.Namespace = (*gatewayv1.Namespace)(&ns)
				return s
			}),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name:     "status not equal, different ancestor kind",
			previous: getPolicyStatus(),
			current: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				s.Ancestors[0].AncestorRef.Kind = helpers.GetPointer[gatewayv1.Kind]("diff")
				return s
			}),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name:     "status not equal, different ancestor group",
			previous: getPolicyStatus(),
			current: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				s.Ancestors[0].AncestorRef.Group = helpers.GetPointer[gatewayv1.Group]("diff")
				return s
			}),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name:     "status not equal, different controller name on current",
			previous: getPolicyStatus(),
			current: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				s.Ancestors[0].ControllerName = "diff"
				return s
			}),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name:     "status not equal, different conds",
			previous: getPolicyStatus(),
			current: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				s.Ancestors[0].Conditions = nil
				return s
			}),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name: "status not equal, different controller name on previous",
			previous: getModifiedPolicyStatus(func(s v1alpha2.PolicyStatus) v1alpha2.PolicyStatus {
				s.Ancestors[0].ControllerName = "diff"
				return s
			}),
			current:        getPolicyStatus(),
			controllerName: "ctlr1",
			expEqual:       false,
		},
		{
			name:           "status not equal, different controller ancestor changed",
			previous:       prevMultiple,
			current:        currMultiple,
			controllerName: "ctlr1",
			expEqual:       false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			equal := policyStatusEqual(test.controllerName, test.previous, test.current)
			g.Expect(equal).To(Equal(test.expEqual))
		})
	}
}

func TestNewSnippetsFilterStatusSetter(t *testing.T) {
	const (
		controllerName      = "controller"
		otherControllerName = "other-controller"
	)
	tests := []struct {
		name                         string
		status, expStatus, newStatus ngfAPI.SnippetsFilterStatus
		expStatusSet                 bool
	}{
		{
			name: "SnippetsFilter has no status",
			newStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "new condition"}},
						ControllerName: controllerName,
					},
				},
			},
			expStatusSet: true,
			expStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "new condition"}},
						ControllerName: controllerName,
					},
				},
			},
		},
		{
			name: "SnippetsFilter has old status",
			status: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "old condition"}},
						ControllerName: controllerName,
					},
				},
			},
			newStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "new condition"}},
						ControllerName: controllerName,
					},
				},
			},
			expStatusSet: true,
			expStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "new condition"}},
						ControllerName: controllerName,
					},
				},
			},
		},
		{
			name: "SnippetsFilter has old status and other controller status",
			newStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "new condition"}},
						ControllerName: controllerName,
					},
				},
			},
			status: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						ControllerName: otherControllerName,
						Conditions:     []metav1.Condition{{Message: "some condition"}},
					},
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "old condition"}},
					},
				},
			},
			expStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						ControllerName: otherControllerName,
						Conditions:     []metav1.Condition{{Message: "some condition"}},
					},
					{
						ControllerName: controllerName,
						Conditions:     []metav1.Condition{{Message: "new condition"}},
					},
				},
			},
			expStatusSet: true,
		},
		{
			name: "SnippetsFilter has same status",
			status: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "same condition"}},
						ControllerName: controllerName,
					},
				},
			},
			newStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "same condition"}},
						ControllerName: controllerName,
					},
				},
			},
			expStatusSet: false,
			expStatus: ngfAPI.SnippetsFilterStatus{
				Controllers: []ngfAPI.ControllerStatus{
					{
						Conditions:     []metav1.Condition{{Message: "same condition"}},
						ControllerName: controllerName,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			setter := newSnippetsFilterStatusSetter(test.newStatus, controllerName)
			sf := &ngfAPI.SnippetsFilter{Status: test.status}

			statusSet := setter(sf)

			g.Expect(statusSet).To(Equal(test.expStatusSet))
			g.Expect(sf.Status).To(Equal(test.expStatus))
		})
	}
}
