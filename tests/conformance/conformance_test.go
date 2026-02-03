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

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordination "k8s.io/api/coordination/v1"
	core "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference_conformance "sigs.k8s.io/gateway-api-inference-extension/conformance"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	"sigs.k8s.io/gateway-api/conformance"
	conf_v1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
	"sigs.k8s.io/gateway-api/conformance/tests"
	"sigs.k8s.io/gateway-api/conformance/utils/flags"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/yaml"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

const (
	// unusableGatewayIPAddress 198.51.100.0 is a publicly reserved IP address specifically for documentation.
	// This is needed to give the conformance tests an example valid ip unusable address.
	unusableGatewayIPAddress = "198.51.100.0"

	// Default NGF namespace and release name for log collection
	ngfNamespace   = "nginx-gateway"
	ngfReleaseName = "nginx-gateway-fabric"
	infraNamespace = "gateway-conformance-infra"
)

func TestConformance(t *testing.T) {
	g := NewWithT(t)

	// Set up log collection on test failure
	defer collectNGFLogsOnFailure(t, g)

	t.Logf(`Running conformance tests with %s GatewayClass\n cleanup: %t\n`+
		`debug: %t\n enable all features: %t \n supported extended features: [%v]\n exempt features: [%v]\n`+
		`conformance profiles: [%v]\n skip tests: [%v]`,
		*flags.GatewayClassName, *flags.CleanupBaseResources, *flags.ShowDebug,
		*flags.EnableAllSupportedFeatures, *flags.SupportedFeatures, *flags.ExemptFeatures,
		*flags.ConformanceProfiles, *flags.SkipTests,
	)

	opts := conformance.DefaultOptions(t)

	ipaddressType := v1.IPAddressType
	opts.UnusableNetworkAddresses = []v1beta1.GatewaySpecAddress{{Type: &ipaddressType, Value: unusableGatewayIPAddress}}
	opts.UsableNetworkAddresses = []v1beta1.GatewaySpecAddress{{Type: &ipaddressType, Value: "192.0.2.1"}}

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

func TestInferenceExtensionConformance(t *testing.T) {
	g := NewWithT(t)

	// Set up log collection on test failure
	defer collectNGFLogsOnFailure(t, g)

	t.Logf(`Running inference conformance tests with %s GatewayClass\n cleanup: %t\n`+
		`debug: %t\n enable all features: %t \n supported extended features: [%v]\n exempt features: [%v]\n`+
		`skip tests: [%v]`,
		*flags.GatewayClassName, *flags.CleanupBaseResources, *flags.ShowDebug,
		*flags.EnableAllSupportedFeatures, *flags.SupportedFeatures, *flags.ExemptFeatures, *flags.SkipTests,
	)

	opts := inference_conformance.DefaultOptions(t)

	opts.Implementation = conf_v1.Implementation{
		Organization: "nginx",
		Project:      "nginx-gateway-fabric",
		URL:          "https://github.com/nginx/nginx-gateway-fabric",
		Version:      *flags.ImplementationVersion,
		Contact: []string{
			"https://github.com/nginx/nginx-gateway-fabric/discussions/new/choose",
		},
	}

	opts.ConformanceProfiles.Insert(inference_conformance.GatewayLayerProfileName)
	inference_conformance.RunConformanceWithOptions(t, opts)
}

// collectNGFLogsOnFailure collects NGF pod logs when tests fail
func collectNGFLogsOnFailure(t *testing.T, g Gomega) {
	if t.Failed() {
		t.Logf("Tests failed, collecting logs...")

		// Create a resource manager to access cluster resources
		k8sConfig := ctlr.GetConfigOrDie()
		scheme := k8sRuntime.NewScheme()
		g.Expect(core.AddToScheme(scheme)).To(Not(HaveOccurred()))
		g.Expect(apps.AddToScheme(scheme)).To(Not(HaveOccurred()))
		g.Expect(apiext.AddToScheme(scheme)).To(Not(HaveOccurred()))
		g.Expect(coordination.AddToScheme(scheme)).To(Not(HaveOccurred()))
		g.Expect(v1.Install(scheme)).To(Not(HaveOccurred()))
		g.Expect(batchv1.AddToScheme(scheme)).To(Not(HaveOccurred()))
		g.Expect(ngfAPIv1alpha1.AddToScheme(scheme)).To(Not(HaveOccurred()))
		g.Expect(ngfAPIv1alpha2.AddToScheme(scheme)).To(Not(HaveOccurred()))

		options := client.Options{
			Scheme: scheme,
		}
		k8sClient, err := client.New(k8sConfig, options)
		g.Expect(err).To(Not(HaveOccurred()))

		clientGoClient, err := kubernetes.NewForConfig(k8sConfig)
		g.Expect(err).To(Not(HaveOccurred()))

		timeoutConfig := framework.DefaultTimeoutConfig()
		rm := framework.ResourceManager{
			K8sClient:      k8sClient,
			ClientGoClient: clientGoClient,
			K8sConfig:      k8sConfig,
			TimeoutConfig:  timeoutConfig,
		}

		// Get NGF container logs
		collectLogs(t, g, rm, ngfNamespace, "nginx-gateway")

		// Get NGINX container logs
		collectLogs(t, g, rm, infraNamespace, "nginx")
	}
}

func collectLogs(t*testing.T,  g Gomega, rm framework.ResourceManager, namespace, containerName string) {
	t.Helper()

	pods, err := rm.GetPods(namespace, nil)
	g.Expect(err).To(Not(HaveOccurred()))

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				containerLogs, err := rm.GetPodLogs(pod.Namespace, pod.Name, &core.PodLogOptions{
					Container: container.Name,
				})
				if err != nil {
					t.Logf("Failed to get %s container logs from pod %s: %v", container.Name, pod.Name, err)
				} else {
					t.Logf("Container %s logs for pod %s:\n%s", container.Name, pod.Name, containerLogs)
				}
			}
		}
	}
}
