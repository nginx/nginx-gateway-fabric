package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func createValidValidator() *validationfakes.FakeGenericValidator {
	v := &validationfakes.FakeGenericValidator{}
	v.ValidateEscapedStringNoVarExpansionReturns(nil)
	v.ValidateEndpointReturns(nil)
	v.ValidateServiceNameReturns(nil)
	v.ValidateNginxDurationReturns(nil)

	return v
}

func createInvalidValidator() *validationfakes.FakeGenericValidator {
	v := &validationfakes.FakeGenericValidator{}
	v.ValidateEscapedStringNoVarExpansionReturns(errors.New("error"))
	v.ValidateEndpointReturns(errors.New("error"))
	v.ValidateServiceNameReturns(errors.New("error"))
	v.ValidateNginxDurationReturns(errors.New("error"))

	return v
}

func TestBuildEffectiveNginxProxy(t *testing.T) {
	t.Parallel()

	newTestNginxProxy := func(
		ipFam ngfAPIv1alpha2.IPFamilyType,
		disableFeats []ngfAPIv1alpha2.DisableTelemetryFeature,
		interval ngfAPIv1alpha1.Duration,
		batchSize int32,
		batchCount int32,
		endpoint string,
		serviceName string,
		spanAttr ngfAPIv1alpha1.SpanAttribute,
		mode ngfAPIv1alpha2.RewriteClientIPModeType,
		trustedAddr []ngfAPIv1alpha2.RewriteClientIPAddress,
		logLevel ngfAPIv1alpha2.NginxErrorLogLevel,
		setIP bool,
		disableHTTP bool,
		nginxDebug bool,
	) *ngfAPIv1alpha2.NginxProxy {
		return &ngfAPIv1alpha2.NginxProxy{
			Spec: ngfAPIv1alpha2.NginxProxySpec{
				IPFamily: &ipFam,
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					DisabledFeatures: disableFeats,
					Exporter: &ngfAPIv1alpha2.TelemetryExporter{
						Interval:   &interval,
						BatchSize:  &batchSize,
						BatchCount: &batchCount,
						Endpoint:   &endpoint,
					},
					ServiceName:    &serviceName,
					SpanAttributes: []ngfAPIv1alpha1.SpanAttribute{spanAttr},
				},
				RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
					Mode:             &mode,
					SetIPRecursively: &setIP,
					TrustedAddresses: trustedAddr,
				},
				Logging: &ngfAPIv1alpha2.NginxLogging{
					ErrorLevel: &logLevel,
				},
				DisableHTTP2: &disableHTTP,
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Container: ngfAPIv1alpha2.ContainerSpec{
							Debug: &nginxDebug,
						},
					},
				},
			},
		}
	}

	getNginxProxy := func() *ngfAPIv1alpha2.NginxProxy {
		return newTestNginxProxy(
			ngfAPIv1alpha2.Dual,
			[]ngfAPIv1alpha2.DisableTelemetryFeature{ngfAPIv1alpha2.DisableTracing},
			"10s",
			10,
			5,
			"endpoint:1234",
			"my-service",
			ngfAPIv1alpha1.SpanAttribute{Key: "key", Value: "val"},
			ngfAPIv1alpha2.RewriteClientIPModeXForwardedFor,
			[]ngfAPIv1alpha2.RewriteClientIPAddress{
				{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "10.0.0.1"},
			},
			ngfAPIv1alpha2.NginxLogLevelAlert,
			true,
			false,
			false,
		)
	}

	getNginxProxyAllFieldsSetDifferently := func() *ngfAPIv1alpha2.NginxProxy {
		return newTestNginxProxy(
			ngfAPIv1alpha2.IPv6,
			[]ngfAPIv1alpha2.DisableTelemetryFeature{},
			"5s",
			8,
			2,
			"diff-endpoint:1234",
			"diff-service",
			ngfAPIv1alpha1.SpanAttribute{Key: "diff-key", Value: "diff-val"},
			ngfAPIv1alpha2.RewriteClientIPModeXForwardedFor,
			[]ngfAPIv1alpha2.RewriteClientIPAddress{
				{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "10.0.0.1/24"},
			},
			ngfAPIv1alpha2.NginxLogLevelError,
			false,
			true,
			true,
		)
	}

	getExpSpec := func() *EffectiveNginxProxy {
		enp := EffectiveNginxProxy(getNginxProxy().Spec)
		return &enp
	}

	getModifiedExpSpec := func(mod func(*ngfAPIv1alpha2.NginxProxy) *ngfAPIv1alpha2.NginxProxy) *EffectiveNginxProxy {
		enp := EffectiveNginxProxy(mod(getNginxProxy()).Spec)
		return &enp
	}

	tests := []struct {
		gcNp *NginxProxy
		gwNp *NginxProxy
		exp  *EffectiveNginxProxy
		name string
	}{
		{
			name: "both gateway class and gateway nginx proxies are nil",
			gcNp: nil,
			gwNp: nil,
			exp:  nil,
		},
		{
			name: "nil gateway class nginx proxy",
			gcNp: nil,
			gwNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			exp:  getExpSpec(),
		},
		{
			name: "nil gateway class nginx proxy; invalid gateway nginx proxy",
			gcNp: nil,
			gwNp: &NginxProxy{Valid: false, Source: getNginxProxy()},
			exp:  nil,
		},
		{
			name: "nil gateway class nginx proxy; nil gateway nginx proxy source",
			gcNp: nil,
			gwNp: &NginxProxy{Valid: true, Source: nil},
			exp:  nil,
		},
		{
			name: "invalid gateway class nginx proxy",
			gcNp: &NginxProxy{Valid: false},
			gwNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			exp:  getExpSpec(),
		},
		{
			name: "nil gateway class nginx proxy source",
			gcNp: &NginxProxy{Valid: true, Source: nil},
			gwNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			exp:  getExpSpec(),
		},
		{
			name: "nil gateway nginx proxy",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: nil,
			exp:  getExpSpec(),
		},
		{
			name: "invalid gateway nginx proxy",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{Valid: false},
			exp:  getExpSpec(),
		},
		{
			name: "nil gateway nginx proxy source",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{Valid: true, Source: nil},
			exp:  getExpSpec(),
		},
		{
			name: "both have all fields set; gateway values should win",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{Valid: true, Source: getNginxProxyAllFieldsSetDifferently()},
			exp: getModifiedExpSpec(func(_ *ngfAPIv1alpha2.NginxProxy) *ngfAPIv1alpha2.NginxProxy {
				return getNginxProxyAllFieldsSetDifferently()
			}),
		},
		{
			name: "gateway nginx proxy overrides nginx error log level",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{
				Valid: true,
				Source: &ngfAPIv1alpha2.NginxProxy{
					Spec: ngfAPIv1alpha2.NginxProxySpec{
						Logging: &ngfAPIv1alpha2.NginxLogging{
							ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
						},
					},
				},
			},
			exp: getModifiedExpSpec(func(np *ngfAPIv1alpha2.NginxProxy) *ngfAPIv1alpha2.NginxProxy {
				np.Spec.Logging.ErrorLevel = helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug)
				return np
			}),
		},
		{
			name: "gateway nginx proxy overrides select telemetry values",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{
				Valid: true,
				Source: &ngfAPIv1alpha2.NginxProxy{
					Spec: ngfAPIv1alpha2.NginxProxySpec{
						Telemetry: &ngfAPIv1alpha2.Telemetry{
							ServiceName: helpers.GetPointer("new-service-name"),
							Exporter: &ngfAPIv1alpha2.TelemetryExporter{
								BatchSize: helpers.GetPointer[int32](20),
								Endpoint:  helpers.GetPointer("new-endpoint"),
							},
						},
					},
				},
			},
			exp: getModifiedExpSpec(func(np *ngfAPIv1alpha2.NginxProxy) *ngfAPIv1alpha2.NginxProxy {
				np.Spec.Telemetry.ServiceName = helpers.GetPointer("new-service-name")
				np.Spec.Telemetry.Exporter.Endpoint = helpers.GetPointer("new-endpoint")
				np.Spec.Telemetry.Exporter.BatchSize = helpers.GetPointer[int32](20)
				return np
			}),
		},
		{
			name: "gateway nginx proxy overrides select rewrite client IP values",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{
				Valid: true,
				Source: &ngfAPIv1alpha2.NginxProxy{
					Spec: ngfAPIv1alpha2.NginxProxySpec{
						RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
							Mode:             helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
							SetIPRecursively: helpers.GetPointer(false),
						},
					},
				},
			},
			exp: getModifiedExpSpec(func(np *ngfAPIv1alpha2.NginxProxy) *ngfAPIv1alpha2.NginxProxy {
				np.Spec.RewriteClientIP.Mode = helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol)
				np.Spec.RewriteClientIP.SetIPRecursively = helpers.GetPointer(false)
				return np
			}),
		},
		{
			name: "gateway nginx proxy unsets slices values",
			gcNp: &NginxProxy{Valid: true, Source: getNginxProxy()},
			gwNp: &NginxProxy{
				Valid: true,
				Source: &ngfAPIv1alpha2.NginxProxy{
					Spec: ngfAPIv1alpha2.NginxProxySpec{
						Telemetry: &ngfAPIv1alpha2.Telemetry{
							DisabledFeatures: []ngfAPIv1alpha2.DisableTelemetryFeature{},
							SpanAttributes:   []ngfAPIv1alpha1.SpanAttribute{},
						},
						RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
							TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{},
						},
					},
				},
			},
			exp: getModifiedExpSpec(func(np *ngfAPIv1alpha2.NginxProxy) *ngfAPIv1alpha2.NginxProxy {
				np.Spec.RewriteClientIP.TrustedAddresses = []ngfAPIv1alpha2.RewriteClientIPAddress{}
				np.Spec.Telemetry.DisabledFeatures = []ngfAPIv1alpha2.DisableTelemetryFeature{}
				np.Spec.Telemetry.SpanAttributes = []ngfAPIv1alpha1.SpanAttribute{}
				return np
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			enp := buildEffectiveNginxProxy(test.gcNp, test.gwNp)
			g.Expect(enp).To(Equal(test.exp))
		})
	}
}

func TestTelemetryEnabledForNginxProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ep      *EffectiveNginxProxy
		name    string
		enabled bool
	}{
		{
			name: "telemetry struct is nil",
			ep: &EffectiveNginxProxy{
				Telemetry: nil,
			},
			enabled: false,
		},
		{
			name: "telemetry exporter is nil",
			ep: &EffectiveNginxProxy{
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					Exporter: nil,
				},
			},
			enabled: false,
		},
		{
			name: "tracing is disabled",
			ep: &EffectiveNginxProxy{
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					DisabledFeatures: []ngfAPIv1alpha2.DisableTelemetryFeature{
						ngfAPIv1alpha2.DisableTracing,
					},
					Exporter: &ngfAPIv1alpha2.TelemetryExporter{
						Endpoint: helpers.GetPointer("new-endpoint"),
					},
				},
			},
			enabled: false,
		},
		{
			name: "exporter endpoint is nil",
			ep: &EffectiveNginxProxy{
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					Exporter: &ngfAPIv1alpha2.TelemetryExporter{
						Endpoint: nil,
					},
				},
			},
			enabled: false,
		},
		{
			name: "normal case; enabled",
			ep: &EffectiveNginxProxy{
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					Exporter: &ngfAPIv1alpha2.TelemetryExporter{
						Endpoint: helpers.GetPointer("new-endpoint"),
					},
				},
			},
			enabled: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			enabled := telemetryEnabledForNginxProxy(test.ep)
			g.Expect(enabled).To(Equal(test.enabled))
		})
	}
}

func TestMetricsEnabledForNginxProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ep      *EffectiveNginxProxy
		port    *int32
		name    string
		enabled bool
	}{
		{
			name:    "NginxProxy is nil",
			port:    nil,
			enabled: true,
		},
		{
			name: "metrics struct is nil",
			ep: &EffectiveNginxProxy{
				Metrics: nil,
			},
			port:    nil,
			enabled: true,
		},
		{
			name: "metrics disable is nil",
			ep: &EffectiveNginxProxy{
				Metrics: &ngfAPIv1alpha2.Metrics{
					Disable: nil,
				},
			},
			port:    nil,
			enabled: true,
		},
		{
			name: "metrics is disabled",
			ep: &EffectiveNginxProxy{
				Metrics: &ngfAPIv1alpha2.Metrics{
					Disable: helpers.GetPointer(true),
				},
			},
			port:    nil,
			enabled: false,
		},
		{
			name: "metrics is enabled with no port specified",
			ep: &EffectiveNginxProxy{
				Metrics: &ngfAPIv1alpha2.Metrics{
					Disable: helpers.GetPointer(false),
				},
			},
			port:    nil,
			enabled: true,
		},
		{
			name: "metrics is enabled with port specified",
			ep: &EffectiveNginxProxy{
				Metrics: &ngfAPIv1alpha2.Metrics{
					Disable: helpers.GetPointer(false),
					Port:    helpers.GetPointer[int32](8080),
				},
			},
			port:    helpers.GetPointer[int32](8080),
			enabled: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			port, enabled := MetricsEnabledForNginxProxy(test.ep)
			g.Expect(port).To(Equal(test.port))
			g.Expect(enabled).To(Equal(test.enabled))
		})
	}
}

