package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// newWAFGatewayBindingPolicy is a test helper that creates a WAFGatewayBindingPolicy with the given spec.
// If Type is not set, it defaults to HTTP with a valid PolicySource so tests focused on other fields
// do not need to set unrelated required fields.
func newWAFGatewayBindingPolicy(
	t *testing.T,
	spec ngfAPIv1alpha1.WAFGatewayBindingPolicySpec,
) *ngfAPIv1alpha1.WAFGatewayBindingPolicy {
	t.Helper()
	if spec.Type == "" {
		spec.Type = ngfAPIv1alpha1.PolicySourceTypeHTTP
		spec.PolicySource = ngfAPIv1alpha1.PolicySource{
			HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
		}
	}
	return &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      uniqueResourceName(testResourceName),
			Namespace: defaultNamespace,
		},
		Spec: spec,
	}
}

// baseLogSource returns a minimal valid LogSource with a URL for use in tests.
func baseLogSource() ngfAPIv1alpha1.LogSource {
	url := "https://example.com/log.tgz"
	return ngfAPIv1alpha1.LogSource{
		URL: &url,
	}
}

// baseSecurityLog returns a minimal valid WAFSecurityLog with stderr destination.
func baseSecurityLog() ngfAPIv1alpha1.WAFSecurityLog {
	return ngfAPIv1alpha1.WAFSecurityLog{
		LogSource: baseLogSource(),
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
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "multiple Gateway targetRefs are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-b"},
				},
			},
		},
		{
			name: "multiple HTTPRoute targetRefs are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-a"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "route-b"},
				},
			},
		},
		{
			name: "multiple GRPCRoute targetRefs are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
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
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "HTTPRoute kind is allowed",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "GRPCRoute kind is allowed",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "invalid kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: invalidKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "TCPRoute kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: tcpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "one invalid kind among valid kinds is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
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
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "invalid group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: invalidGroup},
				},
			},
		},
		{
			name:       "one invalid group among valid groups is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
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
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup, Name: "gw-a"},
				},
			},
		},
		{
			name: "multiple targetRefs with unique names are valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
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
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
				TargetRefs:   []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{baseSecurityLog()},
			},
		},
		{
			name: "no file field with type syslog is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
			wantErrors: []string{expectedWAFFileIfAndOnlyIfFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
			wantErrors: []string{expectedWAFFileIfAndOnlyIfFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
			wantErrors: []string{expectedWAFFileIfAndOnlyIfFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
						},
					},
				},
			},
		},
		{
			name:       "both file and syslog set with type file is invalid",
			wantErrors: []string{expectedWAFSyslogIfAndOnlyIfSyslogType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type:   ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
							File:   &ngfAPIv1alpha1.SecurityLogFile{Path: "/var/log/waf.log"},
							Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{Server: "syslog.example.com:514"},
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
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
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
				TargetRefs:   []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{baseSecurityLog()},
			},
		},
		{
			name: "no syslog field with type file is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
			wantErrors: []string{expectedWAFSyslogIfAndOnlyIfSyslogType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
			wantErrors: []string{expectedWAFSyslogIfAndOnlyIfSyslogType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
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
			wantErrors: []string{expectedWAFSyslogIfAndOnlyIfSyslogType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
						},
					},
				},
			},
		},
		{
			name:       "both file and syslog set with type syslog is invalid",
			wantErrors: []string{expectedWAFFileIfAndOnlyIfFileTypeError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: baseLogSource(),
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type:   ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
							File:   &ngfAPIv1alpha1.SecurityLogFile{Path: "/var/log/waf.log"},
							Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{Server: "syslog.example.com:514"},
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyLogSourceMutualExclusion(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	logURL := "https://example.com/log.tgz"
	defaultProfile := ngfAPIv1alpha1.DefaultLogProfileBlocked

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "url only is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource:   ngfAPIv1alpha1.LogSource{URL: &logURL},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr},
					},
				},
			},
		},
		{
			name: "defaultProfile only is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource:   ngfAPIv1alpha1.LogSource{DefaultProfile: &defaultProfile},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr},
					},
				},
			},
		},
		{
			name:       "both url and defaultProfile set is invalid",
			wantErrors: []string{expectedWAFLogSourceMutualExclusionError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL:            &logURL,
							DefaultProfile: &defaultProfile,
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr},
					},
				},
			},
		},
		{
			name:       "neither url nor defaultProfile set is invalid",
			wantErrors: []string{expectedWAFLogSourceMutualExclusionError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource:   ngfAPIv1alpha1.LogSource{},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr},
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyPolicySource(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	namespace := "my-namespace"

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "HTTP type with httpSource is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
				},
			},
		},
		{
			name: "NIM type with nimSource is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeNIM,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:        "https://nim.example.com",
						PolicyName: helpers.GetPointer("my-policy"),
					},
				},
			},
		},
		{
			name: "N1C type with n1cSource is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:          "https://n1c.example.com",
						PolicyName:   helpers.GetPointer("my-policy"),
						N1CNamespace: namespace,
					},
				},
			},
		},
		{
			name:       "NIM type without nimSource is invalid",
			wantErrors: []string{expectedWAFNIMSourceIfAndOnlyIfNIMType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs:   []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:         ngfAPIv1alpha1.PolicySourceTypeNIM,
				PolicySource: ngfAPIv1alpha1.PolicySource{},
			},
		},
		{
			name:       "N1C type without n1cSource is invalid",
			wantErrors: []string{expectedWAFN1CSourceIfAndOnlyIfN1CType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs:   []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:         ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{},
			},
		},
		{
			name:       "nimSource set with HTTP type is invalid",
			wantErrors: []string{expectedWAFNIMSourceIfAndOnlyIfNIMType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:        "https://nim.example.com",
						PolicyName: helpers.GetPointer("my-policy"),
					},
				},
			},
		},
		{
			name:       "n1cSource set with HTTP type is invalid",
			wantErrors: []string{expectedWAFN1CSourceIfAndOnlyIfN1CType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:          "https://n1c.example.com",
						PolicyName:   helpers.GetPointer("my-policy"),
						N1CNamespace: namespace,
					},
				},
			},
		},
		{
			name:       "nimSource set with N1C type is invalid",
			wantErrors: []string{expectedWAFNIMSourceIfAndOnlyIfNIMType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:          "https://n1c.example.com",
						PolicyName:   helpers.GetPointer("my-policy"),
						N1CNamespace: namespace,
					},
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:        "https://nim.example.com",
						PolicyName: helpers.GetPointer("my-policy"),
					},
				},
			},
		},
		{
			name:       "n1cSource set with NIM type is invalid",
			wantErrors: []string{expectedWAFN1CSourceIfAndOnlyIfN1CType},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeNIM,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:        "https://nim.example.com",
						PolicyName: helpers.GetPointer("my-policy"),
					},
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:          "https://n1c.example.com",
						PolicyName:   helpers.GetPointer("my-policy"),
						N1CNamespace: namespace,
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyBundleValidation(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	validChecksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "verifyChecksum alone is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
					Validation: &ngfAPIv1alpha1.BundleValidation{
						VerifyChecksum: true,
					},
				},
			},
		},
		{
			name: "expectedChecksum alone is valid",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
					Validation: &ngfAPIv1alpha1.BundleValidation{
						ExpectedChecksum: &validChecksum,
					},
				},
			},
		},
		{
			name:       "verifyChecksum and expectedChecksum together is invalid",
			wantErrors: []string{expectedWAFValidationMutualExclusionError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
					Validation: &ngfAPIv1alpha1.BundleValidation{
						VerifyChecksum:   true,
						ExpectedChecksum: &validChecksum,
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyNIMPolicyUID(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "valid UUID is accepted",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeNIM,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:       "https://nim.example.com",
						PolicyUID: helpers.GetPointer("2bc1e3ac-7990-4ca4-910a-8634c444c804"),
					},
				},
			},
		},
		{
			name:       "non-UUID string is rejected",
			wantErrors: []string{expectedWAFNIMPolicyUIDPatternError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeNIM,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:       "https://nim.example.com",
						PolicyUID: helpers.GetPointer("not-a-uuid"),
					},
				},
			},
		},
		{
			name:       "UUID with uppercase letters is rejected",
			wantErrors: []string{expectedWAFNIMPolicyUIDPatternError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeNIM,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
						URL:       "https://nim.example.com",
						PolicyUID: helpers.GetPointer("2BC1E3AC-7990-4CA4-910A-8634C444C804"),
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyN1CPolicyObjectID(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	namespace := "my-namespace"

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "valid pol_ ID is accepted",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:            "https://n1c.example.com",
						PolicyObjectID: helpers.GetPointer("pol_-IUuEUN7ST63oRC7AlQPLw"),
						N1CNamespace:   namespace,
					},
				},
			},
		},
		{
			name:       "missing pol_ prefix is rejected",
			wantErrors: []string{expectedWAFN1CPolicyObjectIDPatternError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:            "https://n1c.example.com",
						PolicyObjectID: helpers.GetPointer("IUuEUN7ST63oRC7AlQPLw"),
						N1CNamespace:   namespace,
					},
				},
			},
		},
		{
			name:       "invalid characters in pol_ ID are rejected",
			wantErrors: []string{expectedWAFN1CPolicyObjectIDPatternError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:            "https://n1c.example.com",
						PolicyObjectID: helpers.GetPointer("pol_invalid!chars"),
						N1CNamespace:   namespace,
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}

func TestWAFGatewayBindingPolicyN1CPolicyVersionID(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	namespace := "my-namespace"

	tests := []struct {
		spec       ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "valid pv_ UID is accepted",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:             "https://n1c.example.com",
						PolicyName:      helpers.GetPointer("my-policy"),
						PolicyVersionID: helpers.GetPointer("pv_UJ2gL5fOQ3Gnb3OVuVo1XA"),
						N1CNamespace:    namespace,
					},
				},
			},
		},
		{
			name:       "missing pv_ prefix is rejected",
			wantErrors: []string{expectedWAFN1CPolicyVersionIDPatternError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:             "https://n1c.example.com",
						PolicyName:      helpers.GetPointer("my-policy"),
						PolicyVersionID: helpers.GetPointer("UJ2gL5fOQ3Gnb3OVuVo1XA"),
						N1CNamespace:    namespace,
					},
				},
			},
		},
		{
			name:       "invalid characters in pv_ UID are rejected",
			wantErrors: []string{expectedWAFN1CPolicyVersionIDPatternError},
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Kind: gatewayKind, Group: gatewayGroup}},
				Type:       ngfAPIv1alpha1.PolicySourceTypeN1C,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
						URL:             "https://n1c.example.com",
						PolicyName:      helpers.GetPointer("my-policy"),
						PolicyVersionID: helpers.GetPointer("pv_invalid!chars"),
						N1CNamespace:    namespace,
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
			validateCrd(t, tt.wantErrors, newWAFGatewayBindingPolicy(t, tt.spec), k8sClient)
		})
	}
}
