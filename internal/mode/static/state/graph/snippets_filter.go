package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

// SnippetsFilter represents a ngfAPI.SnippetsFilter.
type SnippetsFilter struct {
	// Source is the SnippetsFilter.
	Source *ngfAPI.SnippetsFilter
	// Snippets stored as a map of nginx context to snippet value.
	Snippets map[ngfAPI.NginxContext]string
	// Conditions define the conditions to be reported in the status of the SnippetsFilter.
	Conditions []conditions.Condition
	// Valid indicates whether the SnippetsFilter is semantically and syntactically valid.
	Valid bool
	// Referenced indicates whether the SnippetsFilter is referenced by a Route.
	Referenced bool
}

// getSnippetsFilterResolverForNamespace returns a resolveExtRefFilter function.
// This function resolves a LocalObjectReference to a SnippetsFilter in the given namespace.
// If the SnippetsFilter exists, it is marked as referenced and returned as an ExtensionRefFilter.
func getSnippetsFilterResolverForNamespace(
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
	ns string,
) resolveExtRefFilter {
	return func(ref v1.LocalObjectReference) *ExtensionRefFilter {
		if len(snippetsFilters) == 0 {
			return nil
		}

		if ref.Group != ngfAPI.GroupName || ref.Kind != kinds.SnippetsFilter {
			return nil
		}

		sf := snippetsFilters[types.NamespacedName{Namespace: ns, Name: string(ref.Name)}]
		if sf == nil {
			return nil
		}

		sf.Referenced = true

		return &ExtensionRefFilter{SnippetsFilter: sf, Valid: sf.Valid}
	}
}

func processSnippetsFilters(
	snippetsFilters map[types.NamespacedName]*ngfAPI.SnippetsFilter,
) map[types.NamespacedName]*SnippetsFilter {
	if len(snippetsFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*SnippetsFilter)

	for nsname, sf := range snippetsFilters {
		if cond := validateSnippetsFilter(sf); cond != nil {
			processed[nsname] = &SnippetsFilter{
				Source:     sf,
				Conditions: []conditions.Condition{*cond},
				Valid:      false,
			}

			continue
		}

		processed[nsname] = &SnippetsFilter{
			Source:   sf,
			Valid:    true,
			Snippets: createSnippetsMap(sf.Spec.Snippets),
		}
	}

	return processed
}

func createSnippetsMap(snippets []ngfAPI.Snippet) map[ngfAPI.NginxContext]string {
	snippetsMap := make(map[ngfAPI.NginxContext]string)

	// snippets are already validated, so we can assume there's max one snippet per context.
	for _, snippet := range snippets {
		snippetsMap[snippet.Context] = snippet.Value
	}

	return snippetsMap
}

func validateSnippetsFilter(filter *ngfAPI.SnippetsFilter) *conditions.Condition {
	var allErrs field.ErrorList
	snippetsPath := field.NewPath("spec.snippets")

	if len(filter.Spec.Snippets) == 0 {
		cond := staticConds.NewSnippetsFilterInvalid(
			field.Required(snippetsPath, "at least one snippet must be provided").Error(),
		)
		return &cond
	}

	usedContexts := make(map[ngfAPI.NginxContext]struct{})

	for i, snippet := range filter.Spec.Snippets {
		valuePath := snippetsPath.Index(i).Child("value")
		if snippet.Value == "" {
			cond := staticConds.NewSnippetsFilterInvalid(
				field.Required(valuePath, "value cannot be empty").Error(),
			)

			return &cond
		}

		ctxPath := snippetsPath.Index(i).Child("context")

		switch snippet.Context {
		case ngfAPI.NginxContextMain,
			ngfAPI.NginxContextHTTP,
			ngfAPI.NginxContextHTTPServer,
			ngfAPI.NginxContextHTTPServerLocation:
		default:
			err := field.NotSupported(
				ctxPath,
				snippet.Context,
				[]ngfAPI.NginxContext{
					ngfAPI.NginxContextMain,
					ngfAPI.NginxContextHTTP,
					ngfAPI.NginxContextHTTPServer,
					ngfAPI.NginxContextHTTPServerLocation,
				},
			)

			allErrs = append(allErrs, err)
		}

		if _, ok := usedContexts[snippet.Context]; ok {
			allErrs = append(
				allErrs,
				field.Invalid(ctxPath, snippet.Context, "only one snippet is allowed per context"),
			)

			continue
		}

		usedContexts[snippet.Context] = struct{}{}
	}

	if allErrs != nil {
		cond := staticConds.NewSnippetsFilterInvalid(allErrs.ToAggregate().Error())
		return &cond
	}

	return nil
}
