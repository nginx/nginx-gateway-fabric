package graph

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// PolicyPayloadProcessorState holds resolved ExtProcess state for a PayloadProcessor Policy.
// This is only populated for PayloadProcessor resources.
type PolicyPayloadProcessorState struct {
	AuthTokenSecret   *types.NamespacedName
	BackendTLSPolicy  *BackendTLSPolicy
	BackendService    types.NamespacedName
	APIURL            string
	ResolvedAuthToken []byte
	// BackendIsExternalName reports whether the resolved ExtProcess backend Service is an ExternalName
	// Service. Such backends are proxied to by hostname and require a DNS resolver on the attached
	// Gateway's NginxProxy so NGINX can re-resolve the hostname per request.
	BackendIsExternalName bool
}

// PayloadProcessingOutput contains payload processor resolution output.
type PayloadProcessingOutput struct {
	// ReferencedPayloadProcessorSecrets contains Secrets referenced by PayloadProcessor policies
	// (auth token). These must be watched by the change tracker.
	ReferencedPayloadProcessorSecrets map[types.NamespacedName]*corev1.Secret
	// ReferencedPayloadProcessorServices holds backend Service NsNames referenced by
	// PayloadProcessor policies, including missing ones, so the change tracker rebuilds
	// when they are created, deleted, or changed.
	ReferencedPayloadProcessorServices map[types.NamespacedName]struct{}
}

// trackPayloadProcessorService records a backend Service NsName in the output so the change tracker
// watches it. The Service is tracked even when it does not exist, so that a rebuild is triggered when
// the Service later appears.
func trackPayloadProcessorService(output *PayloadProcessingOutput, nsName types.NamespacedName) {
	if output.ReferencedPayloadProcessorServices == nil {
		output.ReferencedPayloadProcessorServices = make(map[types.NamespacedName]struct{})
	}
	output.ReferencedPayloadProcessorServices[nsName] = struct{}{}
}

// processPayloadProcessorPolicies resolves the ExtProcess backend Service (including ExternalName)
// and optional auth token Secret for valid PayloadProcessor policies. Resolved information is stored
// on Policy.PayloadProcessorState and referenced secrets are returned so the change tracker can
// watch them. Policies whose references cannot be resolved are marked invalid.
func processPayloadProcessorPolicies(
	processedPolicies map[PolicyKey]*Policy,
	services map[types.NamespacedName]*corev1.Service,
	clusterSecrets map[types.NamespacedName]*corev1.Secret,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	clusterDomain string,
) *PayloadProcessingOutput {
	output := &PayloadProcessingOutput{}

	for _, policy := range processedPolicies {
		if !policy.Valid || getPolicyKind(policy.Source) != kinds.PayloadProcessor {
			continue
		}

		pp, ok := policy.Source.(*ngfAPIv1alpha1.PayloadProcessor)
		if !ok {
			continue
		}

		resolvePayloadProcessor(pp, policy, services, clusterSecrets, backendTLSPolicies, clusterDomain, output)
	}

	return output
}

// resolvePayloadProcessor resolves a single PayloadProcessor policy's ExtProcess backend Service and
// optional auth token Secret, populating policy.PayloadProcessorState or marking the policy invalid.
func resolvePayloadProcessor(
	pp *ngfAPIv1alpha1.PayloadProcessor,
	policy *Policy,
	services map[types.NamespacedName]*corev1.Service,
	clusterSecrets map[types.NamespacedName]*corev1.Secret,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	clusterDomain string,
	output *PayloadProcessingOutput,
) {
	// The API guarantees exactly one processor of type ExtProcess. Take the first ExtProcess entry.
	var entry *ngfAPIv1alpha1.PayloadProcessorEntry
	for _, processor := range pp.Spec.Processors {
		if processor.ExtProcess != nil {
			entry = &processor
			break
		}
	}
	if entry == nil {
		return
	}
	ext := entry.ExtProcess

	// Track the backend Service before resolution so a rebuild is triggered when the Service is
	// created, deleted, or changed, even if resolution below fails (e.g. the Service is missing).
	svcNsName := extProcessServiceNsName(pp.Namespace, ext)
	trackPayloadProcessorService(output, svcNsName)

	// Resolve the backend Service and its port once, up front. Both the URL and BackendTLSPolicy
	// resolvers need the same resolved port, so validating it here avoids a duplicate lookup (and a
	// duplicate error branch) in each of them.
	svc, exists := services[svcNsName]
	if !exists {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(
			fmt.Sprintf("backend Service %s/%s not found", svcNsName.Namespace, svcNsName.Name),
		))
		policy.Valid = false
		return
	}
	if ext.BackendRef.Port == nil {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(
			fmt.Sprintf("backend Service %s/%s port is not set", svcNsName.Namespace, svcNsName.Name),
		))
		policy.Valid = false
		return
	}
	svcPort, err := getServicePort(svc, *ext.BackendRef.Port)
	if err != nil {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(
			fmt.Sprintf("backend Service %s/%s: %s", svcNsName.Namespace, svcNsName.Name, err.Error()),
		))
		policy.Valid = false
		return
	}

	backendTLS, err := resolveExtProcessBackendTLS(pp.Namespace, ext, backendTLSPolicies, svcPort)
	if err != nil {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(err.Error()))
		policy.Valid = false
		return
	}

	apiURL, isExternalName := resolveExtProcessURL(svcNsName, svc, *ext.BackendRef.Port, clusterDomain)

	token, tokenSecret, err := resolveExtProcessAuthToken(pp.Namespace, ext, clusterSecrets, output)
	if err != nil {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(err.Error()))
		policy.Valid = false
		return
	}

	policy.PayloadProcessorState = &PolicyPayloadProcessorState{
		APIURL:                apiURL,
		ResolvedAuthToken:     token,
		AuthTokenSecret:       tokenSecret,
		BackendService:        svcNsName,
		BackendTLSPolicy:      backendTLS,
		BackendIsExternalName: isExternalName,
	}
}

