package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"

	ngfConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/accesspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/clientsettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/observability"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/proxysettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/ratelimit"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/snippetspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/waf"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
)

//go:generate go tool moq -skip-ensure -pkg configfakes -out configfakes/fake_generator.go . Generator

// Volumes here also need to be added to our crossplane ephemeral test container.
const (
	// configFolder is the folder where NGINX configuration files are stored.
	configFolder = "/etc/nginx"

	// httpFolder is the folder where NGINX HTTP configuration files are stored.
	httpFolder = configFolder + "/conf.d"

	// streamFolder is the folder where NGINX Stream configuration files are stored.
	streamFolder = configFolder + "/stream-conf.d"

	// mainIncludesFolder is the folder where NGINX main context configuration files are stored.
	// For example, these files include load_module directives and snippets that target the main context.
	mainIncludesFolder = configFolder + "/main-includes"

	// eventsIncludesFolder is the folder where NGINX events context configuration files are stored.
	eventsIncludesFolder = configFolder + "/events-includes"

	// secretsFolder is the folder where secrets (like TLS certs/keys) are stored.
	secretsFolder = configFolder + "/secrets"

	// includesFolder is the folder where are all include files are stored.
	includesFolder = configFolder + "/includes"

	// appProtectBundleFolder is the folder where the NGINX App Protect WAF bundles are stored.
	appProtectBundleFolder = "/etc/app_protect/bundles"

	// httpConfigFile is the path to the configuration file with HTTP configuration.
	httpConfigFile = httpFolder + "/http.conf"

	// streamConfigFile is the path to the configuration file with Stream configuration.
	streamConfigFile = streamFolder + "/stream.conf"

	// httpMatchVarsFile is the path to the http_match pairs configuration file.
	httpMatchVarsFile = httpFolder + "/matches.json"

	// mainIncludesConfigFile is the path to the file containing NGINX configuration in the main context.
	mainIncludesConfigFile = mainIncludesFolder + "/main.conf"

	// eventsIncludesConfigFile is the path to the file containing NGINX events configuration.
	eventsIncludesConfigFile = eventsIncludesFolder + "/events.conf"

	// mgmtIncludesFile is the path to the file containing the NGINX Plus mgmt config.
	mgmtIncludesFile = mainIncludesFolder + "/mgmt.conf"

	// MgmtCAFile is the CA cert path for the usage-reporting endpoint's TLS connection.
	MgmtCAFile = secretsFolder + "/mgmt-ca.crt"

	// MgmtClientSSLCertFile is the client cert path for mTLS with the usage-reporting endpoint.
	MgmtClientSSLCertFile = secretsFolder + "/mgmt-tls.crt"

	// MgmtClientSSLKeyFile is the client key path for mTLS with the usage-reporting endpoint.
	MgmtClientSSLKeyFile = secretsFolder + "/mgmt-tls.key"

	// nginxPlusConfigFile is the path to the file containing the NGINX Plus API config.
	nginxPlusConfigFile = httpFolder + "/plus-api.conf"
)

// GenerateResult holds the categories of generated files.
type GenerateResult struct {
	// ConfigApplyFiles are files whose changes require an NGINX reload.
	// These are sent via a ConfigApplyRequest to the agent.
	ConfigApplyFiles []agent.File
	// ContentUpdateFiles are files whose changes don't require a config apply.
	// These are sent via a ContentUpdateRequest to the agent.
	ContentUpdateFiles []agent.File
}

// Generator generates NGINX configuration files.
// This interface is used for testing purposes only.
type Generator interface {
	// Generate generates NGINX configuration files from internal representation.
	Generate(logger logr.Logger, configuration dataplane.Configuration) GenerateResult
	// GenerateDeploymentContext generates the deployment context used for N+ licensing.
	GenerateDeploymentContext(depCtx dataplane.DeploymentContext) (agent.File, error)
}

// GeneratorImpl is an implementation of Generator.
//
// It generates files to be written to the folders above, which must exist and available for writing.
//
// It also expects that the main NGINX configuration file nginx.conf is located in configFolder and nginx.conf
// includes (https://nginx.org/en/docs/ngx_core_module.html#include) the files from other folders.
type GeneratorImpl struct {
	usageReportConfig *ngfConfig.UsageReportConfig
	plus              bool
}

