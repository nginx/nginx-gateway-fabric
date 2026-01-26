package cache

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

// TransformGatewayClass filters GatewayClass objects to only include those
// that match the specified controller name. It also removes managed fields
// to reduce memory usage.
func TransformGatewayClass(controllerName string) cache.TransformFunc {
	return func(obj any) (any, error) {
		gc, ok := obj.(*gatewayv1.GatewayClass)
		if !ok {
			return nil, nil
		}

		if gc.Spec.ControllerName != gatewayv1.GatewayController(controllerName) {
			return nil, nil
		}

		gc.SetManagedFields(nil)
		return gc, nil
	}
}

// TransformSecret filters Secret objects to only include specific keys
// and removes managed fields to reduce memory usage.
func TransformSecret() cache.TransformFunc {
	return func(obj any) (any, error) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil, nil
		}

		keys := []string{
			secrets.LicenseJWTKey,
			secrets.CAKey,
			secrets.TLSCertKey,
			secrets.TLSKeyKey,
			corev1.DockerConfigJsonKey,
			corev1.DockerConfigKey,
		}
		newData := make(map[string][]byte)
		found := false
		for _, k := range keys {
			if v, ok := secret.Data[k]; ok {
				newData[k] = v
				found = true
			}
		}

		if !found {
			return nil, nil
		}

		secret.Data = newData
		secret.SetManagedFields(nil)
		return secret, nil
	}
}

// TransformConfigMap filters ConfigMap objects to only include the CAKey.
// If the CAKey is not present, the ConfigMap is ignored.
// All other keys are dropped, and managed fields are removed to reduce memory usage.
func TransformConfigMap() cache.TransformFunc {
	return func(obj any) (any, error) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, nil
		}

		vStr, foundStr := cm.Data[secrets.CAKey]
		vBin, foundBin := cm.BinaryData[secrets.CAKey]

		switch {
		case foundStr:
			cm.Data = map[string]string{secrets.CAKey: vStr}
			cm.BinaryData = nil
		case foundBin:
			cm.Data = nil
			cm.BinaryData = map[string][]byte{secrets.CAKey: vBin}
		default:
			return nil, nil
		}

		cm.SetManagedFields(nil)
		return cm, nil
	}
}
