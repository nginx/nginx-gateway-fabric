package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

// This test can be flaky when waiting to see traces show up in the collector logs.
// Sometimes they get there right away, sometimes it takes 30 seconds. Retries were
// added to attempt to mitigate the issue, but it didn't fix it 100%.
var _ = Describe("Tracing", FlakeAttempts(2), Label("functional", "tracing"), func() {
	var (
		files = []string{
			"hello-world/apps.yaml",
			"hello-world/gateway.yaml",
			"hello-world/routes.yaml",
		}
		nginxProxyFile     = "tracing/nginxproxy.yaml"
		policySingleFile   = "tracing/policy-single.yaml"
		policyMultipleFile = "tracing/policy-multiple.yaml"

		ns core.Namespace

		collectorPodName, helloURL, worldURL, helloworldURL string
	)

	BeforeEach(func() {
		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "helloworld",
			},
		}

		output, err := framework.InstallCollector()
		Expect(err).ToNot(HaveOccurred(), string(output))

		collectorPodName, err = framework.GetCollectorPodName(resourceManager)
		Expect(err).ToNot(HaveOccurred())

		Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())

		url := "http://foo.example.com"
		helloURL = url + "/hello"
		worldURL = url + "/world"
		helloworldURL = url + "/helloworld"
		if portFwdPort != 0 {
			helloURL = fmt.Sprintf("%s:%d/hello", url, portFwdPort)
			worldURL = fmt.Sprintf("%s:%d/world", url, portFwdPort)
			helloworldURL = fmt.Sprintf("%s:%d/helloworld", url, portFwdPort)
		}
	})

	AfterEach(func() {
		output, err := framework.UninstallCollector(resourceManager)
		Expect(err).ToNot(HaveOccurred(), string(output))

		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteFromFiles(
			[]string{nginxProxyFile, policySingleFile, policyMultipleFile}, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())

		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
		defer cancel()

		key := types.NamespacedName{Name: gatewayClassName}
		var gwClass gatewayv1.GatewayClass
		Expect(k8sClient.Get(ctx, key, &gwClass)).To(Succeed())

		gwClass.Spec.ParametersRef = nil

		Expect(k8sClient.Update(ctx, &gwClass)).To(Succeed())
	})

	updateGatewayClass := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
		defer cancel()

		key := types.NamespacedName{Name: gatewayClassName}
		var gwClass gatewayv1.GatewayClass
		if err := k8sClient.Get(ctx, key, &gwClass); err != nil {
			return err
		}

		gwClass.Spec.ParametersRef = &gatewayv1.ParametersReference{
			Group: ngfAPIv1alpha1.GroupName,
			Kind:  gatewayv1.Kind("NginxProxy"),
			Name:  "nginx-proxy",
		}

		return k8sClient.Update(ctx, &gwClass)
	}

	sendRequests := func(url string, count int) {
		for range count {
			Eventually(
				func() error {
					status, _, err := framework.Get(url, address, timeoutConfig.RequestTimeout)
					if err != nil {
						return err
					}
					if status != http.StatusOK {
						return fmt.Errorf("status not 200; got %d", status)
					}
					return nil
				}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		}
	}

	// Send traffic and verify that traces exist for hello app. We send every time this is called because
	// sometimes it takes awhile to see the traces show up.
	findTraces := func() bool {
		sendRequests(helloURL, 25)
		sendRequests(worldURL, 25)
		sendRequests(helloworldURL, 25)

		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		Expect(err).ToNot(HaveOccurred())
		return strings.Contains(logs, "service.name: Str(ngf:helloworld:gateway:my-test-svc)")
	}

	checkStatusAndTraces := func() {
		Eventually(
			verifyGatewayClassResolvedRefs).
			WithTimeout(timeoutConfig.GetTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Eventually(
			verifyPolicyStatus).
			WithTimeout(timeoutConfig.GetTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())

		// wait for expected first line to show up
		Eventually(findTraces, "5m", "5s").Should(BeTrue())
	}

	It("sends tracing spans for one policy attached to one route", func() {
		sendRequests(helloURL, 5)

		// verify that no traces exist yet
		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(logs).ToNot(ContainSubstring("service.name: Str(ngf:helloworld:gateway:my-test-svc)"))

		// install tracing configuration
		traceFiles := []string{
			nginxProxyFile,
			policySingleFile,
		}
		Expect(resourceManager.ApplyFromFiles(traceFiles, ns.Name)).To(Succeed())
		Expect(updateGatewayClass()).To(Succeed())

		checkStatusAndTraces()

		logs, err = resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(logs).To(ContainSubstring("http.method: Str(GET)"))
		Expect(logs).To(ContainSubstring("http.target: Str(/hello)"))
		Expect(logs).To(ContainSubstring("testkey1: Str(testval1)"))
		Expect(logs).To(ContainSubstring("testkey2: Str(testval2)"))

		// verify traces don't exist for other apps
		Expect(logs).ToNot(ContainSubstring("http.target: Str(/world)"))
		Expect(logs).ToNot(ContainSubstring("http.target: Str(/helloworld)"))
	})

	It("sends tracing spans for one policy attached to multiple routes", func() {
		// install tracing configuration
		traceFiles := []string{
			nginxProxyFile,
			policyMultipleFile,
		}
		Expect(resourceManager.ApplyFromFiles(traceFiles, ns.Name)).To(Succeed())
		Expect(updateGatewayClass()).To(Succeed())

		checkStatusAndTraces()

		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(logs).To(ContainSubstring("http.method: Str(GET)"))
		Expect(logs).To(ContainSubstring("http.target: Str(/hello)"))
		Expect(logs).To(ContainSubstring("http.target: Str(/world)"))
		Expect(logs).To(ContainSubstring("testkey1: Str(testval1)"))
		Expect(logs).To(ContainSubstring("testkey2: Str(testval2)"))

		// verify traces don't exist for helloworld apps
		Expect(logs).ToNot(ContainSubstring("http.target: Str(/helloworld)"))
	})
})

func verifyGatewayClassResolvedRefs() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var gc gatewayv1.GatewayClass
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, &gc); err != nil {
		return err
	}

	for _, cond := range gc.Status.Conditions {
		if cond.Type == string(conditions.GatewayClassResolvedRefs) && cond.Status == metav1.ConditionTrue {
			return nil
		}
	}

	return errors.New("ResolvedRefs status not set to true on GatewayClass")
}

func verifyPolicyStatus() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var pol ngfAPIv1alpha2.ObservabilityPolicy
	key := types.NamespacedName{Name: "test-observability-policy", Namespace: "helloworld"}
	if err := k8sClient.Get(ctx, key, &pol); err != nil {
		return err
	}

	var count int
	for _, ancestor := range pol.Status.Ancestors {
		for _, cond := range ancestor.Conditions {
			if cond.Type == string(gatewayv1alpha2.PolicyConditionAccepted) && cond.Status == metav1.ConditionTrue {
				count++
			}
		}
	}

	if count != len(pol.Status.Ancestors) {
		return fmt.Errorf("Policy not accepted; expected %d accepted conditions, got %d", len(pol.Status.Ancestors), count)
	}

	return nil
}