func TestProcessNginxProxies(t *testing.T) {
	t.Parallel()

	gatewayClassNpName := types.NamespacedName{Namespace: "gc-ns", Name: "gc-np"}
	gatewayNpName := types.NamespacedName{Namespace: "gw-ns", Name: "gw-np"}
	unreferencedNpName := types.NamespacedName{Namespace: "test", Name: "unref"}

	getTestNp := func(nsname types.NamespacedName) *ngfAPIv1alpha2.NginxProxy {
		return &ngfAPIv1alpha2.NginxProxy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nsname.Namespace,
				Name:      nsname.Name,
			},
			Spec: ngfAPIv1alpha2.NginxProxySpec{
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					ServiceName: helpers.GetPointer("service-name"),
				},
			},
		}
	}

	gateway := map[types.NamespacedName]*v1.Gateway{
		gatewayNpName: {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "gw-ns",
			},
			Spec: v1.GatewaySpec{
				Infrastructure: &v1.GatewayInfrastructure{
					ParametersRef: &v1.LocalParametersReference{
						Group: ngfAPIv1alpha2.GroupName,
						Kind:  kinds.NginxProxy,
						Name:  gatewayNpName.Name,
					},
				},
			},
		},
	}

	gatewayClass := &v1.GatewayClass{
		Spec: v1.GatewayClassSpec{
			ParametersRef: &v1.ParametersReference{
				Group:     ngfAPIv1alpha2.GroupName,
				Kind:      kinds.NginxProxy,
				Name:      gatewayClassNpName.Name,
				Namespace: helpers.GetPointer[v1.Namespace]("gc-ns"),
			},
		},
	}

	gatewayClassRefMissingNs := &v1.GatewayClass{
		Spec: v1.GatewayClassSpec{
			ParametersRef: &v1.ParametersReference{
				Group: ngfAPIv1alpha2.GroupName,
				Kind:  kinds.NginxProxy,
				Name:  gatewayClassNpName.Name,
			},
		},
	}

	getNpMap := func() map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy {
		return map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
			gatewayClassNpName: getTestNp(gatewayClassNpName),
			gatewayNpName:      getTestNp(gatewayNpName),
			unreferencedNpName: getTestNp(unreferencedNpName),
		}
	}

	getExpResult := func(valid bool) map[types.NamespacedName]*NginxProxy {
		var errMsgs field.ErrorList
		if !valid {
			errMsgs = field.ErrorList{
				field.Invalid(field.NewPath("spec.telemetry.serviceName"), "service-name", "error"),
			}
		}

		return map[types.NamespacedName]*NginxProxy{
			gatewayNpName: {
				Valid:   valid,
				ErrMsgs: errMsgs,
				Source:  getTestNp(gatewayNpName),
			},
			gatewayClassNpName: {
				Valid:   valid,
				ErrMsgs: errMsgs,
				Source:  getTestNp(gatewayClassNpName),
			},
		}
	}

	tests := []struct {
		validator validation.GenericValidator
		nps       map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy
		gc        *v1.GatewayClass
		gws       map[types.NamespacedName]*v1.Gateway
		expResult map[types.NamespacedName]*NginxProxy
		name      string
	}{
		{
			name:      "no nginx proxies",
			nps:       nil,
			gc:        gatewayClass,
			gws:       gateway,
			validator: createValidValidator(),
			expResult: map[types.NamespacedName]*NginxProxy{gatewayNpName: nil},
		},
		{
			name: "gateway class param ref is missing namespace",
			nps: map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
				gatewayClassNpName: getTestNp(gatewayClassNpName),
				gatewayNpName:      getTestNp(gatewayNpName),
			},
			gc:        gatewayClassRefMissingNs,
			gws:       gateway,
			validator: createValidValidator(),
			expResult: map[types.NamespacedName]*NginxProxy{
				gatewayNpName: {
					Valid:  true,
					Source: getTestNp(gatewayNpName),
				},
			},
		},
		{
			name:      "normal case; both nginx proxies are valid",
			nps:       getNpMap(),
			gc:        gatewayClass,
			gws:       gateway,
			validator: createValidValidator(),
			expResult: getExpResult(true),
		},
		{
			name:      "normal case; both nginx proxies are invalid",
			nps:       getNpMap(),
			gc:        gatewayClass,
			gws:       gateway,
			validator: createInvalidValidator(),
			expResult: getExpResult(false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := processNginxProxies(
				test.nps,
				test.validator,
				test.gc,
				test.gws,
			)

			g.Expect(helpers.Diff(test.expResult, result)).To(BeEmpty())
		})
	}
}

