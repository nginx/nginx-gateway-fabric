package provisioner

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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

			err := serviceSpecSetter(existingService, desiredSpec, desiredMeta)()
			g.Expect(err).ToNot(HaveOccurred())

			// Object meta fields, ensure name and namespace didn't change
			g.Expect(existingService.Name).To(Equal("test-service"))
			g.Expect(existingService.Namespace).To(Equal("default"))
			g.Expect(existingService.Annotations).To(Equal(tt.expectedAnnotations))
			g.Expect(existingService.Labels).To(Equal(desiredMeta.Labels))

			g.Expect(existingService.Spec).To(Equal(desiredSpec))
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

				err := deploymentSpecSetter(existing, spec, makeDesiredMeta(tc.desiredAnnotations))()
				g.Expect(err).ToNot(HaveOccurred())

				// Object meta fields, ensure name and namespace didn't change
				g.Expect(existing.Name).To(Equal("nginx-gateway"))
				g.Expect(existing.Namespace).To(Equal("nginx-gateway"))
				g.Expect(existing.Annotations).To(Equal(tc.expectedAnnotations))
				g.Expect(existing.Labels).To(Equal(labels))

				g.Expect(existing.Spec).To(Equal(spec))
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

				err := daemonSetSpecSetter(existing, spec, makeDesiredMeta(tc.desiredAnnotations))()
				g.Expect(err).ToNot(HaveOccurred())

				// Object meta fields, ensure name and namespace didn't change
				g.Expect(existing.Name).To(Equal("nginx-gateway"))
				g.Expect(existing.Namespace).To(Equal("nginx-gateway"))
				g.Expect(existing.Annotations).To(Equal(tc.expectedAnnotations))
				g.Expect(existing.Labels).To(Equal(labels))

				g.Expect(existing.Spec).To(Equal(spec))
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

	err := hpaSpecSetter(existing, spec, desiredMeta)()
	g.Expect(err).ToNot(HaveOccurred())

	// Object meta fields, ensure name and namespace didn't change
	g.Expect(existing.Name).To(Equal("test-hpa"))
	g.Expect(existing.Namespace).To(Equal("default"))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Labels).To(Equal(labels))

	g.Expect(existing.Spec).To(Equal(spec))
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

	// Test with AutomountServiceAccountToken set to false
	automountToken := false

	err := serviceAccountSpecSetter(existing, &automountToken, desiredMeta)()
	g.Expect(err).ToNot(HaveOccurred())

	// Object meta fields, ensure name and namespace didn't change
	g.Expect(existing.Name).To(Equal("test-service-account"))
	g.Expect(existing.Namespace).To(Equal("default"))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Labels).To(Equal(labels))

	g.Expect(existing.AutomountServiceAccountToken).To(Equal(&automountToken))
}

