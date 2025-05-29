package provisioner

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
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

func serviceSpecSetter(
	service *corev1.Service,
	spec corev1.ServiceSpec,
	objectMeta metav1.ObjectMeta,
) controllerutil.MutateFn {
	return func() error {
		service.Labels = objectMeta.Labels
		service.Annotations = objectMeta.Annotations
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
