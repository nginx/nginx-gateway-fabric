package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

var (
	mainConfigTemplate = gotemplate.Must(gotemplate.New("main").Parse(mainConfigTemplateText))
	mgmtConfigTemplate = gotemplate.Must(gotemplate.New("mgmt").Parse(mgmtConfigTemplateText))
)

type mainConfig struct {
	Includes []shared.Include
	Conf     dataplane.Configuration
}

func executeMainConfig(conf dataplane.Configuration) []executeResult {
	includes := createIncludesFromSnippets(conf.MainSnippets)

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

type mgmtConf struct {
	Endpoint          string
	Resolver          string
	LicenseTokenFile  string
	CACertFile        string
	ClientSSLCertFile string
	ClientSSLKeyFile  string
	SkipVerify        bool
}

// generateMgmtFiles generates the NGINX Plus configuration file for the mgmt block. As part of this,
// it writes the secret and deployment context files that are referenced in the mgmt block.
func (g GeneratorImpl) generateMgmtFiles(conf dataplane.Configuration) []file.File {
	if !g.plus {
		return nil
	}

	tokenContent, ok := conf.AuxiliarySecrets[graph.PlusReportJWTToken]
	if !ok {
		panic("nginx plus token not set in expected map")
	}

	tokenFile := file.File{
		Content: tokenContent,
		Path:    secretsFolder + "/license.jwt",
		Type:    file.TypeSecret,
	}
	files := []file.File{tokenFile}

	cfg := mgmtConf{
		Endpoint:         g.usageReportConfig.Endpoint,
		Resolver:         g.usageReportConfig.Resolver,
		LicenseTokenFile: tokenFile.Path,
		SkipVerify:       g.usageReportConfig.SkipVerify,
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportCACertificate]; ok {
		caFile := file.File{
			Content: content,
			Path:    secretsFolder + "/mgmt-ca.crt",
			Type:    file.TypeSecret,
		}
		cfg.CACertFile = caFile.Path
		files = append(files, caFile)
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportClientSSLCertificate]; ok {
		certFile := file.File{
			Content: content,
			Path:    secretsFolder + "/mgmt-tls.crt",
			Type:    file.TypeSecret,
		}
		cfg.ClientSSLCertFile = certFile.Path
		files = append(files, certFile)
	}

	if content, ok := conf.AuxiliarySecrets[graph.PlusReportClientSSLKey]; ok {
		keyFile := file.File{
			Content: content,
			Path:    secretsFolder + "/mgmt-tls.key",
			Type:    file.TypeSecret,
		}
		cfg.ClientSSLKeyFile = keyFile.Path
		files = append(files, keyFile)
	}

	deploymentCtxFile, err := g.GenerateDeploymentContext(conf.DeploymentContext)
	if err != nil {
		g.logger.Error(err, "error building deployment context for mgmt block")
	} else {
		files = append(files, deploymentCtxFile)
	}

	mgmtBlockFile := file.File{
		Content: helpers.MustExecuteTemplate(mgmtConfigTemplate, cfg),
		Path:    mgmtIncludesFile,
		Type:    file.TypeRegular,
	}

	return append(files, mgmtBlockFile)
}
