package cache

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/configmaps"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

var (
	secretKeys = []string{
		secrets.AuthKey,
		secrets.LicenseJWTKey,
		secrets.CAKey,
		secrets.TLSCertKey,
		secrets.TLSKeyKey,
		corev1.DockerConfigJsonKey,
		corev1.DockerConfigKey,
	}

	configMapKeys = []string{
		secrets.CAKey,
		configmaps.AgentConfKey,
		configmaps.MainConfKey,
		configmaps.EventsConfKey,
		configmaps.MgmtConfKey,
	}
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

		newData := make(map[string][]byte)
		found := false
		for _, k := range secretKeys {
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

		newData := make(map[string]string)
		newBinaryData := make(map[string][]byte)
		var dataFound, binaryDataFound bool

		for _, k := range configMapKeys {
			if v, ok := cm.Data[k]; ok {
				newData[k] = v
				dataFound = true
			}
			if v, ok := cm.BinaryData[k]; ok {
				newBinaryData[k] = v
				binaryDataFound = true
			}
		}

		if !dataFound && !binaryDataFound {
			return nil, nil
		}

		if dataFound {
			cm.Data = newData
		} else {
			cm.Data = nil
		}

		if binaryDataFound {
			cm.BinaryData = newBinaryData
		} else {
			cm.BinaryData = nil
		}

		cm.SetManagedFields(nil)
		return cm, nil
	}
}