func TestConfigMapSpecSetter(t *testing.T) {
	t.Parallel()

	ownerRef1 := metav1.OwnerReference{
		APIVersion:         "apps/v1",
		Kind:               "Deployment",
		Name:               "nginx-gateway",
		UID:                "12345",
		Controller:         helpers.GetPointer(true),
		BlockOwnerDeletion: helpers.GetPointer(true),
	}

	// testing that owner references are compared based on their content, not their memory address
	ownerRef1Copy := metav1.OwnerReference{
		APIVersion:         "apps/v1",
		Kind:               "Deployment",
		Name:               "nginx-gateway",
		UID:                "12345",
		Controller:         helpers.GetPointer(true),
		BlockOwnerDeletion: helpers.GetPointer(true),
	}

	ownerRef2 := metav1.OwnerReference{
		APIVersion:         "apps/v1",
		Kind:               "Deployment",
		Name:               "other-deployment",
		UID:                "67890",
		Controller:         helpers.GetPointer(true),
		BlockOwnerDeletion: helpers.GetPointer(true),
	}

	tests := []struct {
		existingData      map[string]string
		existingLabels    map[string]string
		existingAnns      map[string]string
		desiredData       map[string]string
		desiredLabels     map[string]string
		desiredAnns       map[string]string
		name              string
		existingOwnerRefs []metav1.OwnerReference
		desiredOwnerRefs  []metav1.OwnerReference
		shouldUpdate      bool
	}{
		{
			name:              "updates when data differs",
			existingData:      map[string]string{"key1": "old-value"},
			existingLabels:    map[string]string{"app": "nginx-gateway"},
			existingAnns:      map[string]string{"annotation": "value"},
			existingOwnerRefs: []metav1.OwnerReference{ownerRef1},
			desiredData:       map[string]string{"key1": "new-value"},
			desiredLabels:     map[string]string{"app": "nginx-gateway"},
			desiredAnns:       map[string]string{"annotation": "value"},
			desiredOwnerRefs:  []metav1.OwnerReference{ownerRef1},
			shouldUpdate:      true,
		},
		{
			name:              "updates when labels differ",
			existingData:      map[string]string{"key1": "value"},
			existingLabels:    map[string]string{"app": "old-app"},
			existingAnns:      map[string]string{"annotation": "value"},
			existingOwnerRefs: []metav1.OwnerReference{ownerRef1},
			desiredData:       map[string]string{"key1": "value"},
			desiredLabels:     map[string]string{"app": "nginx-gateway"},
			desiredAnns:       map[string]string{"annotation": "value"},
			desiredOwnerRefs:  []metav1.OwnerReference{ownerRef1},
			shouldUpdate:      true,
		},
		{
			name:              "updates when annotations differ",
			existingData:      map[string]string{"key1": "value"},
			existingLabels:    map[string]string{"app": "nginx-gateway"},
			existingAnns:      map[string]string{"annotation": "old-value"},
			existingOwnerRefs: []metav1.OwnerReference{ownerRef1},
			desiredData:       map[string]string{"key1": "value"},
			desiredLabels:     map[string]string{"app": "nginx-gateway"},
			desiredAnns:       map[string]string{"annotation": "new-value"},
			desiredOwnerRefs:  []metav1.OwnerReference{ownerRef1},
			shouldUpdate:      true,
		},
		{
			name:              "updates when owner references differ",
			existingData:      map[string]string{"key1": "value"},
			existingLabels:    map[string]string{"app": "nginx-gateway"},
			existingAnns:      map[string]string{"annotation": "value"},
			existingOwnerRefs: []metav1.OwnerReference{ownerRef1},
			desiredData:       map[string]string{"key1": "value"},
			desiredLabels:     map[string]string{"app": "nginx-gateway"},
			desiredAnns:       map[string]string{"annotation": "value"},
			desiredOwnerRefs:  []metav1.OwnerReference{ownerRef2},
			shouldUpdate:      true,
		},
		{
			name:              "no update when everything matches",
			existingData:      map[string]string{"key1": "value"},
			existingLabels:    map[string]string{"app": "nginx-gateway"},
			existingAnns:      map[string]string{"annotation": "value"},
			existingOwnerRefs: []metav1.OwnerReference{ownerRef1},
			desiredData:       map[string]string{"key1": "value"},
			desiredLabels:     map[string]string{"app": "nginx-gateway"},
			desiredAnns:       map[string]string{"annotation": "value"},
			desiredOwnerRefs:  []metav1.OwnerReference{ownerRef1Copy},
			shouldUpdate:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-configmap",
					Namespace:       "default",
					Labels:          tt.existingLabels,
					Annotations:     tt.existingAnns,
					OwnerReferences: tt.existingOwnerRefs,
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
			originalOwnerRefs := make([]metav1.OwnerReference, len(existing.OwnerReferences))
			copy(originalOwnerRefs, existing.OwnerReferences)

			desiredMeta := metav1.ObjectMeta{
				Labels:          tt.desiredLabels,
				Annotations:     tt.desiredAnns,
				OwnerReferences: tt.desiredOwnerRefs,
			}

			err := configMapSpecSetter(existing, tt.desiredData, desiredMeta)()
			g.Expect(err).ToNot(HaveOccurred())

			// Object meta fields, ensure name and namespace didn't change
			g.Expect(existing.Name).To(Equal("test-configmap"))
			g.Expect(existing.Namespace).To(Equal("default"))

			if tt.shouldUpdate {
				g.Expect(existing.Annotations).To(Equal(tt.desiredAnns))
				g.Expect(existing.Labels).To(Equal(tt.desiredLabels))
				g.Expect(existing.OwnerReferences).To(Equal(tt.desiredOwnerRefs))

				g.Expect(existing.Data).To(Equal(tt.desiredData))
			} else {
				g.Expect(existing.Data).To(Equal(originalData))

				g.Expect(existing.Labels).To(Equal(originalLabels))
				g.Expect(existing.Annotations).To(Equal(originalAnns))
				g.Expect(existing.OwnerReferences).To(Equal(originalOwnerRefs))
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

	secretType := corev1.SecretTypeOpaque

	err := secretSpecSetter(existing, data, secretType, desiredMeta)()
	g.Expect(err).ToNot(HaveOccurred())

	// Object meta fields, ensure name and namespace didn't change
	g.Expect(existing.Name).To(Equal("test-secret"))
	g.Expect(existing.Namespace).To(Equal("default"))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Labels).To(Equal(labels))

	g.Expect(existing.Data).To(Equal(data))
	g.Expect(existing.Type).To(Equal(secretType))
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

	err := roleSpecSetter(existing, rules, desiredMeta)()
	g.Expect(err).ToNot(HaveOccurred())

	// Object meta fields, ensure name and namespace didn't change
	g.Expect(existing.Name).To(Equal("test-role"))
	g.Expect(existing.Namespace).To(Equal("default"))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Labels).To(Equal(labels))

	g.Expect(existing.Rules).To(Equal(rules))
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

	err := roleBindingSpecSetter(existing, roleRef, subjects, desiredMeta)()
	g.Expect(err).ToNot(HaveOccurred())

	// Object meta fields, ensure name and namespace didn't change
	g.Expect(existing.Name).To(Equal("test-rolebinding"))
	g.Expect(existing.Namespace).To(Equal("default"))
	g.Expect(existing.Annotations).To(Equal(annotations))
	g.Expect(existing.Labels).To(Equal(labels))

	g.Expect(existing.RoleRef).To(Equal(roleRef))
	g.Expect(existing.Subjects).To(Equal(subjects))
}