// resolveExtProcessBackendTLS finds the BackendTLSPolicy targeting the ExtProcess backend Service and
// port, if any. The caller (resolvePayloadProcessor) has already resolved and validated the Service
// port, which is passed in as svcPort. It uses the pure selector (no status side effects) and then
// records Accepted/IsReferenced status on the winner idempotently, so a policy that is only referenced
// by this guardrails backend still gets status, while a policy also referenced by a Route backend is
// not marked twice. Conflict status is owned by the Route backend path and is intentionally not
// recorded here. A non-nil error (invalid winning policy) causes the PayloadProcessor to fail closed.
func resolveExtProcessBackendTLS(
	policyNamespace string,
	ext *ngfAPIv1alpha1.ExtProcessConfig,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	svcPort corev1.ServicePort,
) (*BackendTLSPolicy, error) {
	if len(backendTLSPolicies) == 0 {
		return nil, nil //nolint:nilnil // no error, no policy
	}

	btp, losers, err := selectBackendTLSPolicyForService(
		backendTLSPolicies,
		ext.BackendRef.Namespace,
		string(ext.BackendRef.Name),
		policyNamespace,
		svcPort,
	)

	for _, conflicted := range losers {
		conflicted.IsReferenced = true
		conflicted.Conditions = append(
			conflicted.Conditions,
			conditions.NewPolicyConflicted(
				"Conflicts with another BackendTLSPolicy targeting the same Service",
			),
		)
	}

	if err != nil {
		return nil, err
	}

	markBackendTLSPolicyAccepted(btp)

	return btp, nil
}

// addPayloadProcessorBackendServicesToReferencedServices registers each valid PayloadProcessor
// policy's ExtProcess backend Service into referencedServices, associating it with the Gateways the
// policy attaches to. These Services are referenced only through the policy (not a Route backend), so
// buildReferencedServices does not include them; without this, a BackendTLSPolicy targeting a
// Guardrails backend Service would never be associated with the relevant Gateways and its TLS settings
// would be dropped during dataplane conversion (convertBackendTLS is gateway-scoped).
func addPayloadProcessorBackendServicesToReferencedServices(
	processedPolicies map[PolicyKey]*Policy,
	routes map[RouteKey]*L7Route,
	gateways map[types.NamespacedName]*Gateway,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*corev1.Service,
) map[types.NamespacedName]*ReferencedService {
	for _, policy := range processedPolicies {
		if !policy.Valid || getPolicyKind(policy.Source) != kinds.PayloadProcessor {
			continue
		}

		pp, ok := policy.Source.(*ngfAPIv1alpha1.PayloadProcessor)
		if !ok {
			continue
		}

		var entry *ngfAPIv1alpha1.PayloadProcessorEntry
		for _, processor := range pp.Spec.Processors {
			if processor.ExtProcess != nil {
				entry = &processor
				break
			}
		}
		if entry == nil {
			continue
		}

		gwNsNames := payloadProcessorGateways(policy, routes, gateways)
		if len(gwNsNames) == 0 {
			continue
		}

		svcNsName := extProcessServiceNsName(pp.Namespace, entry.ExtProcess)
		if referencedServices == nil {
			referencedServices = make(map[types.NamespacedName]*ReferencedService)
		}
		ensureReferencedService(svcNsName, referencedServices, services)
		for _, gwNsName := range gwNsNames {
			referencedServices[svcNsName].GatewayNsNames[gwNsName] = struct{}{}
		}
	}

	return referencedServices
}

