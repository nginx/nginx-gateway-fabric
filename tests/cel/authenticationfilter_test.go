package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestAuthenticationFilterBasicRequiredWhenTypeIsBasic(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		spec       ngfAPIv1alpha1.AuthenticationFilterSpec
		wantErrors []string
	}{
		{
			name: "Validate: type=Basic with basic set should be accepted",
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
			name: "Validate: type=Basic with basic unset should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type:  ngfAPIv1alpha1.AuthTypeBasic,
				Basic: nil,
			},
			wantErrors: []string{expectedBasicRequiredError},
		},
		{
			name: "Validate: type=Basic with missing basic should be rejected",
			spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
				Type: ngfAPIv1alpha1.AuthTypeBasic,
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
