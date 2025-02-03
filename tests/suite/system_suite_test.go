// This package needs to be named main to get build info
// because of https://github.com/golang/go/issues/33976
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordination "k8s.io/api/coordination/v1"
	core "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

func TestNGF(t *testing.T) {
	t.Parallel()
	flag.Parse()
	if *gatewayAPIVersion == "" {
		panic("Gateway API version must be set")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NGF System Tests")
}

var (
	gatewayAPIVersion     = flag.String("gateway-api-version", "", "Supported Gateway API version for NGF under test")
	gatewayAPIPrevVersion = flag.String(
		"gateway-api-prev-version", "", "Supported Gateway API version for previous NGF release",
	)
	// Configurable NGF installation variables. Helm values will be used as defaults if not specified.
	ngfImageRepository       = flag.String("ngf-image-repo", "", "Image repo for NGF control plane")
	nginxImageRepository     = flag.String("nginx-image-repo", "", "Image repo for NGF data plane")
	nginxPlusImageRepository = flag.String("nginx-plus-image-repo", "", "Image repo for NGF N+ data plane")
	imageTag                 = flag.String("image-tag", "", "Image tag for NGF images")
	versionUnderTest         = flag.String("version-under-test", "", "Version of NGF that is being tested")
	imagePullPolicy          = flag.String("pull-policy", "", "Image pull policy for NGF images")
	serviceType              = flag.String("service-type", "NodePort", "Type of service fronting NGF to be deployed")
	isGKEInternalLB          = flag.Bool("is-gke-internal-lb", false, "Is the LB service GKE internal only")
	plusEnabled              = flag.Bool("plus-enabled", false, "Is NGINX Plus enabled")
	plusLicenseFileName      = flag.String("plus-license-file-name", "", "File name containing the NGINX Plus JWT")
	plusUsageEndpoint        = flag.String("plus-usage-endpoint", "", "Endpoint for reporting NGINX Plus usage")
	clusterName              = flag.String("cluster-name", "kind", "Cluster name")
	gkeProject               = flag.String("gke-project", "", "GKE Project name")
)

var (
	//go:embed manifests/*
	manifests         embed.FS
	k8sClient         client.Client
	resourceManager   framework.ResourceManager
	portForwardStopCh chan struct{}
	portFwdPort       int
	portFwdHTTPSPort  int
	timeoutConfig     framework.TimeoutConfig
	localChartPath    string
	address           string
	version           string
	chartVersion      string
	clusterInfo       framework.ClusterInfo
	skipNFRTests      bool
	logs              string
)

var formatNginxPlusEdgeImagePath = "us-docker.pkg.dev/%s/nginx-gateway-fabric/nginx-plus"

const (
	releaseName           = "ngf-test"
	ngfNamespace          = "nginx-gateway"
	gatewayClassName      = "nginx"
	ngfHTTPForwardedPort  = 10080
	ngfHTTPSForwardedPort = 10443
	ngfControllerName     = "gateway.nginx.org/nginx-gateway-controller"
)

type setupConfig struct {
	releaseName   string
	chartPath     string
	gwAPIVersion  string
	deploy        bool
	nfr           bool
	debugLogLevel bool
	telemetry     bool
}

func setup(cfg setupConfig, extraInstallArgs ...string) {
	log.SetLogger(GinkgoLogr)

	k8sConfig := ctlr.GetConfigOrDie()
	scheme := k8sRuntime.NewScheme()
	Expect(core.AddToScheme(scheme)).To(Succeed())
	Expect(apps.AddToScheme(scheme)).To(Succeed())
	Expect(apiext.AddToScheme(scheme)).To(Succeed())
	Expect(coordination.AddToScheme(scheme)).To(Succeed())
	Expect(v1.Install(scheme)).To(Succeed())
	Expect(batchv1.AddToScheme(scheme)).To(Succeed())
	Expect(ngfAPIv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(ngfAPIv1alpha2.AddToScheme(scheme)).To(Succeed())

	options := client.Options{
		Scheme: scheme,
	}

	var err error
	k8sClient, err = client.New(k8sConfig, options)
	Expect(err).ToNot(HaveOccurred())

	clientGoClient, err := kubernetes.NewForConfig(k8sConfig)
	Expect(err).ToNot(HaveOccurred())

	timeoutConfig = framework.DefaultTimeoutConfig()
	resourceManager = framework.ResourceManager{
		K8sClient:      k8sClient,
		ClientGoClient: clientGoClient,
		K8sConfig:      k8sConfig,
		FS:             manifests,
		TimeoutConfig:  timeoutConfig,
	}

	clusterInfo, err = resourceManager.GetClusterInfo()
	Expect(err).ToNot(HaveOccurred())

	if cfg.nfr && !clusterInfo.IsGKE {
		skipNFRTests = true
		Skip("NFR tests can only run in GKE")
	}

	if cfg.nfr && *serviceType != "LoadBalancer" {
		skipNFRTests = true
		Skip("GW_SERVICE_TYPE must be 'LoadBalancer' for NFR tests")
	}

	if clusterInfo.IsGKE && strings.Contains(GinkgoLabelFilter(), "graceful-recovery") {
		Skip("Graceful Recovery test must be run on Kind")
	}

	if clusterInfo.IsGKE && strings.Contains(GinkgoLabelFilter(), "longevity") {
		cfg.telemetry = true
	}

	switch {
	case *versionUnderTest != "":
		version = *versionUnderTest
	case *imageTag != "":
		version = *imageTag
	default:
		version = "edge"
	}

	if !cfg.deploy {
		return
	}

	installCfg := createNGFInstallConfig(cfg, extraInstallArgs...)

	podNames, err := framework.GetReadyNGFPodNames(
		k8sClient,
		installCfg.Namespace,
		installCfg.ReleaseName,
		timeoutConfig.CreateTimeout,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(podNames).ToNot(BeEmpty())

	if *serviceType != "LoadBalancer" {
		ports := []string{fmt.Sprintf("%d:80", ngfHTTPForwardedPort), fmt.Sprintf("%d:443", ngfHTTPSForwardedPort)}
		portForwardStopCh = make(chan struct{})
		err = framework.PortForward(k8sConfig, installCfg.Namespace, podNames[0], ports, portForwardStopCh)
		address = "127.0.0.1"
		portFwdPort = ngfHTTPForwardedPort
		portFwdHTTPSPort = ngfHTTPSForwardedPort
	} else {
		address, err = resourceManager.GetLBIPAddress(installCfg.Namespace)
	}
	Expect(err).ToNot(HaveOccurred())
}

func createNGFInstallConfig(cfg setupConfig, extraInstallArgs ...string) framework.InstallationConfig {
	installCfg := framework.InstallationConfig{
		ReleaseName:       cfg.releaseName,
		Namespace:         ngfNamespace,
		ChartPath:         cfg.chartPath,
		ServiceType:       *serviceType,
		IsGKEInternalLB:   *isGKEInternalLB,
		Plus:              *plusEnabled,
		PlusUsageEndpoint: *plusUsageEndpoint,
		Telemetry:         cfg.telemetry,
	}

	switch {
	// if we aren't installing from the public charts, then set the custom images
	case !strings.HasPrefix(cfg.chartPath, "oci://"):
		installCfg.NgfImageRepository = *ngfImageRepository
		installCfg.NginxImageRepository = *nginxImageRepository
		if *plusEnabled && cfg.nfr {
			installCfg.NginxImageRepository = *nginxPlusImageRepository
		}
		installCfg.ImageTag = *imageTag
		installCfg.ImagePullPolicy = *imagePullPolicy
	case version == "edge":
		chartVersion = "0.0.0-edge"
		installCfg.ChartVersion = chartVersion
		if *plusEnabled && cfg.nfr {
			installCfg.NginxImageRepository = fmt.Sprintf(formatNginxPlusEdgeImagePath, *gkeProject)
		}
	case *plusEnabled && cfg.nfr:
		installCfg.NginxImageRepository = fmt.Sprintf(formatNginxPlusEdgeImagePath, *gkeProject)
	}

	output, err := framework.InstallGatewayAPI(cfg.gwAPIVersion)
	Expect(err).ToNot(HaveOccurred(), string(output))

	if cfg.debugLogLevel {
		extraInstallArgs = append(
			extraInstallArgs,
			"--set", "nginxGateway.config.logging.level=debug",
		)
	}

	if *plusEnabled {
		Expect(framework.CreateLicenseSecret(k8sClient, ngfNamespace, *plusLicenseFileName)).To(Succeed())
	}

	output, err = framework.InstallNGF(installCfg, extraInstallArgs...)
	Expect(err).ToNot(HaveOccurred(), string(output))

	return installCfg
}

func teardown(relName string) {
	if portFwdPort != 0 {
		close(portForwardStopCh)
		portFwdPort = 0
		portFwdHTTPSPort = 0
	}

	cfg := framework.InstallationConfig{
		ReleaseName: relName,
		Namespace:   ngfNamespace,
	}

	output, err := framework.UninstallNGF(cfg, k8sClient)
	Expect(err).ToNot(HaveOccurred(), string(output))

	output, err = framework.UninstallGatewayAPI(*gatewayAPIVersion)
	Expect(err).ToNot(HaveOccurred(), string(output))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	Expect(wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			key := k8sTypes.NamespacedName{Name: ngfNamespace}
			if err := k8sClient.Get(ctx, key, &core.Namespace{}); err != nil && apierrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)).To(Succeed())
}

func getDefaultSetupCfg() setupConfig {
	_, file, _, _ := runtime.Caller(0)
	fileDir := path.Join(path.Dir(file), "../")
	basepath := filepath.Dir(fileDir)
	localChartPath = filepath.Join(basepath, "charts/nginx-gateway-fabric")

	return setupConfig{
		releaseName:   releaseName,
		chartPath:     localChartPath,
		gwAPIVersion:  *gatewayAPIVersion,
		deploy:        true,
		debugLogLevel: true,
	}
}

var _ = BeforeSuite(func() {
	cfg := getDefaultSetupCfg()

	labelFilter := GinkgoLabelFilter()
	cfg.nfr = isNFR(labelFilter)

	// Skip deployment if:
	skipSubstrings := []string{
		"upgrade",            // - running upgrade test (this test will deploy its own version)
		"longevity-teardown", // - running longevity teardown (deployment will already exist)
		"telemetry",          // - running telemetry test (NGF will be deployed as part of the test)
		"scale",              // - running scale test (this test will deploy its own version)
		"reconfiguration",    // - running reconfiguration test (test will deploy its own instances)
	}
	for _, s := range skipSubstrings {
		if strings.Contains(labelFilter, s) {
			cfg.deploy = false
			break
		}
	}

	// use a different release name for longevity to allow us to filter on a specific label when collecting
	// logs from GKE
	if strings.Contains(labelFilter, "longevity") {
		cfg.releaseName = "ngf-longevity"
	}

	setup(cfg)
})

var _ = AfterSuite(func() {
	if skipNFRTests {
		Skip("")
	}
	events := framework.GetEvents(resourceManager, ngfNamespace)
	AddReportEntry("Events", events, ReportEntryVisibilityNever)

	logs = framework.GetLogs(resourceManager, ngfNamespace, releaseName)
	AddReportEntry("Logs", logs, ReportEntryVisibilityNever)

	labelFilter := GinkgoLabelFilter()
	if !strings.Contains(labelFilter, "longevity-setup") {
		relName := releaseName
		if strings.Contains(labelFilter, "longevity-teardown") {
			relName = "ngf-longevity"
		}

		teardown(relName)
	}
})

func isNFR(labelFilter string) bool {
	return strings.Contains(labelFilter, "nfr") ||
		strings.Contains(labelFilter, "longevity") ||
		strings.Contains(labelFilter, "performance") ||
		strings.Contains(labelFilter, "upgrade") ||
		strings.Contains(labelFilter, "scale") ||
		strings.Contains(labelFilter, "reconfiguration")
}

var _ = ReportAfterSuite("Print info on failure", func(report Report) {
	if !report.SuiteSucceeded {
		for _, specReport := range report.SpecReports {
			for _, entry := range specReport.ReportEntries {
				fmt.Println(entry.GetRawValue())
			}
		}
	}
})
