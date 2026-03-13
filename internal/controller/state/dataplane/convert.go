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

	switch filter.Source.Spec.Type {
	case ngfAPI.AuthTypeBasic:
		result.Basic = convertAuthenticationFilterBasicAuth(filter, referencedSecrets)
	case ngfAPI.AuthTypeJWT:
		result.JWT = convertAuthenticationFilterJwtAuth(filter, referencedSecrets)
	}

	return result
}

func convertAuthenticationFilterBasicAuth(
	filter *graph.AuthenticationFilter,
	referencedSecrets map[types.NamespacedName]*secrets.Secret,
) *AuthBasic {
	var result *AuthBasic
	if specBasic := filter.Source.Spec.Basic; specBasic != nil {
		referencedSecret, isReferenced := referencedSecrets[types.NamespacedName{
			Namespace: filter.Source.Namespace,
			Name:      specBasic.SecretRef.Name,
		}]

		if isReferenced && referencedSecret.Source != nil {
			result = &AuthBasic{
				SecretName:      specBasic.SecretRef.Name,
				SecretNamespace: referencedSecret.Source.Namespace,
				Data:            referencedSecret.Source.Data[secrets.AuthKey],
				Realm:           specBasic.Realm,
			}
		}
	}

	return result
}

func convertAuthenticationFilterJwtAuth(
	filter *graph.AuthenticationFilter,
	referencedSecrets map[types.NamespacedName]*secrets.Secret,
) *AuthJWT {
	var result *AuthJWT
	if specJWT := filter.Source.Spec.JWT; specJWT != nil {
		if specJWT.Source == ngfAPI.JWTKeySourceFile && specJWT.File != nil {
			// Handle File-based JWT (local JWKS)
			referencedSecret, isReferenced := referencedSecrets[types.NamespacedName{
				Namespace: filter.Source.Namespace,
				Name:      specJWT.File.SecretRef.Name,
			}]

			if isReferenced && referencedSecret.Source != nil {
				result = &AuthJWT{
					SecretName:      specJWT.File.SecretRef.Name,
					SecretNamespace: referencedSecret.Source.Namespace,
					Data:            referencedSecret.Source.Data[secrets.AuthKey],
					Realm:           specJWT.Realm,
					KeyCache:        specJWT.KeyCache,
				}
			}
		} else if specJWT.Source == ngfAPI.JWTKeySourceRemote && specJWT.Remote != nil {
			// Handle Remote JWT (remote JWKS)
			remote := &AuthJWTRemote{
				URI: specJWT.Remote.URI,
			}

			// Handle TLS configuration for remote JWKS
			if specJWT.Remote.TLS != nil {
				remoteTLS := &AuthJWTRemoteTLS{
					Verify:  specJWT.Remote.TLS.Verify,
					SNI:     specJWT.Remote.TLS.SNI,
					SNIName: specJWT.Remote.TLS.SNIName,
				}

				// Handle TLS client certificate if SecretRef is provided
				if specJWT.Remote.TLS.SecretRef != nil {
					referencedSecret, isReferenced := referencedSecrets[types.NamespacedName{
						Namespace: filter.Source.Namespace,
						Name:      specJWT.Remote.TLS.SecretRef.Name,
					}]

					if isReferenced && referencedSecret.Source != nil {
						// Set the CertificatePath if the secret contains client cert/key
						if referencedSecret.Source.Data[secrets.TLSCertKey] != nil &&
							referencedSecret.Source.Data[secrets.TLSKeyKey] != nil {
							remoteTLS.CertificatePath = generateJWTRemoteTLSKeyPairID(
								filter.Source.Namespace,
								specJWT.Remote.TLS.SecretRef.Name,
							)
						}

						// Set the CACertBundlePath if the secret contains CA cert
						if referencedSecret.Source.Data[secrets.CAKey] != nil {
							remoteTLS.CACertBundlePath = generateJWTRemoteTLSCABundleID(
								filter.Source.Namespace,
								specJWT.Remote.TLS.SecretRef.Name,
							)
						}
					}
				}

				remote.TLS = remoteTLS
			}

			result = &AuthJWT{
				Realm:           specJWT.Realm,
				KeyCache:        specJWT.KeyCache,
				Remote:          remote,
				FilterNamespace: filter.Source.Namespace,
				FilterName:      filter.Source.Name,
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
