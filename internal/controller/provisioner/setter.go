package provisioner

import (
	"fmt"
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
//
//nolint:gocyclo // This is the best we can do
func objectSpecSetter(minimalObject, object client.Object) controllerutil.MutateFn {
	switch obj := object.(type) {
	case *appsv1.Deployment:
		if minObj, ok := minimalObject.(*appsv1.Deployment); ok {
			return deploymentSpecSetter(minObj, obj.Spec, obj.ObjectMeta)
		}
	case *autoscalingv2.HorizontalPodAutoscaler:
		if minObj, ok := minimalObject.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			return hpaSpecSetter(minObj, obj.Spec, obj.ObjectMeta)
		}
	case *appsv1.DaemonSet:
		if minObj, ok := minimalObject.(*appsv1.DaemonSet); ok {
			return daemonSetSpecSetter(minObj, obj.Spec, obj.ObjectMeta)
		}
	case *corev1.Service:
		if minObj, ok := minimalObject.(*corev1.Service); ok {
			return serviceSpecSetter(minObj, obj.Spec, obj.ObjectMeta)
		}
	case *corev1.ServiceAccount:
		if minObj, ok := minimalObject.(*corev1.ServiceAccount); ok {
			return serviceAccountSpecSetter(minObj, obj.ObjectMeta)
		}
	case *corev1.ConfigMap:
		if minObj, ok := minimalObject.(*corev1.ConfigMap); ok {
			return configMapSpecSetter(minObj, obj.Data, obj.ObjectMeta)
		}
	case *corev1.Secret:
		if minObj, ok := minimalObject.(*corev1.Secret); ok {
			return secretSpecSetter(minObj, obj.Data, obj.ObjectMeta)
		}
	case *rbacv1.Role:
		if minObj, ok := minimalObject.(*rbacv1.Role); ok {
			return roleSpecSetter(minObj, obj.Rules, obj.ObjectMeta)
		}
	case *rbacv1.RoleBinding:
		if minObj, ok := minimalObject.(*rbacv1.RoleBinding); ok {
			return roleBindingSpecSetter(minObj, obj.RoleRef, obj.Subjects, obj.ObjectMeta)
		}
	}

	panic(fmt.Sprintf("type mismatch: minimalObject type %T does not match object type %T", minimalObject, object))
}

func deploymentSpecSetter(
	deployment *appsv1.Deployment,
	spec appsv1.DeploymentSpec,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		deployment.Labels = objectMeta.Labels
		deployment.Annotations = mergeAnnotations(deployment.Annotations, objectMeta.Annotations)
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
		daemonSet.Annotations = mergeAnnotations(daemonSet.Annotations, objectMeta.Annotations)
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
		service.Labels = objectMeta.Labels
		service.Annotations = mergeAnnotations(service.Annotations, objectMeta.Annotations)
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

func mergeAnnotations(existing, desired map[string]string) map[string]string {
	const trackingKey = "gateway.nginx.org/internal-managed-annotation-keys"
	desiredKeys := make(map[string]struct{}, len(desired))
	for key := range desired {
		desiredKeys[key] = struct{}{}
	}

	previousKeys := make(map[string]struct{}, len(existing))
	if existing != nil {
		if prev, ok := existing[trackingKey]; ok {
			for splitKey := range strings.SplitSeq(prev, ",") {
				if splitKey != "" {
					previousKeys[splitKey] = struct{}{}
				}
			}
		}
	}

	annotations := make(map[string]string)

	// Start with existing annotations (preserves external controller annotations)
	for key, value := range existing {
		if key == trackingKey {
			continue
		}

		// if this key was previously managed and is no longer desired, drop it
		if _, wasManaged := previousKeys[key]; wasManaged {
			if _, stillDesired := desiredKeys[key]; !stillDesired {
				continue
			}
		}

		annotations[key] = value
	}

	// Apply desired annotations (NGF-managed wins)
	maps.Copy(annotations, desired)

	// Store current managed keys
	if len(desiredKeys) > 0 {
		keys := make([]string, 0, len(desiredKeys))
		for key := range desiredKeys {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		annotations[trackingKey] = strings.Join(keys, ",")
	}

	return annotations
}
