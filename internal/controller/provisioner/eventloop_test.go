package provisioner

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
)

func TestAddSecretsToWatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		usageConfig    *config.UsageReportConfig
		name           string
		dockerSecrets  []string
		agentTLSSecret string
		dataplaneKey   string
		expSecrets     []string
	}{
		{
			name:           "no usage config",
			agentTLSSecret: "agent-tls",
			dockerSecrets:  []string{"docker1"},
			expSecrets:     []string{"agent-tls", "docker1"},
		},
		{
			name:           "usage config with jwt secret only",
			agentTLSSecret: "agent-tls",
			usageConfig: &config.UsageReportConfig{
				SecretName: "nplus-license",
			},
			expSecrets: []string{"agent-tls", "nplus-license"},
		},
		{
			name:           "usage config with all secrets",
			agentTLSSecret: "agent-tls",
			usageConfig: &config.UsageReportConfig{
				SecretName:          "nplus-license",
				CASecretName:        "ca",
				ClientSSLSecretName: "client-ssl",
			},
			expSecrets: []string{"agent-tls", "nplus-license", "ca", "client-ssl"},
		},
		{
			name:           "usage config with proxy credentials secret",
			agentTLSSecret: "agent-tls",
			usageConfig: &config.UsageReportConfig{
				SecretName:                 "nplus-license",
				ProxyCredentialsSecretName: "proxy-creds",
			},
			expSecrets: []string{"agent-tls", "nplus-license", "proxy-creds"},
		},
		{
			name:           "usage config with all secrets including proxy credentials",
			agentTLSSecret: "agent-tls",
			usageConfig: &config.UsageReportConfig{
				SecretName:                 "nplus-license",
				CASecretName:               "ca",
				ClientSSLSecretName:        "client-ssl",
				ProxyCredentialsSecretName: "proxy-creds",
			},
			expSecrets: []string{"agent-tls", "nplus-license", "ca", "client-ssl", "proxy-creds"},
		},
		{
			name:           "dataplane key included",
			agentTLSSecret: "agent-tls",
			dataplaneKey:   "dp-key",
			expSecrets:     []string{"agent-tls", "dp-key"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := addSecretsToWatch(test.dockerSecrets, test.agentTLSSecret, test.dataplaneKey, test.usageConfig)
			g.Expect(result).To(ConsistOf(test.expSecrets))
		})
	}
}