// payloadProcessorGateways returns the Gateways a PayloadProcessor policy is effective for, derived
// from its target refs: a Gateway target contributes itself; a Route target contributes the Gateways
// that Route is attached to. Gateways the policy is invalid for are excluded.
func payloadProcessorGateways(
	policy *Policy,
	routes map[RouteKey]*L7Route,
	gateways map[types.NamespacedName]*Gateway,
) []types.NamespacedName {
	seen := make(map[types.NamespacedName]struct{})
	var result []types.NamespacedName

	add := func(gwNsName types.NamespacedName) {
		if _, invalid := policy.InvalidForGateways[gwNsName]; invalid {
			return
		}
		if _, ok := gateways[gwNsName]; !ok {
			return
		}
		if _, dup := seen[gwNsName]; dup {
			return
		}
		seen[gwNsName] = struct{}{}
		result = append(result, gwNsName)
	}

	for _, ref := range policy.TargetRefs {
		switch ref.Kind {
		case kinds.Gateway:
			add(ref.Nsname)
		case kinds.HTTPRoute, kinds.GRPCRoute:
			route, exists := routes[routeKeyForKind(ref.Kind, ref.Nsname)]
			if !exists {
				continue
			}
			for _, parentRef := range route.ParentRefs {
				if parentRef.Attachment == nil || !parentRef.Attachment.Attached {
					continue
				}
				add(parentRef.GatewayNsName)
			}
		}
	}

	return result
}

// extProcessServiceNsName returns the NamespacedName of the ExtProcess backend Service, honoring a
// cross-namespace BackendRef.Namespace when set.
func extProcessServiceNsName(
	policyNamespace string,
	ext *ngfAPIv1alpha1.ExtProcessConfig,
) types.NamespacedName {
	ns := policyNamespace
	if ext.BackendRef.Namespace != nil {
		ns = string(*ext.BackendRef.Namespace)
	}
	return types.NamespacedName{Namespace: ns, Name: string(ext.BackendRef.Name)}
}

// resolveExtProcessURL resolves the already-validated backend Service and port into a base URL the
// Rust module can call. The caller (resolvePayloadProcessor) is responsible for looking up the Service
// and validating the port, so this function does no lookup and returns no error.
//
// ExternalName Services resolve to an https URL using the external hostname (they are always fronted
// by a hostname-verified, system-trust TLS terminator, with no per-Gateway variance). A cluster-local
// (ClusterIP) Service resolves to its cluster DNS name over plaintext http here; the per-Gateway https
// upgrade (when a BackendTLSPolicy is effective for a given Gateway) is applied downstream in the
// dataplane layer (convertGraphGuardrails), which is the single source of truth for the per-Gateway
// TLS decision. Keeping the scheme decision out of this policy-scoped resolution avoids emitting https
// for a Gateway the BackendTLSPolicy is not effective for. The returned bool reports whether the
// backend is an ExternalName Service.
func resolveExtProcessURL(
	svcNsName types.NamespacedName,
	svc *corev1.Service,
	port int32,
	clusterDomain string,
) (string, bool) {
	if svc.Spec.Type == corev1.ServiceTypeExternalName && svc.Spec.ExternalName != "" {
		return fmt.Sprintf("https://%s:%d", svc.Spec.ExternalName, port), true
	}

	// Fall back to the default cluster domain when the controller flag is unset (e.g. older callers/tests).
	domain := clusterDomain
	if domain == "" {
		domain = "cluster.local"
	}

	// In-cluster backends resolve to plaintext http here. When a BackendTLSPolicy is attached to a
	// particular Gateway, the dataplane layer upgrades this base to https per Gateway (see
	// convertGraphGuardrails). This keeps the scheme correct for Gateways the policy is not attached to.
	return fmt.Sprintf("http://%s.%s.svc.%s:%d", svcNsName.Name, svcNsName.Namespace, domain, port), false
}

// resolveExtProcessAuthToken resolves the optional AuthTokenRef Secret into a bearer token. When no
// AuthTokenRef is set it returns nil values with no error. Referenced Secrets are recorded in output
// so the change tracker can watch them.
func resolveExtProcessAuthToken(
	policyNamespace string,
	ext *ngfAPIv1alpha1.ExtProcessConfig,
	clusterSecrets map[types.NamespacedName]*corev1.Secret,
	output *PayloadProcessingOutput,
) ([]byte, *types.NamespacedName, error) {
	if ext.AuthTokenRef == nil {
		return nil, nil, nil
	}

	if output.ReferencedPayloadProcessorSecrets == nil {
		output.ReferencedPayloadProcessorSecrets = make(map[types.NamespacedName]*corev1.Secret)
	}

	secNsName := types.NamespacedName{Namespace: policyNamespace, Name: ext.AuthTokenRef.Name}
	sec, exists := clusterSecrets[secNsName]
	// Track the secret even if there are errors so that a rebuild is triggered when the Secret appears.
	output.ReferencedPayloadProcessorSecrets[secNsName] = sec
	if !exists {
		return nil, nil, fmt.Errorf(
			"auth token Secret %s/%s not found",
			secNsName.Namespace,
			secNsName.Name,
		)
	}

	data, ok := sec.Data[secrets.GuardrailsTokenKey]
	if !ok {
		return nil, nil, fmt.Errorf(
			"auth token Secret %s/%s missing %q key",
			secNsName.Namespace,
			secNsName.Name,
			secrets.GuardrailsTokenKey,
		)
	}

	token := []byte(strings.TrimSpace(string(data)))
	if len(token) == 0 {
		return nil, nil, fmt.Errorf(
			"auth token Secret %s/%s has empty %q key",
			secNsName.Namespace,
			secNsName.Name,
			secrets.GuardrailsTokenKey,
		)
	}

	return token, &secNsName, nil
}
