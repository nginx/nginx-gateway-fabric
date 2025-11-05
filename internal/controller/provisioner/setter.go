package provisioner

import (
	"maps"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// objectSpecSetter sets the spec of the provided object. This is used when creating or updating the object.
func objectSpecSetter(object client.Object) controllerutil.MutateFn {
	switch obj := object.(type) {
	case *appsv1.Deployment:
		return deploymentSpecSetter(obj, obj.Spec, obj.ObjectMeta)
	case *autoscalingv2.HorizontalPodAutoscaler:
		return hpaSpecSetter(obj, obj.Spec, obj.ObjectMeta)
	case *appsv1.DaemonSet:
		return daemonSetSpecSetter(obj, obj.Spec, obj.ObjectMeta)
	case *corev1.Service:
		return serviceSpecSetter(obj, obj.Spec, obj.ObjectMeta)
	case *corev1.ServiceAccount:
		return serviceAccountSpecSetter(obj, obj.ObjectMeta)
	case *corev1.ConfigMap:
		return configMapSpecSetter(obj, obj.Data, obj.ObjectMeta)
	case *corev1.Secret:
		return secretSpecSetter(obj, obj.Data, obj.ObjectMeta)
	case *rbacv1.Role:
		return roleSpecSetter(obj, obj.Rules, obj.ObjectMeta)
	case *rbacv1.RoleBinding:
		return roleBindingSpecSetter(obj, obj.RoleRef, obj.Subjects, obj.ObjectMeta)
	}

	return nil
}

func deploymentSpecSetter(
	deployment *appsv1.Deployment,
	spec appsv1.DeploymentSpec,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		deployment.Labels = objectMeta.Labels
		deployment.Annotations = objectMeta.Annotations
		deployment.Spec = spec
		return nil
	}
}

func hpaSpecSetter(
	hpa *autoscalingv2.HorizontalPodAutoscaler,
	spec autoscalingv2.HorizontalPodAutoscalerSpec,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		hpa.Labels = objectMeta.Labels
		hpa.Annotations = objectMeta.Annotations
		hpa.Spec = spec
		return nil
	}
}

func daemonSetSpecSetter(
	daemonSet *appsv1.DaemonSet,
	spec appsv1.DaemonSetSpec,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		daemonSet.Labels = objectMeta.Labels
		daemonSet.Annotations = objectMeta.Annotations
		daemonSet.Spec = spec
		return nil
	}
}

func serviceSpecSetter(
	service *corev1.Service,
	spec corev1.ServiceSpec,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		const managedKeysAnnotation = "gateway.nginx.org/internal-managed-annotation-keys"

		// Track which annotation keys NGF currently manages
		currentManagedKeys := make(map[string]bool)
		for k := range objectMeta.Annotations {
			currentManagedKeys[k] = true
		}

		// Get previously managed keys from existing service
		var previousManagedKeys map[string]bool
		if prevKeysStr, ok := service.Annotations[managedKeysAnnotation]; ok {
			previousManagedKeys = make(map[string]bool)
			for _, k := range strings.Split(prevKeysStr, ",") {
				if k != "" {
					previousManagedKeys[k] = true
				}
			}
		}

		// Start with existing annotations (preserves external controller annotations)
		mergedAnnotations := make(map[string]string)
		for k, v := range service.Annotations {
			// Skip the internal tracking annotation
			if k == managedKeysAnnotation {
				continue
			}
			// Remove annotations that NGF previously managed but no longer wants
			if previousManagedKeys != nil && previousManagedKeys[k] && !currentManagedKeys[k] {
				continue // Remove this annotation
			}
			mergedAnnotations[k] = v
		}

		// Apply NGF-managed annotations (take precedence)
		for k, v := range objectMeta.Annotations {
			mergedAnnotations[k] = v
		}

		// Store current managed keys for next reconciliation
		if len(currentManagedKeys) > 0 {
			var managedKeysList []string
			for k := range currentManagedKeys {
				managedKeysList = append(managedKeysList, k)
			}
			slices.Sort(managedKeysList) // Sort for deterministic output
			mergedAnnotations[managedKeysAnnotation] = strings.Join(managedKeysList, ",")
		}

		service.Labels = objectMeta.Labels
		service.Annotations = mergedAnnotations
		service.Spec = spec
		return nil
	}
}

func serviceAccountSpecSetter(
	serviceAccount *corev1.ServiceAccount,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		serviceAccount.Labels = objectMeta.Labels
		serviceAccount.Annotations = objectMeta.Annotations
		return nil
	}
}

func configMapSpecSetter(
	configMap *corev1.ConfigMap,
	data map[string]string,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		// this check ensures we don't trigger an unnecessary update to the agent ConfigMap
		// and trigger a Deployment restart
		if maps.Equal(configMap.Labels, objectMeta.Labels) &&
			maps.Equal(configMap.Annotations, objectMeta.Annotations) &&
			maps.Equal(configMap.Data, data) {
			return nil
		}

		configMap.Labels = objectMeta.Labels
		configMap.Annotations = objectMeta.Annotations
		configMap.Data = data
		return nil
	}
}

func secretSpecSetter(
	secret *corev1.Secret,
	data map[string][]byte,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		secret.Labels = objectMeta.Labels
		secret.Annotations = objectMeta.Annotations
		secret.Data = data
		return nil
	}
}

func roleSpecSetter(
	role *rbacv1.Role,
	rules []rbacv1.PolicyRule,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		role.Labels = objectMeta.Labels
		role.Annotations = objectMeta.Annotations
		role.Rules = rules
		return nil
	}
}

func roleBindingSpecSetter(
	roleBinding *rbacv1.RoleBinding,
	roleRef rbacv1.RoleRef,
	subjects []rbacv1.Subject,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		roleBinding.Labels = objectMeta.Labels
		roleBinding.Annotations = objectMeta.Annotations
		roleBinding.RoleRef = roleRef
		roleBinding.Subjects = subjects
		return nil
	}
}