func TestGCReferencesAnyNginxProxy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		gc     *v1.GatewayClass
		name   string
		expRes bool
	}{
		{
			gc:     nil,
			expRes: false,
			name:   "nil gatewayclass",
		},
		{
			gc: &v1.GatewayClass{
				Spec: v1.GatewayClassSpec{},
			},
			expRes: false,
			name:   "nil paramsRef",
		},
		{
			gc: &v1.GatewayClass{
				Spec: v1.GatewayClassSpec{
					ParametersRef: &v1.ParametersReference{
						Group: v1.Group("wrong-group"),
						Kind:  v1.Kind(kinds.NginxProxy),
						Name:  "wrong-group",
					},
				},
			},
			expRes: false,
			name:   "wrong group name",
		},
		{
			gc: &v1.GatewayClass{
				Spec: v1.GatewayClassSpec{
					ParametersRef: &v1.ParametersReference{
						Group: ngfAPIv1alpha2.GroupName,
						Kind:  v1.Kind("WrongKind"),
						Name:  "wrong-kind",
					},
				},
			},
			expRes: false,
			name:   "wrong kind",
		},
		{
			gc: &v1.GatewayClass{
				Spec: v1.GatewayClassSpec{
					ParametersRef: &v1.ParametersReference{
						Group: ngfAPIv1alpha2.GroupName,
						Kind:  v1.Kind(kinds.NginxProxy),
						Name:  "nginx-proxy",
					},
				},
			},
			expRes: true,
			name:   "references an NginxProxy",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(gcReferencesAnyNginxProxy(test.gc)).To(Equal(test.expRes))
		})
	}
}