// NewGeneratorImpl creates a new GeneratorImpl.
func NewGeneratorImpl(
	plus bool,
	usageReportConfig *ngfConfig.UsageReportConfig,
) GeneratorImpl {
	return GeneratorImpl{
		plus:              plus,
		usageReportConfig: usageReportConfig,
	}
}

type executeResult struct {
	dest string
	data []byte
}

// executeFunc is a function that generates NGINX configuration from internal representation.
type executeFunc func(configuration dataplane.Configuration) []executeResult

// Generate generates NGINX configuration files from internal representation.
// It is the responsibility of the caller to validate the configuration before calling this function.
// In case of invalid configuration, NGINX will fail to reload or could be configured with malicious configuration.
// To validate, use the validators from the validation package.
func (g GeneratorImpl) Generate(logger logr.Logger, conf dataplane.Configuration) GenerateResult {
	var result GenerateResult

	for id, pair := range conf.SSLKeyPairs {
		result.ContentUpdateFiles = append(result.ContentUpdateFiles, generatePEM(id, pair.Cert, pair.Key))
	}

	policyGenerator := policies.NewCompositeGenerator(
		clientsettings.NewGenerator(),
		observability.NewGenerator(conf.Telemetry),
		snippetspolicy.NewGenerator(),
		proxysettings.NewGenerator(),
		ratelimit.NewGenerator(),
		waf.NewGenerator(),
		accesspolicy.NewGenerator(),
	)

	configApplyFiles, contentUpdateFiles := g.executeConfigTemplates(logger, conf, policyGenerator)
	result.ConfigApplyFiles = append(result.ConfigApplyFiles, configApplyFiles...)
	result.ContentUpdateFiles = append(result.ContentUpdateFiles, contentUpdateFiles...)

	for id, bundle := range conf.WAF.WAFBundles {
		result.ConfigApplyFiles = append(result.ConfigApplyFiles, generateWAFBundle(id, bundle))
	}

	for id, bundle := range conf.CertBundles {
		if id.IsCRLBundle() {
			result.ConfigApplyFiles = append(result.ConfigApplyFiles, generateCRLBundle(id, bundle))
		} else {
			result.ConfigApplyFiles = append(result.ConfigApplyFiles, generateCertBundle(id, bundle))
		}
	}

	for id, data := range conf.AuthSecrets {
		result.ContentUpdateFiles = append(result.ContentUpdateFiles, generateAuthFile(id, data))
	}
	return result
}

// GenerateDeploymentContext generates the deployment_ctx.json file needed for N+ licensing.
// It's exported since it's used by the init container process.
func (g GeneratorImpl) GenerateDeploymentContext(depCtx dataplane.DeploymentContext) (agent.File, error) {
	depCtxBytes, err := json.Marshal(depCtx)
	if err != nil {
		return agent.File{}, fmt.Errorf("error building deployment context for mgmt block: %w", err)
	}

	deploymentCtxFile := agent.File{
		Meta: &pb.FileMeta{
			Name:        mainIncludesFolder + "/deployment_ctx.json",
			Hash:        filesHelper.GenerateHash(depCtxBytes),
			Permissions: file.RegularFileMode,
			Size:        int64(len(depCtxBytes)),
		},
		Contents: depCtxBytes,
	}

	return deploymentCtxFile, nil
}

func (g GeneratorImpl) executeConfigTemplates(
	logger logr.Logger,
	conf dataplane.Configuration,
	generator policies.Generator,
) (configApplyFiles, contentUpdateFiles []agent.File) {
	fileBytes := make(map[string][]byte)

	httpUpstreams := g.createUpstreams(conf.Upstreams)
	keepAliveCheck := newKeepAliveChecker(httpUpstreams)

	for _, execute := range g.getExecuteFuncs(logger, generator, httpUpstreams, keepAliveCheck) {
		results := execute(conf)
		for _, res := range results {
			fileBytes[res.dest] = append(fileBytes[res.dest], res.data...)
		}
	}

	var mgmtConfigFiles, mgmtContentFiles []agent.File
	if g.plus {
		mgmtConfigFiles, mgmtContentFiles = g.generateMgmtFiles(logger, conf)
	}

	configFiles := make([]agent.File, 0, len(fileBytes)+len(mgmtConfigFiles))
	for fp, bytes := range fileBytes {
		configFiles = append(configFiles, agent.File{
			Meta: &pb.FileMeta{
				Name:        fp,
				Hash:        filesHelper.GenerateHash(bytes),
				Permissions: file.RegularFileMode,
				Size:        int64(len(bytes)),
			},
			Contents: bytes,
		})
	}
	configFiles = append(configFiles, mgmtConfigFiles...)

	return configFiles, mgmtContentFiles
}

