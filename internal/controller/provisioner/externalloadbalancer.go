package provisioner

import (
	"encoding/json"

	"github.com/go-logr/logr"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// IngressLink spec field names, as defined by the F5 CIS IngressLink CRD. They are the keys written
// into the unstructured IngressLink spec, so they must match the CRD's schema exactly.
const (
	ilKeyVirtualServerAddress = "virtualServerAddress"
	ilKeyVirtualServerName    = "virtualServerName"
	ilKeyIPAMLabel            = "ipamLabel"
	ilKeyHost                 = "host"
	ilKeyPartition            = "partition"
	ilKeyBigIPRouteDomain     = "bigipRouteDomain"
	ilKeyIRules               = "iRules"
	ilKeyMonitors             = "monitors"
	ilKeyTLS                  = "tls"
	ilKeyServiceAddress       = "serviceAddress"
	ilKeyMultiClusterServices = "multiClusterServices"
	ilKeySelector             = "selector"

	ilKeyName         = "name"
	ilKeyReference    = "reference"
	ilKeyClientSSLs   = "clientSSLs"
	ilKeyServerSSLs   = "serverSSLs"
	ilKeyICMPEcho     = "icmpEcho"
	ilKeyTrafficGroup = "trafficGroup"
	ilKeyMatchLabels  = "matchLabels"
	ilKeyClusterName  = "clusterName"
	ilKeyNamespace    = "namespace"
	ilKeyService      = "service"
	ilKeyWeight       = "weight"
)

func extractExternalLoadBalancer(gateway *graph.Gateway) *ngfAPIv1alpha1.ExternalLoadBalancer {
	if gateway == nil {
		return nil
	}

	return gateway.ExternalLoadBalancer
}

// buildExternalLoadBalancer builds the object that fronts the Gateway's data plane Service with the
// external load balancer.
func (p *NginxProvisioner) buildExternalLoadBalancer(
	objectMeta metav1.ObjectMeta,
	elb *ngfAPIv1alpha1.ExternalLoadBalancer,
	selectorLabels map[string]string,
) client.Object {
	if !p.cfg.ExternalLoadBalancer || elb == nil {
		return nil
	}

	if elb.Spec.GatewayLink != nil {
		return p.buildIngressLink(objectMeta, elb.Spec.GatewayLink, selectorLabels)
	}

	return nil
}

func (p *NginxProvisioner) buildIngressLink(
	objectMeta metav1.ObjectMeta,
	gatewayLink *ngfAPIv1alpha1.GatewayLinkConfig,
	selectorLabels map[string]string,
) client.Object {
	il := &unstructured.Unstructured{}
	il.SetGroupVersionKind(kinds.IngressLinkGVK)
	il.SetName(objectMeta.Name)
	il.SetNamespace(objectMeta.Namespace)
	il.SetLabels(objectMeta.Labels)
	il.SetAnnotations(objectMeta.Annotations)
	il.Object["spec"] = buildIngressLinkSpec(gatewayLink, objectMeta, selectorLabels, p.cfg.Logger)

	return il
}

// buildIngressLinkSpec builds the IngressLink spec from a GatewayLinkConfig.
//
// The modeled fields win over additionalIngressLinkSpec, and the selector wins over everything: the
// escape hatch sets XPreserveUnknownFields, so letting it override a modeled field would be a way to
// bypass that field's CEL validation, and overriding the selector would break the link to the data
// plane Service. Writing is last-wins, so the escape hatch is laid down first to be overwritten.
func buildIngressLinkSpec(
	ilCfg *ngfAPIv1alpha1.GatewayLinkConfig,
	objectMeta metav1.ObjectMeta,
	selectorLabels map[string]string,
	logger logr.Logger,
) map[string]any {
	spec := unmarshalAdditionalSpec(ilCfg.AdditionalIngressLinkSpec, logger)

	setIngressLinkAddress(spec, ilCfg)
	setIngressLinkOptionalFields(spec, ilCfg)
	setIngressLinkTLS(spec, ilCfg)
	setIngressLinkServiceAddress(spec, ilCfg)
	setIngressLinkMultiCluster(spec, ilCfg, objectMeta)

	// Selects the data plane Service whose endpoints become the BIG-IP pool members.
	spec[ilKeySelector] = map[string]any{ilKeyMatchLabels: toJSONMap(selectorLabels)}

	return spec
}

// toJSONSlice converts a []string to the []any that an Unstructured requires: it deep-copies its
// contents through DeepCopyJSONValue, which panics on a []string.
func toJSONSlice(strs []string) []any {
	out := make([]any, 0, len(strs))
	for _, s := range strs {
		out = append(out, s)
	}

	return out
}

// toJSONMap converts a map[string]string to the map[string]any that an Unstructured requires, for the
// same reason as toJSONSlice.
func toJSONMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}

	return out
}

