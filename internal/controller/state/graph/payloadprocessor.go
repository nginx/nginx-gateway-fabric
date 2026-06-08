package graph

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
)

// PayloadProcessingOutput contains payload processor resolution output.
type PayloadProcessingOutput struct {
	// ReferencedPayloadProcessorSecrets contains Secrets referenced by PayloadProcessor (auth token).
	ReferencedPayloadProcessorSecrets map[types.NamespacedName]*corev1.Secret
}

// processPayloadProcessorPolicies resolves ExtProc backend Service (including ExternalName) and
// optional auth token Secret for PayloadProcessor policies. Resolved information is stored on the
// Policy.PayloadProcessorState and referenced secrets are returned so the change tracker can watch them.
func processPayloadProcessorPolicies(
	processedPolicies map[PolicyKey]*Policy,
	allServices map[types.NamespacedName]*corev1.Service,
	allSecrets map[types.NamespacedName]*corev1.Secret,
) *PayloadProcessingOutput {
	output := &PayloadProcessingOutput{
		ReferencedPayloadProcessorSecrets: make(map[types.NamespacedName]*corev1.Secret),
	}

	for key, policy := range processedPolicies {
		if key.GVK.Kind != "PayloadProcessor" {
			continue
		}

		if !policy.Valid {
			continue
		}

		pp, ok := policy.Source.(*ngfAPIv1alpha1.PayloadProcessor)
		if !ok {
			continue
		}

		// Only support ExtProc processors for now. If none present, skip.
		var ext *ngfAPIv1alpha1.ExtProcConfig
		for _, p := range pp.Spec.Processors {
			if p.ExtProc != nil {
				ext = p.ExtProc
				break
			}
		}
		if ext == nil {
			continue
		}

		// Resolve backend service
		svcNs := types.NamespacedName{Namespace: pp.Namespace, Name: string(ext.BackendRef.Name)}
		var apiURL string
		if svc, exists := allServices[svcNs]; exists {
			if svc.Spec.Type == corev1.ServiceTypeExternalName && svc.Spec.ExternalName != "" {
				apiURL = fmt.Sprintf("https://%s:%d", svc.Spec.ExternalName, ext.Port)
			} else {
				apiURL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcNs.Name, svcNs.Namespace, ext.Port)
			}
		} else {
			// Service not found — mark policy invalid for now
			cond := conditions.Condition{
				Type:    string(conditions.WAFResolvedRefsConditionType),
				Status:  metav1.ConditionFalse,
				Reason:  string(conditions.PolicyReasonInvalidRef),
				Message: fmt.Sprintf("backend Service %s/%s not found", svcNs.Namespace, svcNs.Name),
			}
			policy.Conditions = append(policy.Conditions, cond)
			policy.Valid = false
			continue
		}

		// Resolve auth token secret if present
		var apiToken string
		var tokenSecretNs *types.NamespacedName
		if ext.AuthTokenRef != nil {
			secNs := types.NamespacedName{Namespace: pp.Namespace, Name: ext.AuthTokenRef.Name}
			sec, exists := allSecrets[secNs]
			if !exists {
				cond := conditions.Condition{
					Type:    string(conditions.WAFResolvedRefsConditionType),
					Status:  metav1.ConditionFalse,
					Reason:  string(conditions.PolicyReasonInvalidRef),
					Message: fmt.Sprintf("auth token Secret %s/%s not found", secNs.Namespace, secNs.Name),
				}
				policy.Conditions = append(policy.Conditions, cond)
				policy.Valid = false
				continue
			}

			// Expect key "token"
			if data, ok := sec.Data["token"]; ok {
				apiToken = strings.TrimSpace(string(data))
				if apiToken == "" {
					cond := conditions.Condition{
						Type:    string(conditions.WAFResolvedRefsConditionType),
						Status:  metav1.ConditionFalse,
						Reason:  string(conditions.PolicyReasonInvalidRef),
						Message: fmt.Sprintf("auth token Secret %s/%s missing token key or empty", secNs.Namespace, secNs.Name),
					}
					policy.Conditions = append(policy.Conditions, cond)
					policy.Valid = false
					continue
				}
				tokenSecretNs = &secNs
				output.ReferencedPayloadProcessorSecrets[secNs] = sec
			} else {
				cond := conditions.Condition{
					Type:    string(conditions.WAFResolvedRefsConditionType),
					Status:  metav1.ConditionFalse,
					Reason:  string(conditions.PolicyReasonInvalidRef),
					Message: fmt.Sprintf("auth token Secret %s/%s missing token key", secNs.Namespace, secNs.Name),
				}
				policy.Conditions = append(policy.Conditions, cond)
				policy.Valid = false
				continue
			}
		}

		// Populate policy state (only fields we can currently resolve)
		policy.PayloadProcessorState = &PolicyPayloadProcessorState{
			APIURL:           apiURL,
			APIToken:         apiToken,
			InspectMode:      "",
			TimeoutMS:        nil,
			MaxResponseBytes: nil,
			BackendService:   svcNs,
			APITokenSecret:   tokenSecretNs,
		}
	}

	return output
}
