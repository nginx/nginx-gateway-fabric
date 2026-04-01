package provisioner

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceSpecSetter_PreservesExternalAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		existingAnnotations map[string]string
		desiredAnnotations  map[string]string
		expectedAnnotations map[string]string
		name                string
	}{
		{
			name: "preserves MetalLB annotations while adding NGF annotations",
			existingAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool": "production-public-ips",
				"metallb.universe.tf/loadBalancerIPs":        "192.168.1.100",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "from-gateway-infrastructure",
			},
			expectedAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool":         "production-public-ips",
				"metallb.universe.tf/loadBalancerIPs":                "192.168.1.100",
				"custom.annotation":                                  "from-gateway-infrastructure",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "NGF annotations take precedence on conflicts",
			existingAnnotations: map[string]string{
				"custom.annotation":                "old-value",
				"metallb.universe.tf/address-pool": "staging",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "new-value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation":                                  "new-value",
				"metallb.universe.tf/address-pool":                   "staging",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name:                "creates new service with annotations",
			existingAnnotations: nil,
			desiredAnnotations: map[string]string{
				"custom.annotation": "value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation": "value",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "removes NGF-managed annotations when no longer desired",
			existingAnnotations: map[string]string{
				"custom.annotation":                                  "should-be-removed",
				"metallb.universe.tf/ip-allocated-from-pool":         "production",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
			desiredAnnotations: map[string]string{},
			expectedAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool": "production",
			},
		},
		{
			name: "preserves cloud provider annotations",
			existingAnnotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type":   "nlb",
				"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "from-nginxproxy-patch",
			},
			expectedAnnotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type":   "nlb",
				"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
				"custom.annotation": "from-nginxproxy-patch",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "updates tracking annotation when managed keys change",
			existingAnnotations: map[string]string{
				"annotation-to-keep":                                 "value1",
				"annotation-to-remove":                               "value2",
				"metallb.universe.tf/address-pool":                   "production",
				"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep,annotation-to-remove",
			},
			desiredAnnotations: map[string]string{
				"annotation-to-keep": "value1",
			},
			expectedAnnotations: map[string]string{
				"annotation-to-keep":                                 "value1",
				"metallb.universe.tf/address-pool":                   "production",
				"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Create existing service with annotations
			existingService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-service",
					Namespace:   "default",
					Annotations: tt.existingAnnotations,
				},
			}

			// Create desired object metadata with NGF-managed annotations
			desiredMeta := metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "nginx-gateway",
				},
				Annotations: tt.desiredAnnotations,
			}

			// Create desired TypeMeta
			desiredTypeMeta := metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			}

			// Create desired spec
			desiredSpec := corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:     "http",
						Port:     80,
						Protocol: corev1.ProtocolTCP,
					},
				},
			}

			// Execute the setter
			setter := serviceSpecSetter(existingService, desiredSpec, desiredMeta, desiredTypeMeta)
			err := setter()

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(existingService.Annotations).To(Equal(tt.expectedAnnotations))
			g.Expect(existingService.Labels).To(Equal(desiredMeta.Labels))
			g.Expect(existingService.Spec).To(Equal(desiredSpec))
			g.Expect(existingService.TypeMeta).To(Equal(desiredTypeMeta))
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }

func TestDeploymentAndDaemonSetSpecSetter(t *testing.T) {
	t.Parallel()

	type testCase struct {
		existingAnnotations map[string]string
		desiredAnnotations  map[string]string
		expectedAnnotations map[string]string
		name                string
	}

	tests := []testCase{
		{
			name: "preserves external annotations while adding NGF annotations",
			existingAnnotations: map[string]string{
				"deployment.kubernetes.io/revision": "1",
				"field.cattle.io/publicEndpoints":   "192.61.0.19",
				"field.cattle.io/ports":             "80/tcp",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "from-ngf",
			},
			expectedAnnotations: map[string]string{
				"deployment.kubernetes.io/revision":                  "1",
				"field.cattle.io/publicEndpoints":                    "192.61.0.19",
				"field.cattle.io/ports":                              "80/tcp",
				"custom.annotation":                                  "from-ngf",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "preserves existing NGF-managed annotations when still desired",
			existingAnnotations: map[string]string{
				"custom.annotation":                                  "keep-me",
				"argocd.argoproj.io/sync-options":                    "Prune=false",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "keep-me",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation":                                  "keep-me",
				"argocd.argoproj.io/sync-options":                    "Prune=false",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "removes NGF-managed annotations when no longer desired",
			existingAnnotations: map[string]string{
				"custom.annotation":                                  "should-be-removed",
				"deployment.kubernetes.io/revision":                  "2",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
			desiredAnnotations: map[string]string{},
			expectedAnnotations: map[string]string{
				"deployment.kubernetes.io/revision": "2",
			},
		},
		{
			name: "NGF annotations take precedence on conflicts",
			existingAnnotations: map[string]string{
				"custom.annotation":                "old-value",
				"daemonSet.kubernetes.io/revision": "7",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "new-value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation":                                  "new-value",
				"daemonSet.kubernetes.io/revision":                   "7",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name:                "creates new deployment with annotations",
			existingAnnotations: nil,
			desiredAnnotations: map[string]string{
				"custom.annotation": "value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation": "value",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "updates tracking annotation when managed keys change",
			existingAnnotations: map[string]string{
				"annotation-to-keep":                                 "keep-value",
				"annotation-to-remove":                               "remove-value",
				"argocd.argoproj.io/sync-options":                    "Validate=true",
				"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep,annotation-to-remove",
			},
			desiredAnnotations: map[string]string{
				"annotation-to-keep": "updated-keep-value",
			},
			expectedAnnotations: map[string]string{
				"annotation-to-keep":                                 "updated-keep-value",
				"argocd.argoproj.io/sync-options":                    "Validate=true",
				"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep",
			},
		},
	}

	labels := map[string]string{
		"app.kubernetes.io/name":     "nginx-gateway-fabric",
		"app.kubernetes.io/instance": "nginx-gateway",
	}

	makeDesiredMeta := func(ann map[string]string) metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Labels:      labels,
			Annotations: ann,
		}
	}

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "nginx-gateway",
				Image: "nginx:1.25",
			}},
		},
	}

	runners := []struct {
		run  func(t *testing.T, tc testCase)
		name string
	}{
		{
			name: "Deployment",
			run: func(t *testing.T, tc testCase) {
				t.Helper()
				g := NewWithT(t)

				existing := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "nginx-gateway",
						Namespace:   "nginx-gateway",
						Annotations: tc.existingAnnotations,
					},
				}

				spec := appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{MatchLabels: labels},
					Template: podTemplate,
				}

				desiredTypeMeta := metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				}

				err := deploymentSpecSetter(existing, spec, makeDesiredMeta(tc.desiredAnnotations), desiredTypeMeta)()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(existing.Annotations).To(Equal(tc.expectedAnnotations))
				g.Expect(existing.Labels).To(Equal(labels))
				g.Expect(existing.Spec).To(Equal(spec))
				g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
			},
		},
		{
			name: "DaemonSet",
			run: func(t *testing.T, tc testCase) {
				t.Helper()
				g := NewWithT(t)

				existing := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "nginx-gateway",
						Namespace:   "nginx-gateway",
						Annotations: tc.existingAnnotations,
					},
				}

				spec := appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: labels},
					Template: podTemplate,
				}

				desiredTypeMeta := metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "DaemonSet",
				}

				err := daemonSetSpecSetter(existing, spec, makeDesiredMeta(tc.desiredAnnotations), desiredTypeMeta)()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(existing.Annotations).To(Equal(tc.expectedAnnotations))
				g.Expect(existing.Labels).To(Equal(labels))
				g.Expect(existing.Spec).To(Equal(spec))
				g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, r := range runners {
				t.Run(r.name, func(t *testing.T) {
					r.run(t, tc)
				})
			}
		})
	}
}

func TestHpaSpecSetter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	existing := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hpa",
			Namespace: "default",
		},
	}

	labels := map[string]string{
		"app": "nginx-gateway",
	}

	annotations := map[string]string{
		"custom.annotation": "test-value",
	}

	desiredMeta := metav1.ObjectMeta{
		Labels:      labels,
		Annotations: annotations,
	}

	minReplicas := int32(1)
	maxReplicas := int32(10)
	spec := autoscalingv2.HorizontalPodAutoscalerSpec{
		MinReplicas: &minReplicas,
		MaxReplicas: maxReplicas,
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "nginx-gateway",
		},
		Metrics: []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &[]int32{50}[0],
					},
				},
			},
		},
	}

	desiredTypeMeta := metav1.TypeMeta{
		APIVersion: "autoscaling/v2",
		Kind:       "HorizontalPodAutoscaler",
	}

	err := hpaSpecSetter(existing, spec, desiredMeta, desiredTypeMeta)()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(existing.Labels).To(Equal(labels))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Spec).To(Equal(spec))
	g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
}

func TestServiceAccountSpecSetter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	existing := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-account",
			Namespace: "default",
		},
	}

	labels := map[string]string{
		"app": "nginx-gateway",
	}

	annotations := map[string]string{
		"custom.annotation": "test-value",
	}

	desiredMeta := metav1.ObjectMeta{
		Labels:      labels,
		Annotations: annotations,
	}

	desiredTypeMeta := metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
	}

	err := serviceAccountSpecSetter(existing, desiredMeta, desiredTypeMeta)()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(existing.Labels).To(Equal(labels))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
}

