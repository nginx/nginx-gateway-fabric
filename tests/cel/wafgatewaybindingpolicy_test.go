package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

// newWAFPolicy is a test helper that creates a WAFGatewayBindingPolicy with the given spec.
func newWAFPolicy(
	t *testing.T,
	spec ngfAPIv1alpha1.WAFGatewayBindingPolicySpec,
) *ngfAPIv1alpha1.WAFGatewayBindingPolicy {
	t.Helper()
	return &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      uniqueResourceName(testResourceName),
			Namespace: defaultNamespace,
		},
		Spec: spec,
	}
}

// baseAPPolicySource returns a minimal valid APPolicySource for use in tests.
func baseAPPolicySource() *ngfAPIv1alpha1.APResourceReference {
	return &ngfAPIv1alpha1.APResourceReference{Name: "test-policy"}
}

// baseSecurityLog returns a minimal valid WAFSecurityLog with stderr destination.
func baseSecurityLog() ngfAPIv1alpha1.WAFSecurityLog {
	return ngfAPIv1alpha1.WAFSecurityLog{
		APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
		Destination: ngfAPIv1alpha1.SecurityLogDestination{
			Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
		},
	}
}

func TestWAFGatewayBindingPolicyTargetRefsAllSameKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "single Gateway targetRef is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "multiple Gateway targetRefs are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-b"},
				},
			},
		},
		{
			name: "multiple HTTPRoute targetRefs are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
		{
			name: "multiple GRPCRoute targetRefs are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
		{
			name:       "mixing Gateway and HTTPRoute targetRefs is invalid",
			wantErrors: []string{expectedTargetRefAllSameKindError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
				},
			},
		},
		{
			name:       "mixing Gateway and GRPCRoute targetRefs is invalid",
			wantErrors: []string{expectedTargetRefAllSameKindError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "route-a"},
				},
			},
		},
		{
			name:       "mixing HTTPRoute and GRPCRoute targetRefs is invalid",
			wantErrors: []string{expectedTargetRefAllSameKindError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
		{
			name:       "mixing all three kinds is invalid",
			wantErrors: []string{expectedTargetRefAllSameKindError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				if tt.spec.TargetRefs[i].Name == "" {
					tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
				}
			}
			validateCrd(t, tt.wantErrors, newWAFPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyTargetRefsKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Gateway kind is allowed",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "HTTPRoute kind is allowed",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "GRPCRoute kind is allowed",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "invalid kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: invalidKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "TCPRoute kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: tcpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "one invalid kind among valid kinds is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: invalidKind, Group: gatewayGroup, Name: "bad"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "good"},
				},
			},
		},
		{
			name:       "multiple invalid kinds are not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: invalidKind, Group: gatewayGroup, Name: "bad-a"},
					{Kind: invalidKind, Group: gatewayGroup, Name: "bad-b"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				if tt.spec.TargetRefs[i].Name == "" {
					tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
				}
			}
			validateCrd(t, tt.wantErrors, newWAFPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyTargetRefsGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "gateway.networking.k8s.io group is allowed",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "invalid group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: invalidGroup},
				},
			},
		},
		{
			name:       "one invalid group among valid groups is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: invalidGroup, Name: "gw-a"},
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-b"},
				},
			},
		},
		{
			name:       "multiple invalid groups are not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: invalidGroup, Name: "gw-a"},
					{Kind: gatewayKind, Group: discoveryGroup, Name: "gw-b"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				if tt.spec.TargetRefs[i].Name == "" {
					tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
				}
			}
			validateCrd(t, tt.wantErrors, newWAFPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyTargetRefsNameUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "single targetRef is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
				},
			},
		},
		{
			name: "multiple targetRefs with unique names are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
		{
			name:       "duplicate names are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
				},
			},
		},
		{
			name:       "same name across different kinds is not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "shared-name"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "shared-name"},
				},
			},
		},
		{
			name:       "one duplicate among three targetRefs is not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-b"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
				},
			},
		},
		{
			name:       "multiple duplicate pairs are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-b"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, newWAFPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicySecurityLogDestinationFile(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "file destination with type file is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
							File: &ngfAPIv1alpha1.SecurityLogFile{Path: "/var/log/waf.log"},
						},
					},
				},
			},
		},
		{
			name: "no file field with type stderr is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs:   []ngfAPIv1alpha1.WAFSecurityLog{baseSecurityLog()},
			},
		},
		{
			name: "no file field with type syslog is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type:   ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
							Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{Server: "syslog.example.com:514"},
						},
					},
				},
			},
		},
		{
			name:       "file field set with type stderr is invalid",
			wantErrors: []string{expectedWAFFileNilIfNotFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							File: &ngfAPIv1alpha1.SecurityLogFile{Path: "/var/log/waf.log"},
						},
					},
				},
			},
		},
		{
			name:       "file field set with type syslog is invalid",
			wantErrors: []string{expectedWAFFileNilIfNotFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
							File: &ngfAPIv1alpha1.SecurityLogFile{Path: "/var/log/waf.log"},
						},
					},
				},
			},
		},
		{
			name:       "missing file field with type file is invalid",
			wantErrors: []string{expectedWAFFileRequiredIfFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				if tt.spec.TargetRefs[i].Name == "" {
					tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
				}
			}
			validateCrd(t, tt.wantErrors, newWAFPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicySecurityLogDestinationSyslog(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "syslog destination with type syslog is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type:   ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
							Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{Server: "syslog.example.com:514"},
						},
					},
				},
			},
		},
		{
			name: "no syslog field with type stderr is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs:   []ngfAPIv1alpha1.WAFSecurityLog{baseSecurityLog()},
			},
		},
		{
			name: "no syslog field with type file is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
							File: &ngfAPIv1alpha1.SecurityLogFile{Path: "/var/log/waf.log"},
						},
					},
				},
			},
		},
		{
			name:       "syslog field set with type stderr is invalid",
			wantErrors: []string{expectedWAFSyslogNilIfNotSyslogTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type:   ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{Server: "syslog.example.com:514"},
						},
					},
				},
			},
		},
		{
			name:       "syslog field set with type file is invalid",
			wantErrors: []string{expectedWAFSyslogNilIfNotSyslogTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type:   ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
							Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{Server: "syslog.example.com:514"},
						},
					},
				},
			},
		},
		{
			name:       "missing syslog field with type syslog is invalid",
			wantErrors: []string{expectedWAFSyslogRequiredIfSyslogTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				APPolicySource: baseAPPolicySource(),
				TargetRefs:     []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						APLogConfSource: ngfAPIv1alpha1.APResourceReference{Name: "test-logconf"},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				if tt.spec.TargetRefs[i].Name == "" {
					tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
				}
			}
			validateCrd(t, tt.wantErrors, newWAFPolicy(t, tt.spec), k8sClient)
		})
	}
}
