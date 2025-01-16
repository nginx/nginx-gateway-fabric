package config_test

import (
	"sort"
	"testing"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctlrZap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/file"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	ngfConfig "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	bg := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{UpstreamName: "test", Valid: true, Weight: 1},
			{UpstreamName: "test2", Valid: true, Weight: 1},
		},
	}

	conf := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{
			{
				IsDefault: true,
				Port:      80,
			},
			{
				Hostname: "example.com",
				Port:     80,
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				IsDefault: true,
				Port:      443,
			},
			{
				Hostname: "example.com",
				SSL: &dataplane.SSL{
					KeyPairID: "test-keypair",
				},
				Port: 443,
			},
		},
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname:     "app.example.com",
				Port:         443,
				UpstreamName: "stream_up",
			},
		},
		Upstreams: []dataplane.Upstream{
			{
				Name:      "up",
				Endpoints: nil,
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name: "stream_up",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
		},
		BackendGroups: []dataplane.BackendGroup{bg},
		SSLKeyPairs: map[dataplane.SSLKeyPairID]dataplane.SSLKeyPair{
			"test-keypair": {
				Cert: []byte("test-cert"),
				Key:  []byte("test-key"),
			},
		},
		CertBundles: map[dataplane.CertBundleID]dataplane.CertBundle{
			"test-certbundle": []byte("test-cert"),
		},
		Telemetry: dataplane.Telemetry{
			Endpoint:    "1.2.3.4:123",
			ServiceName: "ngf:gw-ns:gw-name:my-name",
			Interval:    "5s",
			BatchSize:   512,
			BatchCount:  4,
		},
		Logging: dataplane.Logging{
			ErrorLevel: "debug",
		},
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			HTTP2: true,
			Snippets: []dataplane.Snippet{
				{
					Name:     "http_snippet1",
					Contents: "http snippet 1 contents",
				},
				{
					Name:     "http_snippet2",
					Contents: "http 2 contents",
				},
			},
		},
		MainSnippets: []dataplane.Snippet{
			{
				Name:     "main_snippet1",
				Contents: "main snippet 1 contents",
			},
			{
				Name:     "main_snippet2",
				Contents: "main 2 contents",
			},
		},
		DeploymentContext: dataplane.DeploymentContext{
			Integration:      "ngf",
			ClusterID:        helpers.GetPointer("test-uid"),
			InstallationID:   helpers.GetPointer("test-uid-replicaSet"),
			ClusterNodeCount: helpers.GetPointer(1),
		},
		AuxiliarySecrets: map[graph.SecretFileType][]byte{
			graph.PlusReportJWTToken:             []byte("license"),
			graph.PlusReportCACertificate:        []byte("ca"),
			graph.PlusReportClientSSLCertificate: []byte("cert"),
			graph.PlusReportClientSSLKey:         []byte("key"),
		},
	}
	g := NewWithT(t)

	plus := true
	generator := config.NewGeneratorImpl(
		plus,
		&ngfConfig.UsageReportConfig{Endpoint: "test-endpoint"},
		ctlrZap.New(),
	)

	files := generator.Generate(conf)

	g.Expect(files).To(HaveLen(16))
	arrange := func(i, j int) bool {
		return files[i].Meta.Name < files[j].Meta.Name
	}
	sort.Slice(files, arrange)

	/*
		Order of files:
		/etc/nginx/conf.d/http.conf
		/etc/nginx/conf.d/matches.json
		/etc/nginx/includes/http_snippet1.conf
		/etc/nginx/includes/http_snippet2.conf
		/etc/nginx/includes/main_snippet1.conf
		/etc/nginx/includes/main_snippet2.conf
		/etc/nginx/main-includes/deployment_ctx.json
		/etc/nginx/main-includes/main.conf
		/etc/nginx/main-includes/mgmt.conf
		/etc/nginx/secrets/license.jwt
		/etc/nginx/secrets/mgmt-ca.crt
		/etc/nginx/secrets/mgmt-tls.crt
		/etc/nginx/secrets/mgmt-tls.key
		/etc/nginx/secrets/test-certbundle.crt
		/etc/nginx/secrets/test-keypair.pem
		/etc/nginx/stream-conf.d/stream.conf
	*/

	g.Expect(files[0].Meta.Permissions).To(Equal(file.RegularFileMode))
	g.Expect(files[0].Meta.Name).To(Equal("/etc/nginx/conf.d/http.conf"))
	httpCfg := string(files[0].Contents) // converting to string so that on failure gomega prints strings not byte arrays
	// Note: this only verifies that Generate() returns a byte array with upstream, server, and split_client blocks.
	// It does not test the correctness of those blocks. That functionality is covered by other tests in this package.
	g.Expect(httpCfg).To(ContainSubstring("listen 80"))
	g.Expect(httpCfg).To(ContainSubstring("listen unix:/var/run/nginx/https443.sock"))
	g.Expect(httpCfg).To(ContainSubstring("upstream"))
	g.Expect(httpCfg).To(ContainSubstring("split_clients"))

	g.Expect(httpCfg).To(ContainSubstring("endpoint 1.2.3.4:123;"))
	g.Expect(httpCfg).To(ContainSubstring("interval 5s;"))
	g.Expect(httpCfg).To(ContainSubstring("batch_size 512;"))
	g.Expect(httpCfg).To(ContainSubstring("batch_count 4;"))
	g.Expect(httpCfg).To(ContainSubstring("otel_service_name ngf:gw-ns:gw-name:my-name;"))
	g.Expect(httpCfg).To(ContainSubstring("http2 on;"))
	g.Expect(httpCfg).To(ContainSubstring("include /etc/nginx/includes/http_snippet1.conf;"))
	g.Expect(httpCfg).To(ContainSubstring("include /etc/nginx/includes/http_snippet2.conf;"))

	g.Expect(files[1].Meta.Name).To(Equal("/etc/nginx/conf.d/matches.json"))

	g.Expect(files[1].Meta.Permissions).To(Equal(file.RegularFileMode))
	expString := "{}"
	g.Expect(string(files[1].Contents)).To(Equal(expString))

	// snippet include files
	// content is not checked in this test.
	g.Expect(files[2].Meta.Name).To(Equal("/etc/nginx/includes/http_snippet1.conf"))
	g.Expect(files[3].Meta.Name).To(Equal("/etc/nginx/includes/http_snippet2.conf"))
	g.Expect(files[4].Meta.Name).To(Equal("/etc/nginx/includes/main_snippet1.conf"))
	g.Expect(files[5].Meta.Name).To(Equal("/etc/nginx/includes/main_snippet2.conf"))

	g.Expect(files[6].Meta.Name).To(Equal("/etc/nginx/main-includes/deployment_ctx.json"))
	deploymentCtx := string(files[6].Contents)
	g.Expect(deploymentCtx).To(ContainSubstring("\"integration\":\"ngf\""))
	g.Expect(deploymentCtx).To(ContainSubstring("\"cluster_id\":\"test-uid\""))
	g.Expect(deploymentCtx).To(ContainSubstring("\"installation_id\":\"test-uid-replicaSet\""))
	g.Expect(deploymentCtx).To(ContainSubstring("\"cluster_node_count\":1"))

	g.Expect(files[7].Meta.Name).To(Equal("/etc/nginx/main-includes/main.conf"))
	mainConfStr := string(files[7].Contents)
	g.Expect(mainConfStr).To(ContainSubstring("load_module modules/ngx_otel_module.so;"))
	g.Expect(mainConfStr).To(ContainSubstring("include /etc/nginx/includes/main_snippet1.conf;"))
	g.Expect(mainConfStr).To(ContainSubstring("include /etc/nginx/includes/main_snippet2.conf;"))

	g.Expect(files[8].Meta.Name).To(Equal("/etc/nginx/main-includes/mgmt.conf"))
	mgmtConf := string(files[8].Contents)
	g.Expect(mgmtConf).To(ContainSubstring("usage_report endpoint=test-endpoint"))
	g.Expect(mgmtConf).To(ContainSubstring("license_token /etc/nginx/secrets/license.jwt"))
	g.Expect(mgmtConf).To(ContainSubstring("deployment_context /etc/nginx/main-includes/deployment_ctx.json"))
	g.Expect(mgmtConf).To(ContainSubstring("ssl_trusted_certificate /etc/nginx/secrets/mgmt-ca.crt"))
	g.Expect(mgmtConf).To(ContainSubstring("ssl_certificate /etc/nginx/secrets/mgmt-tls.crt"))
	g.Expect(mgmtConf).To(ContainSubstring("ssl_certificate_key /etc/nginx/secrets/mgmt-tls.key"))

	g.Expect(files[9].Meta.Name).To(Equal("/etc/nginx/secrets/license.jwt"))
	g.Expect(string(files[9].Contents)).To(Equal("license"))

	g.Expect(files[10].Meta.Name).To(Equal("/etc/nginx/secrets/mgmt-ca.crt"))
	g.Expect(string(files[10].Contents)).To(Equal("ca"))

	g.Expect(files[11].Meta.Name).To(Equal("/etc/nginx/secrets/mgmt-tls.crt"))
	g.Expect(string(files[11].Contents)).To(Equal("cert"))

	g.Expect(files[12].Meta.Name).To(Equal("/etc/nginx/secrets/mgmt-tls.key"))
	g.Expect(string(files[12].Contents)).To(Equal("key"))

	g.Expect(files[13].Meta.Name).To(Equal("/etc/nginx/secrets/test-certbundle.crt"))
	certBundle := string(files[13].Contents)
	g.Expect(certBundle).To(Equal("test-cert"))

	g.Expect(files[14]).To(Equal(agent.File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test-keypair.pem",
			Hash:        filesHelper.GenerateHash([]byte("test-cert\ntest-key")),
			Permissions: file.SecretFileMode,
		},
		Contents: []byte("test-cert\ntest-key"),
	}))

	g.Expect(files[15].Meta.Name).To(Equal("/etc/nginx/stream-conf.d/stream.conf"))
	g.Expect(files[15].Meta.Permissions).To(Equal(file.RegularFileMode))
	streamCfg := string(files[15].Contents)
	g.Expect(streamCfg).To(ContainSubstring("listen unix:/var/run/nginx/app.example.com-443.sock"))
	g.Expect(streamCfg).To(ContainSubstring("listen 443"))
	g.Expect(streamCfg).To(ContainSubstring("app.example.com unix:/var/run/nginx/app.example.com-443.sock"))
	g.Expect(streamCfg).To(ContainSubstring("example.com unix:/var/run/nginx/https443.sock"))
}
