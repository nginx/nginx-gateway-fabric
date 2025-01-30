package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"

	ngfConfig "github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/clientsettings"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/observability"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Generator

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

	// secretsFolder is the folder where secrets (like TLS certs/keys) are stored.
	secretsFolder = configFolder + "/secrets"

	// includesFolder is the folder where are all include files are stored.
	includesFolder = configFolder + "/includes"

	// httpConfigFile is the path to the configuration file with HTTP configuration.
	httpConfigFile = httpFolder + "/http.conf"

	// streamConfigFile is the path to the configuration file with Stream configuration.
	streamConfigFile = streamFolder + "/stream.conf"

	// configVersionFile is the path to the config version configuration file.
	configVersionFile = httpFolder + "/config-version.conf"

	// httpMatchVarsFile is the path to the http_match pairs configuration file.
	httpMatchVarsFile = httpFolder + "/matches.json"

	// mainIncludesConfigFile is the path to the file containing NGINX configuration in the main context.
	mainIncludesConfigFile = mainIncludesFolder + "/main.conf"

	// mgmtIncludesFile is the path to the file containing the NGINX Plus mgmt config.
	mgmtIncludesFile = mainIncludesFolder + "/mgmt.conf"

	// nginxPlusConfigFile is the path to the file containing the NGINX Plus API config.
	nginxPlusConfigFile = httpFolder + "/plus-api.conf"
)

// ConfigFolders is a list of folders where NGINX configuration files are stored.
// Volumes here also need to be added to our crossplane ephemeral test container.
var ConfigFolders = []string{httpFolder, secretsFolder, includesFolder, mainIncludesFolder, streamFolder}

// Generator generates NGINX configuration files.
// This interface is used for testing purposes only.
type Generator interface {
	// Generate generates NGINX configuration files from internal representation.
	Generate(configuration dataplane.Configuration) []file.File
	// GenerateDeploymentContext generates the deployment context used for N+ licensing.
	GenerateDeploymentContext(depCtx dataplane.DeploymentContext) (file.File, error)
}

// GeneratorImpl is an implementation of Generator.
//
// It generates files to be written to the folders above, which must exist and available for writing.
//
// It also expects that the main NGINX configuration file nginx.conf is located in configFolder and nginx.conf
// includes (https://nginx.org/en/docs/ngx_core_module.html#include) the files from other folders.
type GeneratorImpl struct {
	usageReportConfig *ngfConfig.UsageReportConfig
	logger            logr.Logger
	plus              bool
}

// NewGeneratorImpl creates a new GeneratorImpl.
func NewGeneratorImpl(
	plus bool,
	usageReportConfig *ngfConfig.UsageReportConfig,
	logger logr.Logger,
) GeneratorImpl {
	return GeneratorImpl{
		plus:              plus,
		usageReportConfig: usageReportConfig,
		logger:            logger,
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
func (g GeneratorImpl) Generate(conf dataplane.Configuration) []file.File {
	files := make([]file.File, 0)

	for id, pair := range conf.SSLKeyPairs {
		files = append(files, generatePEM(id, pair.Cert, pair.Key))
	}

	policyGenerator := policies.NewCompositeGenerator(
		clientsettings.NewGenerator(),
		observability.NewGenerator(conf.Telemetry),
	)

	files = append(files, g.executeConfigTemplates(conf, policyGenerator)...)

	for id, bundle := range conf.CertBundles {
		files = append(files, generateCertBundle(id, bundle))
	}

	return files
}

// GenerateDeploymentContext generates the deployment_ctx.json file needed for N+ licensing.
// It's exported since it's used by the init container process.
func (g GeneratorImpl) GenerateDeploymentContext(depCtx dataplane.DeploymentContext) (file.File, error) {
	depCtxBytes, err := json.Marshal(depCtx)
	if err != nil {
		return file.File{}, fmt.Errorf("error building deployment context for mgmt block: %w", err)
	}

	deploymentCtxFile := file.File{
		Content: depCtxBytes,
		Path:    mainIncludesFolder + "/deployment_ctx.json",
		Type:    file.TypeRegular,
	}

	return deploymentCtxFile, nil
}

func (g GeneratorImpl) executeConfigTemplates(
	conf dataplane.Configuration,
	generator policies.Generator,
) []file.File {
	fileBytes := make(map[string][]byte)

	httpUpstreams := g.createUpstreams(conf.Upstreams, upstreamsettings.NewProcessor())
	keepAliveCheck := newKeepAliveChecker(httpUpstreams)

	for _, execute := range g.getExecuteFuncs(generator, httpUpstreams, keepAliveCheck) {
		results := execute(conf)
		for _, res := range results {
			fileBytes[res.dest] = append(fileBytes[res.dest], res.data...)
		}
	}

	var mgmtFiles []file.File
	if g.plus {
		mgmtFiles = g.generateMgmtFiles(conf)
	}

	files := make([]file.File, 0, len(fileBytes)+len(mgmtFiles))
	for fp, bytes := range fileBytes {
		files = append(files, file.File{
			Path:    fp,
			Content: bytes,
			Type:    file.TypeRegular,
		})
	}
	files = append(files, mgmtFiles...)

	return files
}

func (g GeneratorImpl) getExecuteFuncs(
	generator policies.Generator,
	upstreams []http.Upstream,
	keepAliveCheck keepAliveChecker,
) []executeFunc {
	return []executeFunc{
		executeMainConfig,
		executeBaseHTTPConfig,
		g.newExecuteServersFunc(generator, keepAliveCheck),
		newExecuteUpstreamsFunc(upstreams),
		executeSplitClients,
		executeMaps,
		executeTelemetry,
		g.executeStreamServers,
		g.executeStreamUpstreams,
		executeStreamMaps,
		executeVersion,
		executePlusAPI,
	}
}

func generatePEM(id dataplane.SSLKeyPairID, cert []byte, key []byte) file.File {
	c := make([]byte, 0, len(cert)+len(key)+1)
	c = append(c, cert...)
	c = append(c, '\n')
	c = append(c, key...)

	return file.File{
		Content: c,
		Path:    generatePEMFileName(id),
		Type:    file.TypeSecret,
	}
}

func generatePEMFileName(id dataplane.SSLKeyPairID) string {
	return filepath.Join(secretsFolder, string(id)+".pem")
}

func generateCertBundle(id dataplane.CertBundleID, cert []byte) file.File {
	return file.File{
		Content: cert,
		Path:    generateCertBundleFileName(id),
		Type:    file.TypeRegular,
	}
}

func generateCertBundleFileName(id dataplane.CertBundleID) string {
	return filepath.Join(secretsFolder, string(id)+".crt")
}