func TestGWReferencesAnyNginxProxy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		gw     *v1.Gateway
		name   string
		expRes bool
	}{
		{
			gw:     nil,
			expRes: false,
			name:   "nil gateway",
		},
		{
			gw: &v1.Gateway{
				Spec: v1.GatewaySpec{},
			},
			expRes: false,
			name:   "nil infrastructure",
		},
		{
			gw: &v1.Gateway{
				Spec: v1.GatewaySpec{
					Infrastructure: &v1.GatewayInfrastructure{},
				},
			},
			expRes: false,
			name:   "nil parametersRef",
		},
		{
			gw: &v1.Gateway{
				Spec: v1.GatewaySpec{
					Infrastructure: &v1.GatewayInfrastructure{
						ParametersRef: &v1.LocalParametersReference{
							Group: v1.Group("wrong-group"),
							Kind:  v1.Kind(kinds.NginxProxy),
							Name:  "wrong-group",
						},
					},
				},
			},
			expRes: false,
			name:   "wrong group name",
		},
		{
			gw: &v1.Gateway{
				Spec: v1.GatewaySpec{
					Infrastructure: &v1.GatewayInfrastructure{
						ParametersRef: &v1.LocalParametersReference{
							Group: v1.Group(ngfAPIv1alpha2.GroupName),
							Kind:  v1.Kind("wrong-kind"),
							Name:  "wrong-kind",
						},
					},
				},
			},
			expRes: false,
			name:   "wrong kind",
		},
		{
			gw: &v1.Gateway{
				Spec: v1.GatewaySpec{
					Infrastructure: &v1.GatewayInfrastructure{
						ParametersRef: &v1.LocalParametersReference{
							Group: v1.Group(ngfAPIv1alpha2.GroupName),
							Kind:  v1.Kind(kinds.NginxProxy),
							Name:  "normal",
						},
					},
				},
			},
			expRes: true,
			name:   "references an NginxProxy",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(gwReferencesAnyNginxProxy(test.gw)).To(Equal(test.expRes))
		})
	}
}

func TestValidateNginxProxy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		np              *ngfAPIv1alpha2.NginxProxy
		validator       *validationfakes.FakeGenericValidator
		name            string
		expErrSubstring string
		expectErrCount  int
	}{
		{
			name:      "valid nginxproxy",
			validator: createValidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{
						ServiceName: helpers.GetPointer("my-svc"),
						Exporter: &ngfAPIv1alpha2.TelemetryExporter{
							Interval: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5ms"),
							Endpoint: helpers.GetPointer("my-endpoint"),
						},
						SpanAttributes: []ngfAPIv1alpha1.SpanAttribute{
							{Key: "key", Value: "value"},
						},
					},
					IPFamily: helpers.GetPointer[ngfAPIv1alpha2.IPFamilyType](ngfAPIv1alpha2.Dual),
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "2001:db8:a0b:12f0::1/128",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPIPAddressType,
								Value: "1.1.1.1",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPHostnameAddressType,
								Value: "example.com",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 0,
		},
		{
			name:      "invalid serviceName",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{
						ServiceName: helpers.GetPointer("my-svc"), // any value is invalid by the validator
					},
				},
			},
			expErrSubstring: "telemetry.serviceName",
			expectErrCount:  1,
		},
		{
			name:      "invalid endpoint",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{
						Exporter: &ngfAPIv1alpha2.TelemetryExporter{
							Endpoint: helpers.GetPointer("my-endpoint"), // any value is invalid by the validator
						},
					},
				},
			},
			expErrSubstring: "telemetry.exporter.endpoint",
			expectErrCount:  1,
		},
		{
			name:      "invalid interval",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{
						Exporter: &ngfAPIv1alpha2.TelemetryExporter{
							Interval: helpers.GetPointer[ngfAPIv1alpha1.Duration](
								"my-interval",
							), // any value is invalid by the validator
						},
					},
				},
			},
			expErrSubstring: "telemetry.exporter.interval",
			expectErrCount:  1,
		},
		{
			name:      "invalid spanAttributes",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{
						SpanAttributes: []ngfAPIv1alpha1.SpanAttribute{
							{Key: "my-key", Value: "my-value"}, // any value is invalid by the validator
						},
					},
				},
			},
			expErrSubstring: "telemetry.spanAttributes",
			expectErrCount:  2,
		},
		{
			name:      "invalid ipFamily type",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{},
					IPFamily:  helpers.GetPointer[ngfAPIv1alpha2.IPFamilyType]("invalid"),
				},
			},
			expErrSubstring: "spec.ipFamily",
			expectErrCount:  1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateNginxProxy(test.validator, test.np)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
			if len(allErrs) > 0 {
				g.Expect(allErrs.ToAggregate().Error()).To(ContainSubstring(test.expErrSubstring))
			}
		})
	}
}

func TestValidateRewriteClientIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		np             *ngfAPIv1alpha2.NginxProxy
		validator      *validationfakes.FakeGenericValidator
		name           string
		errorString    string
		expectErrCount int
	}{
		{
			name:      "valid rewriteClientIP",
			validator: createValidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "2001:db8:a0b:12f0::1/128",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "10.56.32.11/32",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPIPAddressType,
								Value: "1.1.1.1",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPIPAddressType,
								Value: "2001:db8:a0b:12f0::1",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPHostnameAddressType,
								Value: "example.com",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 0,
		},
		{
			name:      "invalid CIDR in trustedAddresses",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "2001:db8::/129",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "10.0.0.1/32",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 1,
			errorString: "spec.rewriteClientIP.trustedAddresses.value: Invalid value: " +
				"\"2001:db8::/129\": must be a valid CIDR value, (e.g. 10.9.8.0/24 or 2001:db8::/64)",
		},
		{
			name:      "invalid IP address in trustedAddresses",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPIPAddressType,
								Value: "1.2.3.4.5",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPIPAddressType,
								Value: "10.0.0.1",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 1,
			errorString: "spec.rewriteClientIP.trustedAddresses.value: Invalid value: " +
				"\"1.2.3.4.5\": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)",
		},
		{
			name:      "invalid hostname in trustedAddresses",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPHostnameAddressType,
								Value: "bad-host$%^",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPHostnameAddressType,
								Value: "example.com",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 1,
			errorString: "spec.rewriteClientIP.trustedAddresses.value: Invalid value: \"bad-host$%^\": " +
				"a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', " +
				"and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation " +
				"is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')",
		},
		{
			name:      "invalid when mode is set and trustedAddresses is empty",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 1,
			errorString:    "spec.rewriteClientIP: Required value: trustedAddresses field required when mode is set",
		},
		{
			name:      "invalid when trustedAddresses is greater in length than 16",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.RewriteClientIPCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
						},
					},
				},
			},
			expectErrCount: 1,
			errorString:    "spec.rewriteClientIP.trustedAddresses: Too many: 21: must have at most 16 items",
		},
		{
			name:      "invalid when mode is not proxyProtocol or XForwardedFor",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeType("invalid")),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "2001:db8:a0b:12f0::1/128",
							},
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "10.0.0.1/32",
							},
						},
					},
				},
			},
			expectErrCount: 1,
			errorString: "spec.rewriteClientIP.mode: Unsupported value: \"invalid\": " +
				"supported values: \"ProxyProtocol\", \"XForwardedFor\"",
		},
		{
			name:      "invalid when mode is not proxyProtocol or XForwardedFor and trustedAddresses is empty",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeType("invalid")),
					},
				},
			},
			expectErrCount: 2,
			errorString: "[spec.rewriteClientIP: Required value: trustedAddresses field " +
				"required when mode is set, spec.rewriteClientIP.mode: " +
				"Unsupported value: \"invalid\": supported values: \"ProxyProtocol\", \"XForwardedFor\"]",
		},
		{
			name:      "invalid address type in trustedAddresses",
			validator: createInvalidValidator(),
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPAddressType("invalid"),
								Value: "2001:db8::/129",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				},
			},
			expectErrCount: 1,
			errorString: "spec.rewriteClientIP.trustedAddresses.type: " +
				"Unsupported value: \"invalid\": supported values: \"CIDR\", \"IPAddress\", \"Hostname\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateRewriteClientIP(test.np)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
			if len(allErrs) > 0 {
				g.Expect(allErrs.ToAggregate().Error()).To(Equal(test.errorString))
			}
		})
	}
}