func TestConfigMapSpecSetter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		existingData   map[string]string
		existingLabels map[string]string
		existingAnns   map[string]string
		desiredData    map[string]string
		desiredLabels  map[string]string
		desiredAnns    map[string]string
		name           string
		shouldUpdate   bool
	}{
		{
			name:           "updates when data differs",
			existingData:   map[string]string{"key1": "old-value"},
			existingLabels: map[string]string{"app": "nginx-gateway"},
			existingAnns:   map[string]string{"annotation": "value"},
			desiredData:    map[string]string{"key1": "new-value"},
			desiredLabels:  map[string]string{"app": "nginx-gateway"},
			desiredAnns:    map[string]string{"annotation": "value"},
			shouldUpdate:   true,
		},
		{
			name:           "updates when labels differ",
			existingData:   map[string]string{"key1": "value"},
			existingLabels: map[string]string{"app": "old-app"},
			existingAnns:   map[string]string{"annotation": "value"},
			desiredData:    map[string]string{"key1": "value"},
			desiredLabels:  map[string]string{"app": "nginx-gateway"},
			desiredAnns:    map[string]string{"annotation": "value"},
			shouldUpdate:   true,
		},
		{
			name:           "updates when annotations differ",
			existingData:   map[string]string{"key1": "value"},
			existingLabels: map[string]string{"app": "nginx-gateway"},
			existingAnns:   map[string]string{"annotation": "old-value"},
			desiredData:    map[string]string{"key1": "value"},
			desiredLabels:  map[string]string{"app": "nginx-gateway"},
			desiredAnns:    map[string]string{"annotation": "new-value"},
			shouldUpdate:   true,
		},
		{
			name:           "no update when everything matches",
			existingData:   map[string]string{"key1": "value"},
			existingLabels: map[string]string{"app": "nginx-gateway"},
			existingAnns:   map[string]string{"annotation": "value"},
			desiredData:    map[string]string{"key1": "value"},
			desiredLabels:  map[string]string{"app": "nginx-gateway"},
			desiredAnns:    map[string]string{"annotation": "value"},
			shouldUpdate:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-configmap",
					Namespace:   "default",
					Labels:      tt.existingLabels,
					Annotations: tt.existingAnns,
				},
				Data: tt.existingData,
			}

			originalData := make(map[string]string)
			for k, v := range existing.Data {
				originalData[k] = v
			}
			originalLabels := make(map[string]string)
			for k, v := range existing.Labels {
				originalLabels[k] = v
			}
			originalAnns := make(map[string]string)
			for k, v := range existing.Annotations {
				originalAnns[k] = v
			}

			desiredMeta := metav1.ObjectMeta{
				Labels:      tt.desiredLabels,
				Annotations: tt.desiredAnns,
			}

			desiredTypeMeta := metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			}

			err := configMapSpecSetter(existing, tt.desiredData, desiredMeta, desiredTypeMeta)()
			g.Expect(err).ToNot(HaveOccurred())

			if tt.shouldUpdate {
				g.Expect(existing.Labels).To(Equal(tt.desiredLabels))
				g.Expect(existing.Annotations).To(Equal(tt.desiredAnns))
				g.Expect(existing.Data).To(Equal(tt.desiredData))
				g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
			} else {
				g.Expect(existing.Data).To(Equal(originalData))
				g.Expect(existing.Labels).To(Equal(originalLabels))
				g.Expect(existing.Annotations).To(Equal(originalAnns))
				// TypeMeta should not be set when no update occurs
				g.Expect(existing.TypeMeta).To(Equal(metav1.TypeMeta{}))
			}
		})
	}
}

func TestSecretSpecSetter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	labels := map[string]string{
		"app": "nginx-gateway",
	}

	annotations := map[string]string{
		"custom.annotation": "test-value",
	}

	desiredMeta := metav1.ObjectMeta{
		Labels:      labels,
		Annotations: annotations,
	}

	data := map[string][]byte{
		"username": []byte("admin"),
		"password": []byte("secret"),
	}

	desiredTypeMeta := metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Secret",
	}

	err := secretSpecSetter(existing, data, desiredMeta, desiredTypeMeta)()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(existing.Labels).To(Equal(labels))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Data).To(Equal(data))
	g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
}

func TestRoleSpecSetter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	existing := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-role",
			Namespace: "default",
		},
	}

	labels := map[string]string{
		"app": "nginx-gateway",
	}

	annotations := map[string]string{
		"custom.annotation": "test-value",
	}

	desiredMeta := metav1.ObjectMeta{
		Labels:      labels,
		Annotations: annotations,
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
	}

	desiredTypeMeta := metav1.TypeMeta{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
	}

	err := roleSpecSetter(existing, rules, desiredMeta, desiredTypeMeta)()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(existing.Labels).To(Equal(labels))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Rules).To(Equal(rules))
	g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
}

func TestRoleBindingSpecSetter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	existing := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolebinding",
			Namespace: "default",
		},
	}

	labels := map[string]string{
		"app": "nginx-gateway",
	}

	annotations := map[string]string{
		"custom.annotation": "test-value",
	}

	desiredMeta := metav1.ObjectMeta{
		Labels:      labels,
		Annotations: annotations,
	}

	roleRef := rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     "nginx-gateway-role",
	}

	subjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "nginx-gateway",
			Namespace: "nginx-gateway",
		},
	}

	desiredTypeMeta := metav1.TypeMeta{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
	}

	err := roleBindingSpecSetter(existing, roleRef, subjects, desiredMeta, desiredTypeMeta)()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(existing.Labels).To(Equal(labels))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.RoleRef).To(Equal(roleRef))
	g.Expect(existing.Subjects).To(Equal(subjects))
	g.Expect(existing.TypeMeta).To(Equal(desiredTypeMeta))
}
