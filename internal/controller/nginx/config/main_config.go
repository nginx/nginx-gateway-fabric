package config

import (
	gotemplate "text/template"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
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
func (g GeneratorImpl) generateMgmtFiles(conf dataplane.Configuration) []agent.File {
	if !g.plus {
		return nil
	}

	tokenContent, ok := conf.AuxiliarySecrets[graph.PlusReportJWTToken]
	if !ok {
		panic("nginx plus token not set in expected map")
	}

	tokenFile := agent.File{
		Meta: &pb.FileMeta{
			Name:        secretsFolder + "/license.jwt",
			Hash:        filesHelper.GenerateHash(tokenContent),
			Permissions: file.SecretFileMode,
			Size:        int64(len(tokenContent)),
		},
		Contents: tokenContent,
	}
	files := []agent.File{tokenFile}

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
				Name:        secretsFolder + "/mgmt-ca.crt",
				Hash:        filesHelper.GenerateHash(content),
				Permissions: file.SecretFileMode,
				Size:        int64(len(content)),
			},
			Contents: content,
		}
		cfg.CACertFile = caFile.Meta.Name
		files = append(files, caFile)
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportClientSSLCertificate]; ok {
		certFile := agent.File{
			Meta: &pb.FileMeta{
				Name:        secretsFolder + "/mgmt-tls.crt",
				Hash:        filesHelper.GenerateHash(content),
				Permissions: file.SecretFileMode,
				Size:        int64(len(content)),
			},
			Contents: content,
		}
		cfg.ClientSSLCertFile = certFile.Meta.Name
		files = append(files, certFile)
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportClientSSLKey]; ok {
		keyFile := agent.File{
			Meta: &pb.FileMeta{
				Name:        secretsFolder + "/mgmt-tls.key",
				Hash:        filesHelper.GenerateHash(content),
				Permissions: file.SecretFileMode,
				Size:        int64(len(content)),
			},
			Contents: content,
		}
		cfg.ClientSSLKeyFile = keyFile.Meta.Name
		files = append(files, keyFile)
	}

	deploymentCtxFile, err := g.GenerateDeploymentContext(conf.DeploymentContext)
	if err != nil {
		g.logger.Error(err, "error building deployment context for mgmt block")
	} else {
		files = append(files, deploymentCtxFile)
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

	return append(files, mgmtBlockFile)
}
