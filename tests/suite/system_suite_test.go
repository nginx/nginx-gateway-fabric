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

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
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
	nginxImageJWTFileName    = flag.String("nginx-image-jwt-file-name", "", "Name of JWT file name for N+ images")
	wafEnabled               = flag.Bool("waf-enabled", false, "Is NAP WAF enabled (NGINX Plus with WAF images)")
	serviceType              = flag.String("service-type", "NodePort", "Type of service fronting NGF to be deployed")
	plusEnabled              = flag.Bool("plus-enabled", false, "Is NGINX Plus enabled")
	plusLicenseFileName      = flag.String("plus-license-file-name", "", "File name containing the NGINX Plus JWT")
	plusUsageEndpoint        = flag.String("plus-usage-endpoint", "", "Endpoint for reporting NGINX Plus usage")
	clusterName              = flag.String("cluster-name", "kind", "Cluster name")
	gkeProject               = flag.String("gke-project", "", "GKE Project name")
)

var (
	//go:embed manifests/*
	manifests           embed.FS
	k8sClient           client.Client
	resourceManager     framework.ResourceManager
	portForwardStopCh   chan struct{}
	portForwardDoneCh   chan struct{}
	portFwdPort         int
	portFwdHTTPSPort    int
	timeoutConfig       framework.TimeoutConfig
	localChartPath      string
	address             string
	version             string
	chartVersion        string
	clusterInfo         framework.ClusterInfo
	skipNFRTests        bool
	logs                string
	nginxCrossplanePath string
)

var formatNginxPlusEdgeImagePath = "us-docker.pkg.dev/%s/nginx-gateway-fabric/nginx-plus"

