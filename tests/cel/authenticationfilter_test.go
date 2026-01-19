package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestAuthenticationFilterBasicAuth(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=Basic with spec.basic set should be accepted",
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
		{
			name: "Validate: type=Basic with spec.basic unset should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type:  ngfAPIv1alpha1.AuthTypeBasic,
				Basic: nil,
			},
			wantErrors: []string{expectedBasicRequiredError},
		},
		{
			name: "Validate: type=Basic with spec.jwt set, should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeBasic,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm: "Restricted Area",
					Mode:  ngfAPIv1alpha1.JWTKeyModeFile,
				},
			},
			wantErrors: []string{expectedJWTMustBeUnsetForBasic},
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

func TestAuthenticationFilterJWTAuth(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=JWT with spec.jwt set should be accepted with mode=File",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm: "Restricted Area",
					Mode:  ngfAPIv1alpha1.JWTKeyModeFile,
					File:  &ngfAPIv1alpha1.JWTFileKeySource{SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: uniqueResourceName("jwks-secret")}},
				},
			},
		},
		{
			name: "Validate: type=JWT with spec.jwt set should be accepted with mode=Remote",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Mode:   ngfAPIv1alpha1.JWTKeyModeRemote,
					Remote: &ngfAPIv1alpha1.RemoteKeySource{URL: "https://example.com/.well-known/jwks.json"},
				},
			},
		},
		{
			name: "Validate: type=JWT with spec.jwt unset should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
			},
			wantErrors: []string{expectedJWTRequiredError},
		},
		{
			name: "Validate: type=JWT with spec.basic set, should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT:  nil,
				Basic: &ngfAPIv1alpha1.BasicAuth{
					SecretRef: ngfAPIv1alpha1.LocalObjectReference{
						Name: uniqueResourceName("auth-secret"),
					},
					Realm: "Restricted Area",
				},
			},
			wantErrors: []string{expectedBasicMustBeUnsetForJWT},
		},
		{
			name: "Validate: type=JWT with jwt.mode=File and jwt.file unset should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm: "Restricted Area",
					Mode:  ngfAPIv1alpha1.JWTKeyModeFile,
					File:  nil,
				},
			},
			wantErrors: []string{expectedFileMustBeUnsetForRemote},
		},
		{
			name: "Validate: type=JWT with jwt.mode=Remote and jwt.remote unset should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Mode:   ngfAPIv1alpha1.JWTKeyModeRemote,
					Remote: nil,
				},
			},
			wantErrors: []string{expectedRemoteMustBeUnsetForFile},
		},
		{
			name: "Validate: type=JWT with jwt.mode=File and jwt.remote set should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm:  "Restricted Area",
					Mode:   ngfAPIv1alpha1.JWTKeyModeFile,
					Remote: &ngfAPIv1alpha1.RemoteKeySource{URL: "https://example.com/.well-known/jwks.json"},
				},
			},
			wantErrors: []string{expectedFileMustBeUnsetForRemote},
		},
		{
			name: "Validate: type=JWT with jwt.mode=Remote and jwt.file set should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeJWT,
				JWT: &ngfAPIv1alpha1.JWTAuth{
					Realm: "Restricted Area",
					Mode:  ngfAPIv1alpha1.JWTKeyModeRemote,
					File:  &ngfAPIv1alpha1.JWTFileKeySource{SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: uniqueResourceName("jwks-secret")}},
				},
			},
			wantErrors: []string{expectedRemoteMustBeUnsetForFile},
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
