package resolver

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/configmaps"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type configMapEntry struct {
	// err holds the corresponding error if the ConfigMap is invalid or does not exist.
	err             error
	caCertConfigMap configmaps.CaCertConfigMap
}

func (c *configMapEntry) setError(err error) {
	c.err = err
}

func (c *configMapEntry) error() error {
	return c.err
}

func (c *configMapEntry) validate(obj client.Object) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		panic(fmt.Sprintf("expected ConfigMap object, got %T", obj))
	}

	var validationErr error
	cert := &secrets.Certificate{}

	// Any future ConfigMap keys that are needed MUST be added to cache/transform.go
	// in order to track them.
	if cm.Data != nil {
		if _, exists := cm.Data[secrets.CAKey]; exists {
			validationErr = secrets.ValidateCA([]byte(cm.Data[secrets.CAKey]))
			cert.CACert = []byte(cm.Data[secrets.CAKey])
		}
	}
	if cm.BinaryData != nil {
		if _, exists := cm.BinaryData[secrets.CAKey]; exists {
			validationErr = secrets.ValidateCA(cm.BinaryData[secrets.CAKey])
			cert.CACert = cm.BinaryData[secrets.CAKey]
		}
	}
	if len(cert.CACert) == 0 {
		validationErr = fmt.Errorf("ConfigMap does not have the data or binaryData field %v", secrets.CAKey)
	}

	c.caCertConfigMap = configmaps.CaCertConfigMap{
		Source:     cm,
		CertBundle: secrets.NewCertificateBundle(client.ObjectKeyFromObject(cm), kinds.ConfigMap, cert),
	}
	c.setError(validationErr)
}
