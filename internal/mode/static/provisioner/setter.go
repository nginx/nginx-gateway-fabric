package provisioner

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// objectSpecSetter sets the spec of the provided object. This is used when creating or updating the object.
func objectSpecSetter(object client.Object) controllerutil.MutateFn {
	switch obj := object.(type) {
	case *appsv1.Deployment:
		return deploymentSpecSetter(obj, obj.Spec)
	case *corev1.Service:
		return serviceSpecSetter(obj, obj.Spec)
	case *corev1.ServiceAccount:
		return func() error { return nil }
	case *corev1.ConfigMap:
		return configMapSpecSetter(obj, obj.Data)
	case *corev1.Secret:
		return secretSpecSetter(obj, obj.Data)
	}

	return nil
}

func deploymentSpecSetter(deployment *appsv1.Deployment, spec appsv1.DeploymentSpec) controllerutil.MutateFn {
	return func() error {
		deployment.Spec = spec
		return nil
	}
}

func serviceSpecSetter(service *corev1.Service, spec corev1.ServiceSpec) controllerutil.MutateFn {
	return func() error {
		service.Spec = spec
		return nil
	}
}

func configMapSpecSetter(configMap *corev1.ConfigMap, data map[string]string) controllerutil.MutateFn {
	return func() error {
		configMap.Data = data
		return nil
	}
}

func secretSpecSetter(secret *corev1.Secret, data map[string][]byte) controllerutil.MutateFn {
	return func() error {
		secret.Data = data
		return nil
	}
}