var (
	releaseName           string
	ngfNamespace          string
	gatewayClassName      string
	ngfHTTPForwardedPort  int // base HTTP port for this proc
	ngfHTTPSForwardedPort int // base HTTPS port for this proc
	portCounter           int // increments per setUpPortForward call; ensures each test gets unique ports
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

	if *gkeProject != "" {
		nginxCrossplanePath = "us-docker.pkg.dev/" + *gkeProject + "/nginx-gateway-fabric"
	}

	// Set text replacements for per-proc resource names so manifests reference the correct
	// GatewayClass and NginxGateway config for this parallel process.
	// Longevity installs NGF with the default "nginx" class, so no replacement is needed.
	if !strings.Contains(GinkgoLabelFilter(), "longevity") {
		resourceManager.TextReplacements = map[string]string{
			"gatewayClassName: nginx": fmt.Sprintf("gatewayClassName: %s", gatewayClassName),
			"name: ngf-test-config":   fmt.Sprintf("name: %s-config", releaseName),
		}
	}

	if !cfg.deploy {
		return
	}

	installCfg := createNGFInstallConfig(cfg, extraInstallArgs...)

	podNames, err := resourceManager.GetReadyNGFPodNames(
		installCfg.Namespace,
		installCfg.ReleaseName,
		timeoutConfig.CreateTimeout,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(podNames).ToNot(BeEmpty())
}

func setUpPortForward(nginxPodName, nginxNamespace string) {
	var err error

	if *serviceType != "LoadBalancer" {
		httpPort := ngfHTTPForwardedPort + portCounter
		httpsPort := ngfHTTPSForwardedPort + portCounter
		portCounter++

		GinkgoWriter.Printf("Service Type: %s, HTTP port: %d, HTTPS port: %d\n", *serviceType, httpPort, httpsPort)
		ports := []string{fmt.Sprintf("%d:80", httpPort), fmt.Sprintf("%d:443", httpsPort)}
		portForwardStopCh = make(chan struct{})
		portForwardDoneCh, err = framework.PortForward(
			resourceManager.K8sConfig,
			nginxNamespace,
			nginxPodName,
			ports,
			portForwardStopCh,
		)
		if err == nil {
			address = "127.0.0.1"
			portFwdPort = httpPort
			portFwdHTTPSPort = httpsPort
		}
	} else {
		GinkgoWriter.Printf("Service Type: LoadBalancer\n")
		address, err = resourceManager.GetLBIPAddress(nginxNamespace)
	}
	Expect(err).ToNot(HaveOccurred())
}

// cleanUpPortForward closes the port forward channel and needs to be called before deleting any gateways or else
// the logs will be flooded with port forward errors.
func cleanUpPortForward() {
	GinkgoWriter.Printf("Cleaning up port forward\n")
	if portFwdPort != 0 {
		close(portForwardStopCh)
		<-portForwardDoneCh
		portFwdPort = 0
		portFwdHTTPSPort = 0
	}
}

func createNGFInstallConfig(cfg setupConfig, extraInstallArgs ...string) framework.InstallationConfig {
	GinkgoWriter.Printf("Creating NGF installation config\n")
	installCfg := framework.InstallationConfig{
		ReleaseName:          cfg.releaseName,
		Namespace:            ngfNamespace,
		ChartPath:            cfg.chartPath,
		ServiceType:          *serviceType,
		Plus:                 *plusEnabled,
		PlusUsageEndpoint:    *plusUsageEndpoint,
		Telemetry:            cfg.telemetry,
		GatewayClassName:     gatewayClassName,
		NginxImagePullSecret: *nginxImageJWTFileName,
	}

	switch {
	// if we aren't installing from the public charts, then set the custom images
	case !strings.HasPrefix(cfg.chartPath, "oci://"):
		GinkgoWriter.Printf("Installing chart from local directory\n")
		installCfg.NgfImageRepository = *ngfImageRepository
		installCfg.NginxImageRepository = *nginxImageRepository
		if *plusEnabled && cfg.nfr {
			installCfg.NginxImageRepository = *nginxPlusImageRepository
		}
		installCfg.ImageTag = *imageTag
		installCfg.ImagePullPolicy = *imagePullPolicy
	case *plusEnabled && cfg.nfr:
		installCfg.NginxImageRepository = fmt.Sprintf(formatNginxPlusEdgeImagePath, *gkeProject)
	}

	if cfg.debugLogLevel {
		extraInstallArgs = append(
			extraInstallArgs,
			"--set", "nginxGateway.config.logging.level=debug",
			"--set", "nginx.config.logging.agentLevel=debug",
		)
	}

	if *plusEnabled {
		Expect(framework.CreateLicenseSecret(resourceManager, ngfNamespace, *plusLicenseFileName)).To(Succeed())
		if *nginxImageJWTFileName != "" {
			Expect(framework.CreateImagePullSecret(resourceManager, ngfNamespace, *nginxImageJWTFileName)).To(Succeed())
			extraInstallArgs = append(
				extraInstallArgs,
				"--set", "nginx.imagePullSecret="+framework.PlusImagePullSecretName,
			)
		}
	}

	output, err := framework.InstallNGF(installCfg, extraInstallArgs...)
	Expect(err).ToNot(HaveOccurred(), string(output))

	return installCfg
}

func teardown(relName string) {
	cfg := framework.InstallationConfig{
		ReleaseName:    relName,
		Namespace:      ngfNamespace,
		SkipCRDCleanup: true,
	}

	output, err := framework.UninstallNGF(cfg, resourceManager)
	Expect(err).ToNot(HaveOccurred(), string(output))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	Expect(wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			key := k8sTypes.NamespacedName{Name: ngfNamespace}
			if err := resourceManager.Get(
				ctx,
				key,
				&core.Namespace{},
			); err != nil && apierrors.IsNotFound(err) {
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

var _ = SynchronizedBeforeSuite(
	// Phase 1: runs on proc 1 only. Install cluster-wide CRDs once before all procs start helm installs.
	func() []byte {
		_, file, _, _ := runtime.Caller(0)
		fileDir := path.Join(path.Dir(file), "../")
		basepath := filepath.Dir(fileDir)
		chartPath := filepath.Join(basepath, "charts/nginx-gateway-fabric")

		output, err := framework.InstallGatewayAPI(*gatewayAPIVersion)
		Expect(err).ToNot(HaveOccurred(), string(output))

		output, err = framework.InstallNGFCRDs(chartPath)
		Expect(err).ToNot(HaveOccurred(), string(output))

		return nil
	},
	// Phase 2: runs on all procs after phase 1 completes.
	func(_ []byte) {
		proc := GinkgoParallelProcess()
		releaseName = fmt.Sprintf("ngf-test-%d", proc)
		ngfNamespace = fmt.Sprintf("nginx-gateway-%d", proc)
		gatewayClassName = fmt.Sprintf("nginx-%d", proc)
		// Each proc gets a 100-port range so tests can use unique ports without reuse.
		// HTTP:  proc 1=10080-10179, proc 2=10180-10279, ..., proc 8=10780-10879
		// HTTPS: proc 1=11080-11179, proc 2=11180-11279, ..., proc 8=11780-11879
		ngfHTTPForwardedPort = 10080 + (proc-1)*100
		ngfHTTPSForwardedPort = 11080 + (proc-1)*100

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
		// logs from GKE. Also reset gateway class and namespace to defaults since longevity runs standalone.
		if strings.Contains(labelFilter, "longevity") {
			cfg.releaseName = "ngf-longevity"
			releaseName = cfg.releaseName
			gatewayClassName = "nginx"
			ngfNamespace = "nginx-gateway"
		}

		setup(cfg)
	},
)

var _ = SynchronizedAfterSuite(
	// Phase 1: runs on all procs. Per-proc helm uninstall and namespace cleanup.
	func() {
		if skipNFRTests {
			Skip("")
		}
		events := framework.GetEvents(resourceManager, ngfNamespace)
		AddReportEntry("Events", events, ReportEntryVisibilityNever)

		logs = framework.GetLogs(resourceManager, ngfNamespace, releaseName)
		AddReportEntry("NGF Logs", logs, ReportEntryVisibilityNever)

		labelFilter := GinkgoLabelFilter()
		if !strings.Contains(labelFilter, "longevity-setup") {
			relName := releaseName
			if strings.Contains(labelFilter, "longevity-teardown") {
				relName = "ngf-longevity"
			}

			teardown(relName)
		}
	},
	// Phase 2: runs on proc 1 only, after all procs complete phase 1.
	// Delete cluster-wide CRDs once all per-proc helm releases are gone.
	// Skip for longevity-setup: the environment must remain intact for the duration of the longevity run.
	func() {
		if strings.Contains(GinkgoLabelFilter(), "longevity-setup") {
			return
		}

		Expect(framework.DeleteNGFCRDs(resourceManager)).To(Succeed())

		output, err := framework.UninstallGatewayAPI(*gatewayAPIVersion)
		Expect(err).ToNot(HaveOccurred(), string(output))
	},
)

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
