package main

import (
	"context"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// The GatewayLink/ExternalLoadBalancer feature is exercised end-to-end against a real BIG-IP fronted
// by F5 CIS. CIS watches the Gateway and the ExternalLoadBalancer and programs a virtual server;
// traffic flows client -> BIG-IP VIP -> NGINX -> app. Each test applies an ExternalLoadBalancer,
// waits for Accepted, and asserts a 200 through the VIP, which proves CIS programmed a working
// virtual server. Two suites run serially, one per CIS pool mode: nodeport first, then cluster.

const gatewaylinkNamespace = "gatewaylink"

// gatewaylinkManifests are the resources every pool mode needs. The NginxProxy is applied separately
// because its Service type differs per mode.
var gatewaylinkManifests = []string{
	"externalloadbalancer/gatewaylink/apps.yaml",
	"externalloadbalancer/gatewaylink/routes.yaml",
	"externalloadbalancer/gatewaylink/gateway.yaml",
}

// externalLoadBalancerAccepted reports whether any controller wrote an Accepted=True condition on
// the ExternalLoadBalancer.
func externalLoadBalancerAccepted(elb *ngfAPIv1alpha1.ExternalLoadBalancer) bool {
	for _, ctrl := range elb.Status.Controllers {
		for _, cond := range ctrl.Conditions {
			if cond.Type == string(ngfAPIv1alpha1.ExternalLoadBalancerConditionTypeAccepted) &&
				cond.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// applyELBAndWaitAccepted applies the ExternalLoadBalancer in the given manifest and waits for a
// controller to mark it Accepted. The manifests carry ${BIGIP_*} tokens substituted via
// resourceManager.TextReplacements.
func applyELBAndWaitAccepted(file string) {
	Expect(resourceManager.ApplyFromFiles([]string{file}, gatewaylinkNamespace)).To(Succeed())
	Eventually(
		func(g Gomega) {
			got := &ngfAPIv1alpha1.ExternalLoadBalancer{}
			g.Expect(k8sClient.Get(
				context.Background(),
				client.ObjectKey{Namespace: gatewaylinkNamespace, Name: "gateway-elb"},
				got,
			)).To(Succeed())
			g.Expect(externalLoadBalancerAccepted(got)).To(BeTrue(), "ExternalLoadBalancer not Accepted")
		}).
		WithTimeout(2 * timeoutConfig.CreateTimeout).
		WithPolling(2 * time.Second).
		Should(Succeed())
}

// ingressLinkVSAddress returns the virtual server address CIS allocated through IPAM. NGF creates an
// IngressLink for the Gateway, and CIS writes the allocated address to its status.vsAddress field,
// so the address is not known until CIS has processed the resource. It retries until the field is
// populated.
func ingressLinkVSAddress() string {
	var address string
	Eventually(
		func(g Gomega) {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{Group: "cis.f5.com", Version: "v1", Kind: "IngressLinkList"})
			g.Expect(k8sClient.List(context.Background(), list, client.InNamespace(gatewaylinkNamespace))).To(Succeed())
			g.Expect(list.Items).ToNot(BeEmpty(), "expected NGF to create an IngressLink")

			addr, found, err := unstructured.NestedString(list.Items[0].Object, "status", "vsAddress")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue(), "IngressLink status.vsAddress not set yet")
			g.Expect(addr).ToNot(BeEmpty(), "IngressLink status.vsAddress is empty")
			// Validate the address parses so a malformed value fails here rather than as a long
			// traffic timeout later.
			g.Expect(net.ParseIP(addr)).ToNot(BeNil(), "IngressLink status.vsAddress %q is not a valid IP", addr)
			address = addr
		}).
		WithTimeout(2 * timeoutConfig.CreateTimeout).
		WithPolling(2 * time.Second).
		Should(Succeed())
	return address
}

// expectTrafficThroughVIP asserts traffic reaches the app through the BIG-IP virtual server at the
// given address. A 200 proves the whole chain is live: CIS observed the IngressLink, pushed the AS3
// declaration, and the BIG-IP monitor marked the pool member up.
func expectTrafficThroughVIP(address string) {
	Eventually(
		func(g Gomega) {
			resp, err := framework.Get(framework.Request{
				URL:     "http://cafe.example.com/coffee",
				Address: address,
				Timeout: timeoutConfig.RequestTimeout,
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
			g.Expect(resp.Body).To(ContainSubstring("URI: /coffee"))
		}).
		// The AS3 push plus the BIG-IP monitor marking the member up runs after the ELB is Accepted and
		// can take a few probe intervals, so allow a generous window.
		WithTimeout(10 * time.Minute).
		WithPolling(2 * time.Second).
		Should(Succeed())
}

// installGatewaylinkCIS installs CIS in the given pool mode and deploys the shared app, route, and
// Gateway plus the mode-specific NginxProxy. It is the common BeforeAll body for both pool modes.
func installGatewaylinkCIS(poolMode, nginxProxyFile string) {
	// Start from a clean partition. A previous run that was killed before its teardown can leave a
	// virtual server behind at the same address, which would let the static test pass on stale state
	// even if the current ExternalLoadBalancer was never programmed. Best-effort: a failure here is
	// reported but does not fail the run.
	if err := framework.DeleteAS3Tenant(
		*bigipVIP, *bigipMgmtPort, *bigipUsername, *bigipPassword, *bigipPartition,
	); err != nil {
		AddReportEntry("setup: failed to pre-clean BIG-IP AS3 tenant", err.Error())
	}

	// CIS runs in-cluster and reaches the BIG-IP over the internal subnet, so it is pointed at the
	// internal VIP address rather than the external NAT management address.
	cisCfg := framework.CISConfig{
		BIGIPAddress:   *bigipVIP,
		BIGIPMgmtPort:  *bigipMgmtPort,
		BIGIPPartition: *bigipPartition,
		BIGIPUsername:  *bigipUsername,
		BIGIPPassword:  *bigipPassword,
		PoolMemberType: poolMode,
		EnableIPAM:     true,
	}
	output, err := framework.InstallCIS(cisCfg)
	Expect(err).ToNot(HaveOccurred(), string(output))

	// CIS enables IPAM but does not run the allocator, so deploy FIC into the CIS namespace to fulfill
	// the ipamLabel requests. The CIS install created that namespace.
	ficOutput, err := framework.InstallFIC(resourceManager, *ipamRange)
	Expect(err).ToNot(HaveOccurred(), string(ficOutput))

	ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewaylinkNamespace}}
	Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

	// Substitute environment-specific BIG-IP values into the ExternalLoadBalancer manifests. Merge
	// rather than replace so the suite-level gatewayClassName replacement is preserved.
	if resourceManager.TextReplacements == nil {
		resourceManager.TextReplacements = map[string]string{}
	}
	resourceManager.TextReplacements["${BIGIP_VIP}"] = *bigipVIP
	resourceManager.TextReplacements["${BIGIP_PARTITION}"] = *bigipPartition

	files := append([]string{nginxProxyFile}, gatewaylinkManifests...)
	Expect(resourceManager.ApplyFromFiles(files, gatewaylinkNamespace)).To(Succeed())
	Expect(resourceManager.WaitForAppsToBeReady(gatewaylinkNamespace)).To(Succeed())
}

// teardownGatewaylinkCIS tears down the resources installGatewaylinkCIS created and deletes the AS3
// tenant so the next pool mode starts against a clean partition. It is the common AfterAll body for
// both pool modes.
func teardownGatewaylinkCIS() {
	framework.AddNginxLogsAndEventsToReport(resourceManager, gatewaylinkNamespace)

	Expect(resourceManager.DeleteNamespace(gatewaylinkNamespace)).To(Succeed())
	framework.UninstallFIC(resourceManager)
	output, err := framework.UninstallCIS()
	Expect(err).ToNot(HaveOccurred(), string(output))
	Expect(resourceManager.DeleteNamespace(framework.CISNamespace)).To(Succeed())

	// CIS does not remove the virtual server it programmed for an IPAM-allocated address when the
	// IngressLink is deleted, so delete the AS3 tenant here to keep repeated runs against the same
	// BIG-IP from accumulating orphaned objects. The internal VIP is used rather than the external
	// management address because the test runs inside the VPC, where the internal address answers
	// quickly and the external NAT address is slow and times out. This is best-effort: a failure is
	// reported but does not fail the run.
	if err := framework.DeleteAS3Tenant(
		*bigipVIP, *bigipMgmtPort, *bigipUsername, *bigipPassword, *bigipPartition,
	); err != nil {
		AddReportEntry("cleanup: failed to delete BIG-IP AS3 tenant", err.Error())
	}
}

// Both pool modes run in one invocation, nodeport first then cluster. The outer container is Ordered
// so Ginkgo runs the two inner suites in spec order, and each reinstalls CIS with its own pool mode
// and tears it down before the next starts.
var _ = Describe("GatewayLink external LB", Ordered, Label("gatewaylink"), func() {
	BeforeAll(func() {
		if !*gatewaylinkEnabled {
			Skip("Skipping GatewayLink tests: --gatewaylink-enabled is not set")
		}
		if *bigipVIP == "" || *bigipPassword == "" {
			Skip("Skipping GatewayLink tests: --bigip-vip and --bigip-password must be set")
		}
		if *ipamRange == "" {
			Skip("Skipping GatewayLink tests: --ipam-range must be set (required for IPAM allocation)")
		}
	})

	// CIS runs in nodeport pool mode: the BIG-IP targets node IP + node port. The NGF data-plane
	// Service is NodePort with externalTrafficPolicy: Local, which preserves the client IP and makes
	// only the pod's node advertise the pool member.
	Describe("nodeport pool mode", Ordered, Label("nodeport"), func() {
		BeforeAll(func() {
			installGatewaylinkCIS("nodeport", "externalloadbalancer/gatewaylink/nginx-proxy.yaml")
		})

		AfterAll(func() {
			teardownGatewaylinkCIS()
		})

		It("programs a statically-addressed virtual server and routes traffic through it", func() {
			applyELBAndWaitAccepted("externalloadbalancer/gatewaylink/static.yaml")
			expectTrafficThroughVIP(*bigipVIP)
		})

		// The IPAM test runs after the static test on purpose. CIS does not fully clean up the resources
		// it allocates through IPAM, so running it last keeps that residue from affecting the static
		// test.
		It("allocates a virtual server address from CIS IPAM and routes traffic through it", func() {
			applyELBAndWaitAccepted("externalloadbalancer/gatewaylink/ipam.yaml")
			address := ingressLinkVSAddress()
			expectTrafficThroughVIP(address)
		})
	})

	// CIS runs in cluster pool mode: the BIG-IP targets the NGINX pod IPs directly. On GKE pod IPs are
	// native VPC addresses, so create-bigip-vm.sh adds a route on the BIG-IP that sends the pod CIDR to
	// the subnet gateway; the reply routes back natively because the BIG-IP address is on the subnet.
	// That routing is in place before this suite runs, so the tests here only assert traffic flows.
	Describe("cluster pool mode", Ordered, Label("cluster"), func() {
		BeforeAll(func() {
			installGatewaylinkCIS("cluster", "externalloadbalancer/gatewaylink/nginx-proxy-cluster.yaml")
		})

		AfterAll(func() {
			teardownGatewaylinkCIS()
		})

		It("programs a statically-addressed virtual server and routes traffic through it", func() {
			applyELBAndWaitAccepted("externalloadbalancer/gatewaylink/static.yaml")
			expectTrafficThroughVIP(*bigipVIP)
		})

		It("allocates a virtual server address from CIS IPAM and routes traffic through it", func() {
			applyELBAndWaitAccepted("externalloadbalancer/gatewaylink/ipam.yaml")
			address := ingressLinkVSAddress()
			expectTrafficThroughVIP(address)
		})
	})
})
