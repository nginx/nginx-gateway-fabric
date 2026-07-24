package graph

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// guardrailsTokenSecretKey is the key within the AuthTokenRef Secret that holds the bearer token.
const guardrailsTokenSecretKey = "token"

// PolicyPayloadProcessorState holds resolved ExtProcess state for a PayloadProcessor Policy.
// This is only populated for PayloadProcessor resources.
type PolicyPayloadProcessorState struct {
	Timeout           *ngfAPIv1alpha1.Duration
	AuthTokenSecret   *types.NamespacedName
	BackendService    types.NamespacedName
	APIURL            string
	ResolvedAuthToken []byte
}

// PayloadProcessingOutput contains payload processor resolution output.
type PayloadProcessingOutput struct {
	// ReferencedPayloadProcessorSecrets contains Secrets referenced by PayloadProcessor policies
	// (auth token). These must be watched by the change tracker.
	ReferencedPayloadProcessorSecrets map[types.NamespacedName]*corev1.Secret
}

// processPayloadProcessorPolicies resolves the ExtProcess backend Service (including ExternalName)
// and optional auth token Secret for valid PayloadProcessor policies. Resolved information is stored
// on Policy.PayloadProcessorState and referenced secrets are returned so the change tracker can
// watch them. Policies whose references cannot be resolved are marked invalid.
func processPayloadProcessorPolicies(
	processedPolicies map[PolicyKey]*Policy,
	services map[types.NamespacedName]*corev1.Service,
	secrets map[types.NamespacedName]*corev1.Secret,
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

		resolvePayloadProcessor(pp, policy, services, secrets, output)
	}

	return output
}

// resolvePayloadProcessor resolves a single PayloadProcessor policy's ExtProcess backend Service and
// optional auth token Secret, populating policy.PayloadProcessorState or marking the policy invalid.
func resolvePayloadProcessor(
	pp *ngfAPIv1alpha1.PayloadProcessor,
	policy *Policy,
	services map[types.NamespacedName]*corev1.Service,
	secrets map[types.NamespacedName]*corev1.Secret,
	output *PayloadProcessingOutput,
) {
	// The API guarantees exactly one processor of type ExtProcess. Take the first ExtProcess entry.
	var entry *ngfAPIv1alpha1.PayloadProcessorEntry
	for i := range pp.Spec.Processors {
		if pp.Spec.Processors[i].ExtProcess != nil {
			entry = &pp.Spec.Processors[i]
			break
		}
	}
	if entry == nil {
		return
	}
	ext := entry.ExtProcess

	apiURL, err := resolveExtProcessURL(pp.Namespace, ext, services)
	if err != nil {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(err.Error()))
		policy.Valid = false
		return
	}

	token, tokenSecret, err := resolveExtProcessAuthToken(pp.Namespace, ext, secrets, output)
	if err != nil {
		policy.Conditions = append(policy.Conditions, conditions.NewPolicyInvalid(err.Error()))
		policy.Valid = false
		return
	}

	svcNsName := extProcessServiceNsName(pp.Namespace, ext)
	policy.PayloadProcessorState = &PolicyPayloadProcessorState{
		APIURL:            apiURL,
		ResolvedAuthToken: token,
		AuthTokenSecret:   tokenSecret,
		Timeout:           entry.Timeout,
		BackendService:    svcNsName,
	}
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

// resolveExtProcessURL resolves the backend Service into a URL the Rust module can call. ExternalName
// Services resolve to an https URL using the external hostname; all others resolve to the cluster-local
// Service DNS name over http.
func resolveExtProcessURL(
	policyNamespace string,
	ext *ngfAPIv1alpha1.ExtProcessConfig,
	services map[types.NamespacedName]*corev1.Service,
) (string, error) {
	svcNsName := extProcessServiceNsName(policyNamespace, ext)

	var port int32
	if ext.BackendRef.Port != nil {
		port = *ext.BackendRef.Port
	}

	svc, exists := services[svcNsName]
	if !exists {
		return "", fmt.Errorf(
			"backend Service %s/%s not found",
			svcNsName.Namespace,
			svcNsName.Name,
		)
	}

	if svc.Spec.Type == corev1.ServiceTypeExternalName && svc.Spec.ExternalName != "" {
		return fmt.Sprintf("https://%s:%d", svc.Spec.ExternalName, port), nil
	}

	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcNsName.Name, svcNsName.Namespace, port), nil
}

// resolveExtProcessAuthToken resolves the optional AuthTokenRef Secret into a bearer token. When no
// AuthTokenRef is set it returns nil values with no error. Referenced Secrets are recorded in output
// so the change tracker can watch them.
func resolveExtProcessAuthToken(
	policyNamespace string,
	ext *ngfAPIv1alpha1.ExtProcessConfig,
	secrets map[types.NamespacedName]*corev1.Secret,
	output *PayloadProcessingOutput,
) ([]byte, *types.NamespacedName, error) {
	if ext.AuthTokenRef == nil {
		return nil, nil, nil
	}

	secNsName := types.NamespacedName{Namespace: policyNamespace, Name: ext.AuthTokenRef.Name}
	sec, exists := secrets[secNsName]
	if !exists {
		return nil, nil, fmt.Errorf(
			"auth token Secret %s/%s not found",
			secNsName.Namespace,
			secNsName.Name,
		)
	}

	data, ok := sec.Data[guardrailsTokenSecretKey]
	if !ok {
		return nil, nil, fmt.Errorf(
			"auth token Secret %s/%s missing %q key",
			secNsName.Namespace,
			secNsName.Name,
			guardrailsTokenSecretKey,
		)
	}

	token := []byte(strings.TrimSpace(string(data)))
	if len(token) == 0 {
		return nil, nil, fmt.Errorf(
			"auth token Secret %s/%s has empty %q key",
			secNsName.Namespace,
			secNsName.Name,
			guardrailsTokenSecretKey,
		)
	}

	if output.ReferencedPayloadProcessorSecrets == nil {
		output.ReferencedPayloadProcessorSecrets = make(map[types.NamespacedName]*corev1.Secret)
	}
	output.ReferencedPayloadProcessorSecrets[secNsName] = sec

	return token, &secNsName, nil
}
