package graph

// func Test_Gateways(t *testing.T) {
// 	const gcName = "my-gateway-class"
// 	createListener := func(
// 		name string,
// 		hostname string,
// 		port int,
// 		protocol v1.ProtocolType,
// 		tls *v1.GatewayTLSConfig,
// 	) v1.Listener {
// 		return v1.Listener{
// 			Name:     v1.SectionName(name),
// 			Hostname: (*v1.Hostname)(helpers.GetPointer(hostname)),
// 			Port:     v1.PortNumber(port), //nolint:gosec // port number will not overflow int32
// 			Protocol: protocol,
// 			TLS:      tls,
// 		}
// 	}

// 	createHTTPListener := func(name, hostname string, port int) v1.Listener {
// 		return createListener(name, hostname, port, v1.HTTPProtocolType, nil)
// 	}
// 	createTCPListener := func(name, hostname string, port int) v1.Listener {
// 		return createListener(name, hostname, port, v1.TCPProtocolType, nil)
// 	}
// 	createTLSListener := func(name, hostname string, port int) v1.Listener {
// 		return createListener(
// 			name,
// 			hostname,
// 			port,
// 			v1.TLSProtocolType,
// 			&v1.GatewayTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
// 		)
// 	}

// 	createHTTPSListener := func(name, hostname string, port int, tls *v1.GatewayTLSConfig) v1.Listener {
// 		return createListener(name, hostname, port, v1.HTTPSProtocolType, tls)
// 	}

// 	var lastCreatedGateway *v1.Gateway
// 	type gatewayCfg struct {
// 		name      string
// 		ref       *v1.LocalParametersReference
// 		listeners []v1.Listener
// 		addresses []v1.GatewayAddress
// 	}
// 	createGateway := func(cfg gatewayCfg) map[types.NamespacedName]*v1.Gateway {
// 		gatewayMap := make(map[types.NamespacedName]*v1.Gateway)
// 		lastCreatedGateway = &v1.Gateway{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      cfg.name,
// 				Namespace: "test",
// 			},
// 			Spec: v1.GatewaySpec{
// 				GatewayClassName: gcName,
// 				Listeners:        cfg.listeners,
// 				Addresses:        cfg.addresses,
// 			},
// 		}

// 		if cfg.ref != nil {
// 			lastCreatedGateway.Spec.Infrastructure = &v1.GatewayInfrastructure{
// 				ParametersRef: cfg.ref,
// 			}
// 		}

// 		gatewayMap[types.NamespacedName{
// 			Namespace: lastCreatedGateway.Namespace,
// 			Name:      lastCreatedGateway.Name,
// 		}] = lastCreatedGateway
// 		return gatewayMap
// 	}

// 	getLastCreatedGateway := func() *v1.Gateway {
// 		return lastCreatedGateway
// 	}

// 	validGwNp := &ngfAPIv1alpha2.NginxProxy{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Namespace: "test",
// 			Name:      "valid-gw-np",
// 		},
// 		Spec: ngfAPIv1alpha2.NginxProxySpec{
// 			Logging: &ngfAPIv1alpha2.NginxLogging{ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelError)},
// 		},
// 	}

// 	validGC := &GatewayClass{
// 		Valid: true,
// 	}

// 	gatewayTLSConfigDiffNs := &v1.GatewayTLSConfig{
// 		Mode: helpers.GetPointer(v1.TLSModeTerminate),
// 		CertificateRefs: []v1.SecretObjectReference{
// 			{
// 				Kind:      helpers.GetPointer[v1.Kind]("Secret"),
// 				Name:      v1.ObjectName(secretDiffNamespace.Name),
// 				Namespace: (*v1.Namespace)(&secretDiffNamespace.Namespace),
// 			},
// 		},
// 	}

// 	secretDiffNamespace := &apiv1.Secret{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Namespace: "diff-ns",
// 			Name:      "secret",
// 		},
// 		Data: map[string][]byte{
// 			apiv1.TLSCertKey:       cert,
// 			apiv1.TLSPrivateKeyKey: key,
// 		},
// 		Type: apiv1.SecretTypeTLS,
// 	}
// }
