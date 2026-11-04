package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/big"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTLSCertSerialNumber performs a TLS handshake against the given host:port and returns
// the serial number of the server certificate. The serverName is used as the SNI value.
func GetTLSCertSerialNumber(address string, port int, serverName string) (*big.Int, error) {
	target := fmt.Sprintf("%s:%d", address, port)
	dialer := &tls.Dialer{
		Config: &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: true, //nolint:gosec // test traffic with self-signed certs
		},
	}
	netConn, err := dialer.DialContext(context.Background(), "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("TLS dial to %s (SNI %s) failed: %w", target, serverName, err)
	}
	defer netConn.Close()

	conn, ok := netConn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("unexpected connection type from TLS dialer")
	}
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no peer certificates returned by %s (SNI %s)", target, serverName)
	}

	return certs[0].SerialNumber, nil
}

// CreateTLSSecret returns a kubernetes.io/tls Secret object.
func CreateTLSSecret(name, namespace string, certPEM, keyPEM []byte) *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: core.SecretTypeTLS,
		Data: map[string][]byte{
			core.TLSCertKey:       certPEM,
			core.TLSPrivateKeyKey: keyPEM,
		},
	}
}
