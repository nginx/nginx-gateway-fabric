package graph

import (
	"encoding/json"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
)

// ExternalLoadBalancer represents a processed ExternalLoadBalancer, carrying the
// conditions to report in its status.
type ExternalLoadBalancer struct {
	// Source is the ExternalLoadBalancer.
	Source *ngfAPIv1alpha1.ExternalLoadBalancer
	// Conditions define the conditions to report in the status of the ExternalLoadBalancer.
	Conditions []conditions.Condition
	// Valid indicates whether the ExternalLoadBalancer is accepted and attached to its Gateway.
	Valid bool
}

// processExternalLoadBalancers validates each ExternalLoadBalancer and attaches the accepted one to
// the Gateway its targetRef resolves to. A Gateway yields one data plane Service, so it is fronted by
// at most one ExternalLoadBalancer: when several target the same Gateway the oldest is accepted and
// the rest are Conflicted.
func processExternalLoadBalancers(
	elbs map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer,
	gws map[types.NamespacedName]*Gateway,
) map[types.NamespacedName]*ExternalLoadBalancer {
	if len(elbs) == 0 {
		return nil
	}

	// Sorted so that the oldest consistently wins, with namespace/name breaking ties.
	ordered := make([]*ngfAPIv1alpha1.ExternalLoadBalancer, 0, len(elbs))
	for _, elb := range elbs {
		ordered = append(ordered, elb)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ngfsort.LessObjectMeta(&ordered[i].ObjectMeta, &ordered[j].ObjectMeta)
	})

	processed := make(map[types.NamespacedName]*ExternalLoadBalancer, len(ordered))
	for _, elb := range ordered {
		nsName := types.NamespacedName{Namespace: elb.Namespace, Name: elb.Name}

		if msg := validateAdditionalSpec(elb); msg != "" {
			processed[nsName] = &ExternalLoadBalancer{
				Source:     elb,
				Valid:      false,
				Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerInvalid(msg)},
			}
			continue
		}

		gwNsName := types.NamespacedName{Namespace: elb.Namespace, Name: string(elb.Spec.TargetRefs[0].Name)}
		gw, exists := gws[gwNsName]

		switch {
		case !exists:
			processed[nsName] = &ExternalLoadBalancer{
				Source: elb,
				Valid:  false,
				Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerInvalid(
					"targetRef references a Gateway that does not exist",
				)},
			}
		case gw.ExternalLoadBalancer == nil:
			gw.ExternalLoadBalancer = elb
			processed[nsName] = &ExternalLoadBalancer{
				Source:     elb,
				Valid:      true,
				Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerAccepted()},
			}
		default:
			processed[nsName] = &ExternalLoadBalancer{
				Source: elb,
				Valid:  false,
				Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerConflicted(fmt.Sprintf(
					"Gateway %q is already fronted by ExternalLoadBalancer %q",
					gwNsName.String(), gw.ExternalLoadBalancer.Name,
				))},
			}
		}
	}

	return processed
}

// validateAdditionalSpec returns a message when the additionalIngressLinkSpec escape hatch sets a
// Common partition, or an empty string otherwise. The modeled partition field is guarded by CEL, but
// the escape hatch preserves unknown fields and so can be used to set a Common partition.
func validateAdditionalSpec(elb *ngfAPIv1alpha1.ExternalLoadBalancer) string {
	if elb.Spec.GatewayLink == nil || elb.Spec.GatewayLink.AdditionalIngressLinkSpec == nil {
		return ""
	}

	raw := elb.Spec.GatewayLink.AdditionalIngressLinkSpec.Raw
	if len(raw) == 0 {
		return ""
	}

	var m struct {
		Partition string `json:"partition"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

	if m.Partition == "Common" {
		return "additionalIngressLinkSpec.partition cannot be Common"
	}

	return ""
}