func TestValidateLogging(t *testing.T) {
	t.Parallel()
	invalidLogLevel := ngfAPIv1alpha2.NginxErrorLogLevel("invalid-log-level")

	tests := []struct {
		np             *ngfAPIv1alpha2.NginxProxy
		name           string
		errorString    string
		expectErrCount int
	}{
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
					},
				},
			},
			name:           "valid debug log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelInfo),
					},
				},
			},
			name:           "valid info log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelNotice),
					},
				},
			},
			name:           "valid notice log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelWarn),
					},
				},
			},
			name:           "valid warn log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelError),
					},
				},
			},
			name:           "valid error log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelCrit),
					},
				},
			},
			name:           "valid crit log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelAlert),
					},
				},
			},
			name:           "valid alert log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelEmerg),
					},
				},
			},
			name:           "valid emerg log level",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: &invalidLogLevel,
					},
				},
			},
			name: "invalid log level",
			errorString: "spec.logging.errorLevel: Unsupported value: \"invalid-log-level\": supported values:" +
				" \"debug\", \"info\", \"notice\", \"warn\", \"error\", \"crit\", \"alert\", \"emerg\"",
			expectErrCount: 1,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Logging: &ngfAPIv1alpha2.NginxLogging{},
				},
			},
			name:           "empty log level",
			errorString:    "",
			expectErrCount: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateLogging(test.np)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
			if len(allErrs) > 0 {
				g.Expect(allErrs.ToAggregate().Error()).To(Equal(test.errorString))
			}
		})
	}
}

func TestValidateNginxPlus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		np             *ngfAPIv1alpha2.NginxProxy
		name           string
		errorString    string
		expectErrCount int
	}{
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "2001:db8:a0b:12f0::1"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowCIDRAddressType, Value: "127.0.0.3/32"},
						},
					},
				},
			},
			name:           "valid NginxPlus",
			errorString:    "",
			expectErrCount: 0,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowCIDRAddressType, Value: "2001:db8:a0b:12f0::1/128"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowCIDRAddressType, Value: "127.0.0.3/37"},
						},
					},
				},
			},
			name: "invalid CIDR in AllowedAddresses",
			errorString: "spec.nginxPlus.value: Invalid value: \"127.0.0.3/37\": must be a valid CIDR value, " +
				"(e.g. 10.9.8.0/24 or 2001:db8::/64)",
			expectErrCount: 1,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3.5/32"},
						},
					},
				},
			},
			name: "invalid IP address in AllowedAddresses",
			errorString: "spec.nginxPlus.value: Invalid value: \"127.0.0.3.5/32\": must be a valid IP address, " +
				"(e.g. 10.9.8.7 or 2001:db8::ffff)",
			expectErrCount: 1,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowAddressType("Hostname"), Value: "example.com"},
						},
					},
				},
			},
			name: "hostname type in AllowedAddresses",
			errorString: "spec.nginxPlus.type: Unsupported value: \"Hostname\": supported " +
				"values: \"CIDR\", \"IPAddress\"",
			expectErrCount: 1,
		},
		{
			np: &ngfAPIv1alpha2.NginxProxy{
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowAddressType("invalid"), Value: "example.com"},
						},
					},
				},
			},
			name: "invalid type in AllowedAddresses",
			errorString: "spec.nginxPlus.type: Unsupported value: \"invalid\": supported " +
				"values: \"CIDR\", \"IPAddress\"",
			expectErrCount: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateNginxPlus(test.np)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
			if len(allErrs) > 0 {
				g.Expect(allErrs.ToAggregate().Error()).To(Equal(test.errorString))
			}
		})
	}
}

func TestValidateNginxProxy_NilCase(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Just testing the nil case for coverage reasons. The rest of the function is covered by other tests.
	g.Expect(buildNginxProxy(nil, &validationfakes.FakeGenericValidator{})).To(BeNil())
}