// unmarshalAdditionalSpec decodes the additionalIngressLinkSpec escape hatch into the map the rest of
// the spec is built on. The API constrains it to an object, so a decode failure here would mean the
// stored resource is malformed and the whole escape hatch is dropped and logged.
func unmarshalAdditionalSpec(raw *apiextv1.JSON, logger logr.Logger) map[string]any {
	if raw == nil || len(raw.Raw) == 0 {
		return map[string]any{}
	}

	var m map[string]any
	if err := json.Unmarshal(raw.Raw, &m); err != nil {
		logger.Error(err, "ignoring invalid additionalIngressLinkSpec; must be a JSON object")
		return map[string]any{}
	}
	if m == nil {
		return map[string]any{}
	}

	return m
}

// setIngressLinkMultiCluster adds the local cluster's Service first, then each remote cluster,
// defaulting a remote's namespace and service to the local Gateway's when unset.
func setIngressLinkMultiCluster(
	spec map[string]any,
	ilCfg *ngfAPIv1alpha1.GatewayLinkConfig,
	objectMeta metav1.ObjectMeta,
) {
	if ilCfg.MultiCluster == nil {
		return
	}

	mcServices := make([]any, 0, len(ilCfg.MultiCluster.RemoteClusters)+1)

	mcServices = append(mcServices, map[string]any{
		ilKeyClusterName: ilCfg.MultiCluster.LocalClusterName,
		ilKeyNamespace:   objectMeta.Namespace,
		ilKeyService:     objectMeta.Name,
	})

	for _, remote := range ilCfg.MultiCluster.RemoteClusters {
		ns := objectMeta.Namespace
		if remote.Namespace != nil {
			ns = *remote.Namespace
		}
		svc := objectMeta.Name
		if remote.Service != nil {
			svc = *remote.Service
		}

		entry := map[string]any{
			ilKeyClusterName: *remote.ClusterName,
			ilKeyNamespace:   ns,
			ilKeyService:     svc,
		}
		if remote.Weight != nil {
			entry[ilKeyWeight] = *remote.Weight
		}
		mcServices = append(mcServices, entry)
	}

	spec[ilKeyMultiClusterServices] = mcServices
}

// setIngressLinkAddress sets the virtual server's address, either statically or by the IPAM label it
// is allocated from. The API requires exactly one of the two, so only one is ever set.
func setIngressLinkAddress(spec map[string]any, ilCfg *ngfAPIv1alpha1.GatewayLinkConfig) {
	if ilCfg.VirtualServerAddress != nil {
		spec[ilKeyVirtualServerAddress] = *ilCfg.VirtualServerAddress
	}
	if ilCfg.IPAMLabel != nil {
		spec[ilKeyIPAMLabel] = *ilCfg.IPAMLabel
	}
}

func setIngressLinkOptionalFields(spec map[string]any, ilCfg *ngfAPIv1alpha1.GatewayLinkConfig) {
	if ilCfg.VirtualServerName != nil {
		spec[ilKeyVirtualServerName] = *ilCfg.VirtualServerName
	}
	if ilCfg.Host != nil {
		spec[ilKeyHost] = *ilCfg.Host
	}
	if len(ilCfg.IRules) > 0 {
		spec[ilKeyIRules] = toJSONSlice(ilCfg.IRules)
	}
	if ilCfg.Partition != nil {
		spec[ilKeyPartition] = *ilCfg.Partition
	}
	if ilCfg.BigIPRouteDomain != nil {
		spec[ilKeyBigIPRouteDomain] = *ilCfg.BigIPRouteDomain
	}
	if len(ilCfg.Monitors) > 0 {
		monitors := make([]any, 0, len(ilCfg.Monitors))
		for _, m := range ilCfg.Monitors {
			monitors = append(monitors, map[string]any{
				ilKeyName:      m.Name,
				ilKeyReference: m.Reference,
			})
		}
		spec[ilKeyMonitors] = monitors
	}
}

func setIngressLinkTLS(spec map[string]any, ilCfg *ngfAPIv1alpha1.GatewayLinkConfig) {
	if ilCfg.TLS == nil {
		return
	}

	tls := map[string]any{}
	if len(ilCfg.TLS.ClientSSLs) > 0 {
		tls[ilKeyClientSSLs] = toJSONSlice(ilCfg.TLS.ClientSSLs)
	}
	if len(ilCfg.TLS.ServerSSLs) > 0 {
		tls[ilKeyServerSSLs] = toJSONSlice(ilCfg.TLS.ServerSSLs)
	}
	if ilCfg.TLS.Reference != nil {
		tls[ilKeyReference] = string(*ilCfg.TLS.Reference)
	}
	if len(tls) > 0 {
		spec[ilKeyTLS] = tls
	}
}

func setIngressLinkServiceAddress(spec map[string]any, ilCfg *ngfAPIv1alpha1.GatewayLinkConfig) {
	if ilCfg.ServiceAddress == nil {
		return
	}

	serviceAddress := map[string]any{}
	if ilCfg.ServiceAddress.ICMPEcho != nil {
		serviceAddress[ilKeyICMPEcho] = string(*ilCfg.ServiceAddress.ICMPEcho)
	}
	if ilCfg.ServiceAddress.TrafficGroup != nil {
		serviceAddress[ilKeyTrafficGroup] = *ilCfg.ServiceAddress.TrafficGroup
	}
	if len(serviceAddress) > 0 {
		spec[ilKeyServiceAddress] = serviceAddress
	}
}
