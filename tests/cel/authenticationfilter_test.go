package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestAuthenticationFilterValidateBasicAccepted(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=Basic with spec.basic set is accepted",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeBasic,
				Basic: &ngfAPIv1alpha1.BasicAuth{
					SecretRef: ngfAPIv1alpha1.LocalObjectReference{
						Name: uniqueResourceName("auth-secret"),
					},
					Realm: "Restricted Area",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authFilter := &ngfAPIv1alpha1.AuthenticationFilter{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}

			validateCrd(t, tt.wantErrors, authFilter, k8sClient)
		})
	}
}

func TestAuthenticationFilterValidateBasicRejected(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=Basic with basic unset is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type:  ngfAPIv1alpha1.AuthTypeBasic,
				Basic: nil,
			},
			wantErrors: []string{expectedBasicRequiredError},
		},
		{
			name: "Validate: type=Basic with spec.jwt set is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeBasic,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Source: ngfAPIv1alpha1.JWTKeySourceFile,
					File: &ngfAPIv1alpha1.JWTFileKeySource{
						SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: uniqueResourceName("jwt-secret")},
					},
					Realm: "Restricted Area",
				},
			},
			wantErrors: []string{expectedBasicRequiredError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authFilter := &ngfAPIv1alpha1.AuthenticationFilter{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}

			validateCrd(t, tt.wantErrors, authFilter, k8sClient)
		})
	}
}

func TestAuthenticationFilterValidateJWTAccepted(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=JWT with source=File and spec.jwt.file set is accepted",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceFile,
					File: &ngfAPIv1alpha1.JWTFileKeySource{
						SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: uniqueResourceName("jwt-secret")},
					},
				},
			},
		},
		{
			name: "Validate: type=JWT with source=Remote and spec.jwt.remote set is accepted with HTTP protocol",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceRemote,
					Remote: &ngfAPIv1alpha1.JWTRemoteKeySource{
						URI: "http://issuer.example.com/well-known/jwks.json",
					},
				},
			},
		},
		{
			name: "Validate: type=JWT with source=Remote and spec.jwt.remote set is accepted with HTTPS protocol",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceRemote,
					Remote: &ngfAPIv1alpha1.JWTRemoteKeySource{
						URI: "https://keycloak.keycloak.svc.cluster.local:8080/realms/ngf/.well-known/openid-configuration",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authFilter := &ngfAPIv1alpha1.AuthenticationFilter{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}

			validateCrd(t, tt.wantErrors, authFilter, k8sClient)
		})
	}
}

func TestAuthenticationFilterValidateJWTRejected(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=JWT with spec.jwt unset is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT:  nil,
			},
			wantErrors: []string{expectedJWTRequiredError},
		},
		{
			name: "Validate: type=JWT with spec.basic set is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				Basic: &ngfAPIv1alpha1.BasicAuth{
					SecretRef: ngfAPIv1alpha1.LocalObjectReference{
						Name: uniqueResourceName("auth-secret"),
					},
					Realm: "Restricted Area",
				},
			},
			wantErrors: []string{expectedJWTRequiredError},
		},
		{
			name: "Validate: type=JWT with source=File and spec.jwt.file unset is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceFile,
					File:   nil,
				},
			},
			wantErrors: []string{expectedJWTFileRequiredError},
		},
		{
			name: "Validate: type=JWT with source=Remote and spec.jwt.remote unset is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceRemote,
					Remote: nil,
				},
			},
			wantErrors: []string{expectedJWTRemoteRequiredError},
		},
		{
			name: "Validate: type=JWT with source=File and spec.jwt.remote set is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceFile,
					Remote: &ngfAPIv1alpha1.JWTRemoteKeySource{
						URI: "https://issuer.example.com/.well-known/jwks.json",
					},
				},
			},
			wantErrors: []string{expectedJWTFileRequiredError},
		},
		{
			name: "Validate: type=JWT with source=Remote and spec.jwt.file set is rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Source: ngfAPIv1alpha1.JWTKeySourceRemote,
					File: &ngfAPIv1alpha1.JWTFileKeySource{
						SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: uniqueResourceName("jwt-secret")},
					},
				},
			},
			wantErrors: []string{expectedJWTRemoteRequiredError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authFilter := &ngfAPIv1alpha1.AuthenticationFilter{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}

			validateCrd(t, tt.wantErrors, authFilter, k8sClient)
		})
	}
}
