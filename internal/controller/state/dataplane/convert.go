package dataplane

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/mirror"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func convertMatch(m v1.HTTPRouteMatch) Match {
	match := Match{}

	if m.Method != nil {
		method := string(*m.Method)
		match.Method = &method
	}

	if len(m.Headers) != 0 {
		match.Headers = make([]HTTPHeaderMatch, 0, len(m.Headers))
		for _, h := range m.Headers {
			match.Headers = append(match.Headers, HTTPHeaderMatch{
				Name:  string(h.Name),
				Value: h.Value,
				Type:  convertMatchType(h.Type),
			})
		}
	}

	if len(m.QueryParams) != 0 {
		match.QueryParams = make([]HTTPQueryParamMatch, 0, len(m.QueryParams))
		for _, q := range m.QueryParams {
			match.QueryParams = append(match.QueryParams, HTTPQueryParamMatch{
				Name:  string(q.Name),
				Value: q.Value,
				Type:  convertMatchType(q.Type),
			})
		}
	}

	return match
}

func convertHTTPRequestRedirectFilter(filter *v1.HTTPRequestRedirectFilter) *HTTPRequestRedirectFilter {
	return &HTTPRequestRedirectFilter{
		Scheme:     filter.Scheme,
		Hostname:   (*string)(filter.Hostname),
		Port:       filter.Port,
		StatusCode: filter.StatusCode,
		Path:       convertPathModifier(filter.Path),
	}
}

func convertHTTPURLRewriteFilter(filter *v1.HTTPURLRewriteFilter) *HTTPURLRewriteFilter {
	return &HTTPURLRewriteFilter{
		Hostname: (*string)(filter.Hostname),
		Path:     convertPathModifier(filter.Path),
	}
}

func convertHTTPRequestMirrorFilter(
	filter *v1.HTTPRequestMirrorFilter,
	ruleIdx int,
	routeNsName types.NamespacedName,
) *HTTPRequestMirrorFilter {
	if filter.BackendRef.Name == "" {
		return &HTTPRequestMirrorFilter{}
	}

	result := &HTTPRequestMirrorFilter{
		Name: helpers.GetPointer(string(filter.BackendRef.Name)),
	}

	namespace := (*string)(filter.BackendRef.Namespace)
	if namespace != nil && len(*namespace) > 0 {
		result.Namespace = namespace
	}

	result.Target = mirror.BackendPath(ruleIdx, namespace, *result.Name, routeNsName)
	switch {
	case filter.Percent != nil:
		result.Percent = helpers.GetPointer(float64(*filter.Percent))
	case filter.Fraction != nil:
		denominator := int32(100)
		if filter.Fraction.Denominator != nil {
			denominator = *filter.Fraction.Denominator
		}
		result.Percent = helpers.GetPointer(float64(filter.Fraction.Numerator*100) / float64(denominator))
	default:
		result.Percent = helpers.GetPointer(float64(100))
	}

	if *result.Percent > 100.0 {
		result.Percent = helpers.GetPointer(100.0)
	}

	return result
}

func convertHTTPHeaderFilter(filter *v1.HTTPHeaderFilter) *HTTPHeaderFilter {
	result := &HTTPHeaderFilter{
		Remove: filter.Remove,
	}

	if len(filter.Set) != 0 {
		result.Set = make([]HTTPHeader, 0, len(filter.Set))
		for _, s := range filter.Set {
			result.Set = append(result.Set, HTTPHeader{Name: string(s.Name), Value: s.Value})
		}
	}

	if len(filter.Add) != 0 {
		result.Add = make([]HTTPHeader, 0, len(filter.Add))
		for _, a := range filter.Add {
			result.Add = append(result.Add, HTTPHeader{Name: string(a.Name), Value: a.Value})
		}
	}

	return result
}

func convertPathType(pathType v1.PathMatchType) PathType {
	switch pathType {
	case v1.PathMatchPathPrefix:
		return PathTypePrefix
	case v1.PathMatchExact:
		return PathTypeExact
	case v1.PathMatchRegularExpression:
		return PathTypeRegularExpression
	default:
		panic(fmt.Sprintf("unsupported path type: %s", pathType))
	}
}

func convertMatchType[T ~string](matchType *T) MatchType {
	switch *matchType {
	case T(v1.HeaderMatchExact), T(v1.QueryParamMatchExact):
		return MatchTypeExact
	case T(v1.HeaderMatchRegularExpression), T(v1.QueryParamMatchRegularExpression):
		return MatchTypeRegularExpression
	default:
		panic(fmt.Sprintf("unsupported match type: %v", *matchType))
	}
}

func convertPathModifier(path *v1.HTTPPathModifier) *HTTPPathModifier {
	if path != nil {
		switch path.Type {
		case v1.FullPathHTTPPathModifier:
			return &HTTPPathModifier{
				Type:        ReplaceFullPath,
				Replacement: *path.ReplaceFullPath,
			}
		case v1.PrefixMatchHTTPPathModifier:
			return &HTTPPathModifier{
				Type:        ReplacePrefixMatch,
				Replacement: *path.ReplacePrefixMatch,
			}
		}
	}

	return nil
}

func convertSnippetsFilter(filter *graph.SnippetsFilter) SnippetsFilter {
	result := SnippetsFilter{}

	if snippet, ok := filter.Snippets[ngfAPI.NginxContextHTTPServer]; ok {
		result.ServerSnippet = &Snippet{
			Name:     createSnippetName(ngfAPI.NginxContextHTTPServer, client.ObjectKeyFromObject(filter.Source)),
			Contents: snippet,
		}
	}

	if snippet, ok := filter.Snippets[ngfAPI.NginxContextHTTPServerLocation]; ok {
		result.LocationSnippet = &Snippet{
			Name: createSnippetName(
				ngfAPI.NginxContextHTTPServerLocation,
				client.ObjectKeyFromObject(filter.Source),
			),
			Contents: snippet,
		}
	}

	return result
}

func convertAuthenticationFilter(
	filter *graph.AuthenticationFilter,
	referencedSecrets map[types.NamespacedName]*secrets.Secret,
) *AuthenticationFilter {
	result := &AuthenticationFilter{}

	// Do not convert invalid filters; graph validation will have emitted a condition.
	if filter == nil || !filter.Valid {
		return result
	}

	if specBasic := filter.Source.Spec.Basic; specBasic != nil {
		referencedSecret, isReferenced := referencedSecrets[types.NamespacedName{
			Namespace: filter.Source.Namespace,
			Name:      specBasic.SecretRef.Name,
		}]

		if isReferenced && referencedSecret.Source != nil {
			result.Basic = &AuthBasic{
				SecretName:      specBasic.SecretRef.Name,
				SecretNamespace: referencedSecret.Source.Namespace,
				Data:            referencedSecret.Source.Data[secrets.AuthKey],
				Realm:           specBasic.Realm,
			}
		}
	}

	return result
}

func convertDNSResolverAddresses(addresses []ngfAPIv1alpha2.DNSResolverAddress) []string {
	if len(addresses) == 0 {
		return nil
	}

	result := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		result = append(result, addr.Value)
	}

	return result
}
