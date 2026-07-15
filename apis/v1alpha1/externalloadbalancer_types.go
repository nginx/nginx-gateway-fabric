package v1alpha1

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=elbs
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=direct"

// ExternalLoadBalancer configures an external load balancer that fronts a Gateway.
// It references a Gateway through TargetRefs. NGINX Gateway Fabric provisions the
// external load balancer integration for the Gateway's data plane Service.
//
// ExternalLoadBalancer maps one-to-one to a Gateway: a Gateway yields exactly one data plane
// Service, so it is fronted by exactly one ExternalLoadBalancer. When more than one
// ExternalLoadBalancer references the same Gateway, the oldest is accepted and the others are
// rejected with Accepted=False.
//
// A resource configures exactly one external load balancer backend. The gatewayLink backend
// integrates F5 BIG-IP through F5 CIS.
type ExternalLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ExternalLoadBalancer.
	Spec ExternalLoadBalancerSpec `json:"spec"`

	// Status defines the state of the ExternalLoadBalancer.
	Status ExternalLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ExternalLoadBalancerList contains a list of ExternalLoadBalancer.
type ExternalLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalLoadBalancer `json:"items"`
}

// ExternalLoadBalancerSpec defines the desired state of ExternalLoadBalancer.
//
// +kubebuilder:validation:XValidation:message="exactly one external load balancer backend must be set",rule="has(self.gatewayLink)"
//
//nolint:lll
type ExternalLoadBalancerSpec struct {
	// GatewayLink configures F5 BIG-IP as the external load balancer using F5
	// Container Ingress Services. It is the first supported backend. Additional
	// backend types may be added as sibling fields in the future.
	//
	// +optional
	GatewayLink *GatewayLinkConfig `json:"gatewayLink,omitempty"`

	// TargetRefs identifies the Gateways this external load balancer applies to.
	// Each object must be in the same namespace as the ExternalLoadBalancer resource.
	// Exactly one Gateway is supported for now.
	// Support: Gateway.
	//
	// +kubebuilder:validation:MaxItems=1
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be Gateway",rule="self.all(ref, ref.kind=='Gateway')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(ref, ref.group=='gateway.networking.k8s.io')"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`
}

