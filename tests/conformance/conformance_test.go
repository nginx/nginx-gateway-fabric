//go:build conformance

/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package conformance

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/gateway-api/conformance"
	conf_v1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
	"sigs.k8s.io/gateway-api/conformance/tests"
	"sigs.k8s.io/gateway-api/conformance/utils/flags"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/yaml"
)

func TestConformance(t *testing.T) {
	g := NewWithT(t)

	t.Logf(`Running conformance tests with %s GatewayClass\n cleanup: %t\n`+
		`debug: %t\n enable all features: %t \n supported extended features: [%v]\n exempt features: [%v]\n`+
		`conformance profiles: [%v]\n skip tests: [%v]`,
		*flags.GatewayClassName, *flags.CleanupBaseResources, *flags.ShowDebug,
		*flags.EnableAllSupportedFeatures, *flags.SupportedFeatures, *flags.ExemptFeatures,
		*flags.ConformanceProfiles, *flags.SkipTests,
	)

	opts := conformance.DefaultOptions(t)
	opts.Implementation = conf_v1.Implementation{
		Organization: "nginx",
		Project:      "nginx-gateway-fabric",
		URL:          "https://github.com/nginx/nginx-gateway-fabric",
		Version:      *flags.ImplementationVersion,
		Contact: []string{
			"https://github.com/nginx/nginx-gateway-fabric/discussions/new/choose",
		},
	}

	testSuite, err := suite.NewConformanceTestSuite(opts)
	g.Expect(err).To(Not(HaveOccurred()))

	testSuite.Setup(t, tests.ConformanceTests)
	err = testSuite.Run(t, tests.ConformanceTests)
	g.Expect(err).To(Not(HaveOccurred()))

	report, err := testSuite.Report()
	g.Expect(err).To(Not(HaveOccurred()))

	yamlReport, err := yaml.Marshal(report)
	g.Expect(err).ToNot(HaveOccurred())

	f, err := os.Create(*flags.ReportOutput)
	g.Expect(err).ToNot(HaveOccurred())
	defer f.Close()

	_, err = f.WriteString("CONFORMANCE PROFILE\n")
	g.Expect(err).ToNot(HaveOccurred())

	_, err = f.Write(yamlReport)
	g.Expect(err).ToNot(HaveOccurred())
}