func (g GeneratorImpl) getExecuteFuncs(
	logger logr.Logger,
	generator policies.Generator,
	upstreams []http.Upstream,
	keepAliveCheck keepAliveChecker,
) []executeFunc {
	return []executeFunc{
		newExecuteMainConfigFunc(generator),
		executeEventsConfig,
		newExecuteBaseHTTPConfigFunc(generator),
		g.newExecuteServersFunc(generator, keepAliveCheck, upstreams),
		newExecuteUpstreamsFunc(upstreams),
		executeSplitClients,
		executeMaps,
		executeTelemetry,
		g.newExecuteStreamServersFunc(generator, logger.WithName("streamServers")),
		g.executeStreamUpstreams,
		executeStreamMaps,
		executePlusAPI,
	}
}

func generatePEM(id dataplane.SSLKeyPairID, cert []byte, key []byte) agent.File {
	c := make([]byte, 0, len(cert)+len(key)+1)
	c = append(c, cert...)
	c = append(c, '\n')
	c = append(c, key...)

	return agent.File{
		Meta: &pb.FileMeta{
			Name:        generatePEMFileName(id),
			Hash:        filesHelper.GenerateHash(c),
			Permissions: file.SecretFileMode,
			Size:        int64(len(c)),
		},
		Contents: c,
	}
}

func generatePEMFileName(id dataplane.SSLKeyPairID) string {
	return filepath.Join(secretsFolder, string(id)+".pem")
}

func generateCertBundle(id dataplane.CertBundleID, cert []byte) agent.File {
	return agent.File{
		Meta: &pb.FileMeta{
			Name:        generateCertBundleFileName(id),
			Hash:        filesHelper.GenerateHash(cert),
			Permissions: file.SecretFileMode,
			Size:        int64(len(cert)),
		},
		Contents: cert,
	}
}

func generateCertBundleFileName(id dataplane.CertBundleID) string {
	return filepath.Join(secretsFolder, string(id)+".crt")
}

func generateCRLBundle(id dataplane.CertBundleID, crl []byte) agent.File {
	return agent.File{
		Meta: &pb.FileMeta{
			Name:        generateCRLBundleFileName(id),
			Hash:        filesHelper.GenerateHash(crl),
			Permissions: file.SecretFileMode,
			Size:        int64(len(crl)),
		},
		Contents: crl,
	}
}

func generateCRLBundleFileName(id dataplane.CertBundleID) string {
	return filepath.Join(secretsFolder, string(id)+".pem")
}

// generateAuthFile generates a file with data related to authentication in NGINX (e.g. auth_basic or auth_jwt).
func generateAuthFile(id dataplane.AuthFileID, data []byte) agent.File {
	return agent.File{
		Meta: &pb.FileMeta{
			Name:        generateAuthFileName(id),
			Hash:        filesHelper.GenerateHash(data),
			Permissions: file.SecretFileMode,
			Size:        int64(len(data)),
		},
		Contents: data,
	}
}

func generateAuthFileName(id dataplane.AuthFileID) string {
	return filepath.Join(secretsFolder, string(id))
}

func generateWAFBundle(id dataplane.WAFBundleID, bundle []byte) agent.File {
	return agent.File{
		Meta: &pb.FileMeta{
			Name:        GenerateWAFBundleFileName(id),
			Hash:        filesHelper.GenerateHash(bundle),
			Permissions: file.RegularFileMode,
			Size:        int64(len(bundle)),
		},
		Contents: bundle,
	}
}

func GenerateWAFBundleFileName(id dataplane.WAFBundleID) string {
	return filepath.Join(appProtectBundleFolder, string(id)+".tgz")
}
