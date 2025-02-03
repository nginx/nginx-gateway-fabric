package config

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

func TestExecuteMainConfig_Telemetry(t *testing.T) {
	t.Parallel()

	telemetryOff := dataplane.Configuration{
		Telemetry: dataplane.Telemetry{},
	}
	telemetryOn := dataplane.Configuration{
		Telemetry: dataplane.Telemetry{
			Endpoint: "endpoint",
		},
	}
	loadModuleDirective := "load_module modules/ngx_otel_module.so;"

	tests := []struct {
		name                   string
		conf                   dataplane.Configuration
		expLoadModuleDirective bool
	}{
		{
			name:                   "telemetry off",
			conf:                   telemetryOff,
			expLoadModuleDirective: false,
		},
		{
			name:                   "telemetry on",
			conf:                   telemetryOn,
			expLoadModuleDirective: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeMainConfig(test.conf)
			g.Expect(res).To(HaveLen(1))
			g.Expect(res[0].dest).To(Equal(mainIncludesConfigFile))
			if test.expLoadModuleDirective {
				g.Expect(res[0].data).To(ContainSubstring(loadModuleDirective))
			} else {
				g.Expect(res[0].data).ToNot(ContainSubstring(loadModuleDirective))
			}
		})
	}
}

func TestExecuteMainConfig_Logging(t *testing.T) {
	t.Parallel()

	conf := dataplane.Configuration{
		Logging: dataplane.Logging{
			ErrorLevel: "info",
		},
	}

	g := NewWithT(t)

	res := executeMainConfig(conf)
	g.Expect(res).To(HaveLen(1))
	g.Expect(res[0].dest).To(Equal(mainIncludesConfigFile))

	g.Expect(string(res[0].data)).To(ContainSubstring("error_log stderr info"))
}

func TestExecuteMainConfig_Snippets(t *testing.T) {
	t.Parallel()

	conf := dataplane.Configuration{
		MainSnippets: []dataplane.Snippet{
			{
				Name:     "snippet1",
				Contents: "contents1",
			},
			{
				Name:     "snippet2",
				Contents: "contents2",
			},
			{
				Name:     "snippet3",
				Contents: "contents3",
			},
		},
	}

	g := NewWithT(t)

	res := executeMainConfig(conf)
	g.Expect(res).To(HaveLen(4))

	// sort results by filename
	sort.Slice(
		res, func(i, j int) bool {
			return res[i].dest < res[j].dest
		},
	)

	/*
		Order of files:
		/etc/nginx/includes/snippet1.conf
		/etc/nginx/includes/snippet2.conf
		/etc/nginx/includes/snippet3.conf
		/etc/nginx/main-includes/main.conf
	*/

	g.Expect(res[0].dest).To(Equal("/etc/nginx/includes/snippet1.conf"))
	g.Expect(string(res[0].data)).To(ContainSubstring("contents1"))

	g.Expect(res[1].dest).To(Equal("/etc/nginx/includes/snippet2.conf"))
	g.Expect(string(res[1].data)).To(ContainSubstring("contents2"))

	g.Expect(res[2].dest).To(Equal("/etc/nginx/includes/snippet3.conf"))
	g.Expect(string(res[2].data)).To(ContainSubstring("contents3"))

	g.Expect(res[3].dest).To(Equal(mainIncludesConfigFile))
}

func TestGenerateMgmtFiles_NoPlus(t *testing.T) {
	t.Parallel()

	gen := GeneratorImpl{}
	files := gen.generateMgmtFiles(dataplane.Configuration{})

	g := NewWithT(t)
	g.Expect(files).To(BeNil())
}

func TestGenerateMgmtFiles_Panic(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}

	// panics if JWT token is not set in the AuxiliarySecrets map
	g.Expect(func() {
		gen.generateMgmtFiles(dataplane.Configuration{})
	}).To(Panic())
}
