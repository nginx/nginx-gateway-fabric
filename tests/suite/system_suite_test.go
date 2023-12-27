package suite

import (
	"embed"
	"flag"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginxinc/nginx-gateway-fabric/tests/framework"
)

func TestNGF(t *testing.T) {
	flag.Parse()
	if *gatewayAPIVersion == "" {
		panic("Gateway API version must be set")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NGF System Tests")
}

var (
	gatewayAPIVersion = flag.String("gateway-api-version", "", "Version of Gateway API to install")
	k8sVersion        = flag.String("k8s-version", "latest", "Version of k8s being tested on")
	// Configurable NGF installation variables. Helm values will be used as defaults if not specified.
	ngfImageRepository   = flag.String("ngf-image-repo", "", "Image repo for NGF control plane")
	nginxImageRepository = flag.String("nginx-image-repo", "", "Image repo for NGF data plane")
	imageTag             = flag.String("image-tag", "", "Image tag for NGF images")
	imagePullPolicy      = flag.String("pull-policy", "", "Image pull policy for NGF images")
	serviceType          = flag.String("service-type", "NodePort", "Type of service fronting NGF to be deployed")
	isGKEInternalLB      = flag.Bool("is-gke-internal-lb", false, "Is the LB service GKE internal only")
)

var (
	//go:embed manifests/*
	manifests         embed.FS
	k8sClient         client.Client
	resourceManager   framework.ResourceManager
	portForwardStopCh = make(chan struct{}, 1)
	portFwdPort       int
	timeoutConfig     framework.TimeoutConfig
	address           string
	version           string
	clusterInfo       framework.ClusterInfo
)

var _ = BeforeSuite(func() {
	k8sConfig := ctlr.GetConfigOrDie()
	scheme := k8sRuntime.NewScheme()
	Expect(core.AddToScheme(scheme)).To(Succeed())
	Expect(apps.AddToScheme(scheme)).To(Succeed())
	Expect(apiext.AddToScheme(scheme)).To(Succeed())
	Expect(v1.AddToScheme(scheme)).To(Succeed())

	options := client.Options{
		Scheme: scheme,
	}

	var err error
	k8sClient, err = client.New(k8sConfig, options)
	Expect(err).ToNot(HaveOccurred())

	timeoutConfig = framework.DefaultTimeoutConfig()
	resourceManager = framework.ResourceManager{
		K8sClient:     k8sClient,
		FS:            manifests,
		TimeoutConfig: timeoutConfig,
	}

	_, file, _, _ := runtime.Caller(0)
	fileDir := path.Join(path.Dir(file), "../")
	basepath := filepath.Dir(fileDir)

	cfg := framework.InstallationConfig{
		ReleaseName:          "ngf-test",
		Namespace:            "nginx-gateway",
		ChartPath:            filepath.Join(basepath, "deploy/helm-chart"),
		NgfImageRepository:   *ngfImageRepository,
		NginxImageRepository: *nginxImageRepository,
		ImageTag:             *imageTag,
		ImagePullPolicy:      *imagePullPolicy,
		ServiceType:          *serviceType,
		IsGKEInternalLB:      *isGKEInternalLB,
	}

	if *imageTag != "" {
		version = *imageTag
	} else {
		version = "edge"
	}

	clusterInfo, err = resourceManager.GetClusterInfo()
	Expect(err).ToNot(HaveOccurred())

	output, err := framework.InstallGatewayAPI(k8sClient, *gatewayAPIVersion, *k8sVersion)
	Expect(err).ToNot(HaveOccurred(), string(output))

	output, err = framework.InstallNGF(cfg)
	Expect(err).ToNot(HaveOccurred(), string(output))

	podName, err := framework.GetNGFPodName(k8sClient, cfg.Namespace, cfg.ReleaseName, timeoutConfig.CreateTimeout)
	Expect(err).ToNot(HaveOccurred())

	if *serviceType != "LoadBalancer" {
		portFwdPort, err = framework.PortForward(k8sConfig, cfg.Namespace, podName, portForwardStopCh)
		address = "127.0.0.1"
	} else {
		address, err = resourceManager.GetLBIPAddress(cfg.Namespace)
	}
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	portForwardStopCh <- struct{}{}

	cfg := framework.InstallationConfig{
		ReleaseName: "ngf-test",
		Namespace:   "nginx-gateway",
	}

	output, err := framework.UninstallNGF(cfg, k8sClient)
	Expect(err).ToNot(HaveOccurred(), string(output))

	output, err = framework.UninstallGatewayAPI(*gatewayAPIVersion, *k8sVersion)
	Expect(err).ToNot(HaveOccurred(), string(output))
})
