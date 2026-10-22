package config

import (
	gotemplate "text/template"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var (
	mainConfigTemplate   = gotemplate.Must(gotemplate.New("main").Parse(mainConfigTemplateText))
	mgmtConfigTemplate   = gotemplate.Must(gotemplate.New("mgmt").Parse(mgmtConfigTemplateText))
	eventsConfigTemplate = gotemplate.Must(gotemplate.New("events").Parse(eventsConfigTemplateText))
)

type mainConfig struct {
	Includes []shared.Include
	Conf     dataplane.Configuration
}

func newExecuteMainConfigFunc(generator policies.Generator) executeFunc {
	return func(conf dataplane.Configuration) []executeResult {
		return executeMainConfig(conf, generator)
	}
}

func executeMainConfig(conf dataplane.Configuration, generator policies.Generator) []executeResult {
	includes := createIncludesFromSnippets(conf.MainSnippets)

	policyIncludes := createIncludesFromPolicyGenerateResult(generator.GenerateForMain(conf.Policies))
	includes = append(includes, policyIncludes...)

	mc := mainConfig{
		Conf:     conf,
		Includes: includes,
	}

	results := make([]executeResult, 0, len(includes)+1)
	results = append(results, executeResult{
		dest: mainIncludesConfigFile,
		data: helpers.MustExecuteTemplate(mainConfigTemplate, mc),
	})
	results = append(results, createIncludeExecuteResults(includes)...)

	return results
}

func executeEventsConfig(conf dataplane.Configuration) []executeResult {
	eventsData := helpers.MustExecuteTemplate(eventsConfigTemplate, conf)

	return []executeResult{
		{
			dest: eventsIncludesConfigFile,
			data: eventsData,
		},
	}
}

type mgmtConf struct {
	Endpoint             string
	Resolver             string
	LicenseTokenFile     string
	CACertFile           string
	ClientSSLCertFile    string
	ClientSSLKeyFile     string
	SkipVerify           bool
	EnforceInitialReport bool
}

// generateMgmtFiles generates the NGINX Plus configuration file for the mgmt block. As part of this,
// it writes the secret and deployment context files that are referenced in the mgmt block.
// It returns two slices: config apply files and content update files, both containing agent.File objects.
func (g GeneratorImpl) generateMgmtFiles(
	logger logr.Logger,
	conf dataplane.Configuration,
) (configApplyFiles []agent.File, contentUpdateFiles []agent.File) {
	if !g.plus {
		return nil, nil
	}

	tokenContent, ok := conf.AuxiliarySecrets[graph.PlusReportJWTToken]
	if !ok {
		panic("nginx plus token not set in expected map")
	}

	tokenFile := agent.File{
		Meta: &pb.FileMeta{
			Name:        secretsFolder + "/" + secrets.LicenseJWTKey,
			Hash:        filesHelper.GenerateHash(tokenContent),
			Permissions: file.SecretFileMode,
			Size:        int64(len(tokenContent)),
		},
		Contents: tokenContent,
	}
	configApplyFiles = []agent.File{tokenFile}

	cfg := mgmtConf{
		Endpoint:             g.usageReportConfig.Endpoint,
		Resolver:             g.usageReportConfig.Resolver,
		LicenseTokenFile:     tokenFile.Meta.Name,
		SkipVerify:           g.usageReportConfig.SkipVerify,
		EnforceInitialReport: g.usageReportConfig.EnforceInitialReport,
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportCACertificate]; ok {
		caFile := agent.File{
			Meta: &pb.FileMeta{
				Name:        MgmtCAFile,
				Hash:        filesHelper.GenerateHash(content),
				Permissions: file.SecretFileMode,
				Size:        int64(len(content)),
			},
			Contents: content,
		}
		cfg.CACertFile = caFile.Meta.Name
		configApplyFiles = append(configApplyFiles, caFile)
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportClientSSLCertificate]; ok {
		certFile := agent.File{
			Meta: &pb.FileMeta{
				Name:        MgmtClientSSLCertFile,
				Hash:        filesHelper.GenerateHash(content),
				Permissions: file.SecretFileMode,
				Size:        int64(len(content)),
			},
			Contents: content,
		}
		cfg.ClientSSLCertFile = certFile.Meta.Name
		contentUpdateFiles = append(contentUpdateFiles, certFile)
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportClientSSLKey]; ok {
		keyFile := agent.File{
			Meta: &pb.FileMeta{
				Name:        MgmtClientSSLKeyFile,
				Hash:        filesHelper.GenerateHash(content),
				Permissions: file.SecretFileMode,
				Size:        int64(len(content)),
			},
			Contents: content,
		}
		cfg.ClientSSLKeyFile = keyFile.Meta.Name
		contentUpdateFiles = append(contentUpdateFiles, keyFile)
	}

	deploymentCtxFile, err := g.GenerateDeploymentContext(conf.DeploymentContext)
	if err != nil {
		logger.Error(err, "Error building deployment context for mgmt block")
	} else {
		configApplyFiles = append(configApplyFiles, deploymentCtxFile)
	}

	mgmtContents := helpers.MustExecuteTemplate(mgmtConfigTemplate, cfg)
	mgmtBlockFile := agent.File{
		Meta: &pb.FileMeta{
			Name:        mgmtIncludesFile,
			Hash:        filesHelper.GenerateHash(mgmtContents),
			Permissions: file.RegularFileMode,
			Size:        int64(len(mgmtContents)),
		},
		Contents: mgmtContents,
	}
	configApplyFiles = append(configApplyFiles, mgmtBlockFile)

	return configApplyFiles, contentUpdateFiles
}