// GatewayLinkConfig defines the configuration for integrating with F5 BIG-IP
// as the external load balancer for NGINX Gateway Fabric using F5
// Container Ingress Services.
// IngressLink API Definition: https://github.com/F5Networks/k8s-bigip-ctlr/blob/master/docs/config_examples/customResourceDefinitions/customresourcedefinitions.yml
//
// +kubebuilder:validation:XValidation:message="virtualServerAddress and ipamLabel are mutually exclusive",rule="!(has(self.virtualServerAddress) && has(self.ipamLabel))"
// +kubebuilder:validation:XValidation:message="one of virtualServerAddress or ipamLabel must be set",rule="has(self.virtualServerAddress) || has(self.ipamLabel)"
// +kubebuilder:validation:XValidation:message="partition cannot be Common",rule="!has(self.partition) || self.partition != 'Common'"
//
//nolint:lll
type GatewayLinkConfig struct {
	// VirtualServerAddress is the static IP address to configure on BIG-IP for the virtual server.
	// This is mutually exclusive with IPAMLabel.
	//
	// +kubebuilder:validation:Pattern=`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`
	// +optional
	VirtualServerAddress *string `json:"virtualServerAddress,omitempty"`

	// VirtualServerName is a custom name for the BIG-IP virtual server.
	//
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]+([A-z0-9-._+])*([A-z0-9])$`
	// +optional
	VirtualServerName *string `json:"virtualServerName,omitempty"`

	// IPAMLabel is the label used by F5 IPAM Controller to allocate an IP address.
	// The IPAM controller will assign an IP from the pool associated with this label.
	// This is mutually exclusive with VirtualServerAddress.
	//
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]+[-A-z0-9_.:]+[A-z0-9]+$`
	// +optional
	IPAMLabel *string `json:"ipamLabel,omitempty"`

	// Host is the hostname for the BIG-IP virtual server.
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(([a-zA-Z0-9\*]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`
	Host *string `json:"host,omitempty"`

	// Partition is the BIG-IP partition where resources will be created.
	// The partition must already exist on BIG-IP and cannot be "Common".
	//
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]+[-A-Za-z0-9_.]+$`
	// +optional
	Partition *string `json:"partition,omitempty"`

	// BigIPRouteDomain is the route domain ID for the BIG-IP virtual server.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +optional
	BigIPRouteDomain *int32 `json:"bigipRouteDomain,omitempty"`

	// TLS defines the TLS configuration for the BIG-IP virtual server.
	//
	// +optional
	TLS *GatewayLinkTLS `json:"tls,omitempty"`

	// MultiCluster defines the multi-cluster configuration for load balancing traffic
	// across NGINX instances in multiple clusters.
	//
	// +optional
	MultiCluster *GatewayLinkMultiCluster `json:"multiCluster,omitempty"`

	// ServiceAddress configures Layer 3 settings for the BIG-IP virtual server address.
	//
	// +optional
	ServiceAddress *GatewayLinkServiceAddress `json:"serviceAddress,omitempty"`

	// AdditionalIngressLinkSpec is an escape hatch for IngressLink fields that are not yet
	// modeled by GatewayLink. Its contents are merged verbatim into the generated IngressLink
	// spec and are NOT validated by NGINX Gateway Fabric. Fields set here take lower precedence
	// than the explicitly modeled GatewayLink fields above; NGINX Gateway Fabric always sets the
	// IngressLink selector internally and it cannot be overridden through this field. Use with
	// caution since contents bypass schema validation, defaulting, and CEL rules, and flow through to
	// BIG-IP via F5 CIS.
	//
	// +kubebuilder:validation:Type=object
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AdditionalIngressLinkSpec *apiextv1.JSON `json:"additionalIngressLinkSpec,omitempty"`

	// IRules is a list of BIG-IP iRules to apply to the virtual server.
	// Each iRule must be specified using the full path format /partition/irule_name,
	// for example "/Common/Proxy_Protocol_iRule".
	//
	// +kubebuilder:validation:items:Pattern=`^\/[a-zA-Z]+([A-z0-9-_+]+\/)+([-A-z0-9_.:]+\/?)*$`
	// +optional
	IRules []string `json:"iRules,omitempty"`

	// Monitors is a list of BIG-IP health monitors to associate with the virtual server pool.
	//
	// +optional
	Monitors []GatewayLinkMonitor `json:"monitors,omitempty"`
}

// GatewayLinkServiceAddress configures Layer 3 settings for the BIG-IP virtual server address.
type GatewayLinkServiceAddress struct {
	// ICMPEcho controls whether the virtual server address responds to ICMP echo (ping).
	//
	// +optional
	ICMPEcho *ICMPEcho `json:"icmpEcho,omitempty"`

	// TrafficGroup is the BIG-IP traffic group that owns the virtual server address,
	// in the full path format, for example "/Common/traffic-group-test".
	//
	// +kubebuilder:validation:Pattern=`^\/([A-z0-9-_+]+\/)+([-A-z0-9_.:]+\/?)*$`
	// +optional
	TrafficGroup *string `json:"trafficGroup,omitempty"`
}

// ICMPEcho controls whether the BIG-IP virtual server address responds to ICMP echo.
// +kubebuilder:validation:Enum=enable;disable;selective
type ICMPEcho string

const (
	// ICMPEchoEnable means the virtual server address always responds to ICMP echo.
	ICMPEchoEnable ICMPEcho = "enable"

	// ICMPEchoDisable means the virtual server address never responds to ICMP echo.
	ICMPEchoDisable ICMPEcho = "disable"

	// ICMPEchoSelective means BIG-IP responds to ICMP echo based on the state of the virtual server.
	ICMPEchoSelective ICMPEcho = "selective"
)

// TLSReferenceType specifies where the BIG-IP SSL profiles come from.
// +kubebuilder:validation:Enum=bigip;secret
type TLSReferenceType string

const (
	// TLSReferenceBigIP means the SSL profiles already exist on BIG-IP.
	TLSReferenceBigIP TLSReferenceType = "bigip"

	// TLSReferenceSecret means the SSL profiles are sourced from Kubernetes secrets.
	TLSReferenceSecret TLSReferenceType = "secret"
)

// GatewayLinkTLS defines the TLS configuration for the BIG-IP virtual server.
type GatewayLinkTLS struct {
	// Reference specifies the source of the SSL profiles. "bigip" means the profiles already
	// exist on BIG-IP. "secret" means they come from Kubernetes secrets of type kubernetes.io/tls.
	//
	// +optional
	Reference *TLSReferenceType `json:"reference,omitempty"`

	// ClientSSLs is a list of client SSL profiles that BIG-IP uses to terminate TLS from the client.
	// When reference is "bigip", each entry is the full path of a profile on BIG-IP in the form
	// /partition/profile_name, for example /Common/clientssl. When reference is "secret", each entry
	// is the name of a Kubernetes secret of type kubernetes.io/tls that holds the certificate and key.
	//
	// +kubebuilder:validation:items:Pattern=`^\/?[a-zA-Z]+([-A-z0-9_+]+\/)*([-A-z0-9_.:]+\/?)*$`
	// +optional
	ClientSSLs []string `json:"clientSSLs,omitempty"`

	// ServerSSLs is a list of server SSL profiles that BIG-IP uses to re-encrypt traffic to NGINX.
	// When reference is "bigip", each entry is the full path of a profile on BIG-IP in the form
	// /partition/profile_name, for example /Common/serverssl. When reference is "secret", each entry
	// is the name of a Kubernetes secret of type kubernetes.io/tls that holds the certificate and key.
	//
	// +kubebuilder:validation:items:Pattern=`^\/?[a-zA-Z]+([-A-z0-9_+]+\/)*([-A-z0-9_.:]+\/?)*$`
	// +optional
	ServerSSLs []string `json:"serverSSLs,omitempty"`
}

// GatewayLinkMonitor defines a BIG-IP health monitor reference.
type GatewayLinkMonitor struct {
	// Name is the full path of the health monitor on BIG-IP (e.g., "/Common/http").
	//
	// +kubebuilder:validation:Pattern=`^\/[a-zA-Z]+([A-z0-9-_+]+\/)+([-A-z0-9_.:]+\/?)*$`
	Name string `json:"name"`

	// Reference specifies the source of the monitor. Currently only "bigip" is supported.
	//
	// +kubebuilder:validation:Enum=bigip
	Reference string `json:"reference"`
}

// GatewayLinkMultiCluster defines the multi-cluster configuration for GatewayLink.
// When configured, CIS load balances traffic across NGINX instances
// in multiple clusters. This is set only on the cluster that runs CIS. The other
// clusters run NGINX with a matching Gateway and Service but not CIS,
// so they do not set multiCluster. CIS reaches those clusters over a kubeconfig.
type GatewayLinkMultiCluster struct {
	// LocalClusterName is the name of this cluster as configured in the CIS deployment
	// via the --local-cluster-name flag. NGINX Gateway Fabric uses it as the cluster name
	// for the local entry in the IngressLink's multiClusterServices, which points at this
	// cluster's own Gateway Service. It must match the name CIS knows this cluster by,
	// otherwise CIS cannot resolve the local service.
	LocalClusterName string `json:"localClusterName"`

	// RemoteClusters is the list of remote clusters that also run NGINX Gateway Fabric.
	//
	// +kubebuilder:validation:MinItems=1
	RemoteClusters []GatewayLinkRemoteCluster `json:"remoteClusters"`
}

// GatewayLinkRemoteCluster defines a remote cluster for multi-cluster load balancing.
type GatewayLinkRemoteCluster struct {
	// ClusterName is one of the names of the remote clusters as configured in the CIS deployment.
	//
	// +kubebuilder:validation:Required
	ClusterName *string `json:"clusterName"`

	// Namespace is the namespace of the NGINX service in the remote cluster.
	// If not specified, defaults to the local Gateway's namespace.
	//
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Service is the name of the NGINX service in the remote cluster.
	// If not specified, defaults to the local Gateway's service name.
	//
	// +optional
	Service *string `json:"service,omitempty"`

	// Weight is the load balancing weight for this cluster's service.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=256
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// ExternalLoadBalancerStatus defines the state of ExternalLoadBalancer.
type ExternalLoadBalancerStatus struct {
	// Controllers is a list of Gateway API controllers that processed the ExternalLoadBalancer
	// and the status of the ExternalLoadBalancer with respect to each controller.
	//
	// +kubebuilder:validation:MaxItems=16
	Controllers []ControllerStatus `json:"controllers,omitempty"`
}

// ExternalLoadBalancerConditionType is a type of condition associated with ExternalLoadBalancer.
type ExternalLoadBalancerConditionType string

// ExternalLoadBalancerConditionReason is a reason for an ExternalLoadBalancer condition type.
type ExternalLoadBalancerConditionReason string

const (
	// ExternalLoadBalancerConditionTypeAccepted indicates that the ExternalLoadBalancer is accepted.
	//
	// Possible reasons for this condition to be True:
	//
	// * Accepted
	//
	// Possible reasons for this condition to be False:
	//
	// * Invalid
	// * Conflicted.
	ExternalLoadBalancerConditionTypeAccepted ExternalLoadBalancerConditionType = "Accepted"

	// ExternalLoadBalancerConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	ExternalLoadBalancerConditionReasonAccepted ExternalLoadBalancerConditionReason = "Accepted"

	// ExternalLoadBalancerConditionReasonInvalid is used with the Accepted condition type when
	// the ExternalLoadBalancer is invalid.
	ExternalLoadBalancerConditionReasonInvalid ExternalLoadBalancerConditionReason = "Invalid"

	// ExternalLoadBalancerConditionReasonConflicted is used with the Accepted condition type when
	// another ExternalLoadBalancer already references the same Gateway. A Gateway can be fronted by
	// exactly one external load balancer, so the oldest is accepted and the others are Conflicted.
	ExternalLoadBalancerConditionReasonConflicted ExternalLoadBalancerConditionReason = "Conflicted"
)
