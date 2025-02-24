package graph

import (
	"errors"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CaCertConfigMap represents a ConfigMap resource that holds CA Cert data.
type CaCertConfigMap struct {
	// Source holds the actual ConfigMap resource. Can be nil if the ConfigMap does not exist.
	Source     *apiv1.ConfigMap
	CertBundle *CertificateBundle
}

type caCertConfigMapEntry struct {
	// err holds the corresponding error if the ConfigMap is invalid or does not exist.
	err             error
	caCertConfigMap CaCertConfigMap
}

// configMapResolver wraps the cluster ConfigMaps so that they can be resolved (includes validation). All resolved
// ConfigMaps are saved to be used later.
type configMapResolver struct {
	clusterConfigMaps        map[types.NamespacedName]*apiv1.ConfigMap
	resolvedCaCertConfigMaps map[types.NamespacedName]*caCertConfigMapEntry
}

func newConfigMapResolver(configMaps map[types.NamespacedName]*apiv1.ConfigMap) *configMapResolver {
	return &configMapResolver{
		clusterConfigMaps:        configMaps,
		resolvedCaCertConfigMaps: make(map[types.NamespacedName]*caCertConfigMapEntry),
	}
}

func (r *configMapResolver) resolve(nsname types.NamespacedName) error {
	if s, resolved := r.resolvedCaCertConfigMaps[nsname]; resolved {
		return s.err
	}

	cm, exist := r.clusterConfigMaps[nsname]

	var validationErr error
	cert := &Certificate{}

	if !exist {
		validationErr = errors.New("ConfigMap does not exist")
	} else {
		if cm.Data != nil {
			if _, exists := cm.Data[CAKey]; exists {
				validationErr = validateCA([]byte(cm.Data[CAKey]))
				cert.CACert = []byte(cm.Data[CAKey])
			}
		}
		if cm.BinaryData != nil {
			if _, exists := cm.BinaryData[CAKey]; exists {
				validationErr = validateCA(cm.BinaryData[CAKey])
				cert.CACert = cm.BinaryData[CAKey]
			}
		}
		if len(cert.CACert) == 0 {
			validationErr = fmt.Errorf("ConfigMap does not have the data or binaryData field %v", CAKey)
		}
	}

	r.resolvedCaCertConfigMaps[nsname] = &caCertConfigMapEntry{
		caCertConfigMap: CaCertConfigMap{
			Source:     cm,
			CertBundle: NewCertificateBundle(nsname, "ConfigMap", cert),
		},
		err: validationErr,
	}

	return validationErr
}

func (r *configMapResolver) getResolvedConfigMaps() map[types.NamespacedName]*CaCertConfigMap {
	if len(r.resolvedCaCertConfigMaps) == 0 {
		return nil
	}

	resolved := make(map[types.NamespacedName]*CaCertConfigMap)

	for nsname, entry := range r.resolvedCaCertConfigMaps {
		// create iteration variable inside the loop to fix implicit memory aliasing
		caCertConfigMap := entry.caCertConfigMap
		resolved[nsname] = &caCertConfigMap
	}

	return resolved
}
