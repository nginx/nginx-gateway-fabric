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

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// This test can be flaky when waiting to see traces show up in the collector logs.
// Sometimes they get there right away, sometimes it takes 30 seconds. Retries were
// added to attempt to mitigate the issue, but it didn't fix it 100%.
var _ = Describe("Tracing", FlakeAttempts(2), Ordered, Label("functional", "tracing"), func() {
	// To run the tracing test, you must build NGF with the following values:
	// TELEMETRY_ENDPOINT=otel-collector-opentelemetry-collector.collector.svc.cluster.local:4317
	// TELEMETRY_ENDPOINT_INSECURE = true

	var (
		files = []string{
			"hello-world/apps.yaml",
			"hello-world/gateway.yaml",
			"hello-world/routes.yaml",
		}
		policySingleFile   = "tracing/policy-single.yaml"
		policyMultipleFile = "tracing/policy-multiple.yaml"

		namespace = "helloworld"

		collectorPodName, helloURL, worldURL, helloworldURL string
	)

	updateNginxProxyTelemetrySpec := func(telemetry ngfAPIv1alpha2.Telemetry) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.UpdateTimeout)
		defer cancel()

		key := types.NamespacedName{Name: "ngf-test-proxy-config", Namespace: "nginx-gateway"}
		var nginxProxy ngfAPIv1alpha2.NginxProxy

		getK8sObjErr := k8sClient.Get(ctx, key, &nginxProxy)
		if getK8sObjErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during getting k8s object with key %v, error: %s\n",
				key,
				getK8sObjErr,
			)
		}
		Expect(getK8sObjErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("NGINXProxy %s/%s fetched successfully\n", nginxProxy.Namespace, nginxProxy.Name)

		nginxProxy.Spec.Telemetry = &telemetry

		updateProxyErr := k8sClient.Update(ctx, &nginxProxy)
		if updateProxyErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during updating k8s object with key %v, error: %s\n",
				key,
				updateProxyErr,
			)
		}
		Expect(updateProxyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("NGINXProxy %s/%s updated successfully\n", nginxProxy.Namespace, nginxProxy.Name)
	}

	BeforeAll(func() {
		telemetry := ngfAPIv1alpha2.Telemetry{
			Exporter: &ngfAPIv1alpha2.TelemetryExporter{
				Endpoint: helpers.GetPointer("otel-collector-opentelemetry-collector.collector.svc:4317"),
			},
			ServiceName: helpers.GetPointer("my-test-svc"),
			SpanAttributes: []ngfAPIv1alpha1.SpanAttribute{{
				Key:   "testkey1",
				Value: "testval1",
			}},
		}

		GinkgoWriter.Printf("Setting telemetry spec for NginxProxy to: %v\n", telemetry)
		updateNginxProxyTelemetrySpec(telemetry)
	})

	// BeforeEach is needed because FlakeAttempts do not re-run BeforeAll/AfterAll nodes
	BeforeEach(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		GinkgoWriter.Printf("Installing tracing collector\n")
		output, err := framework.InstallCollector()
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during installing collector, error: %v\n", err)
		}
		Expect(err).ToNot(HaveOccurred(), string(output))
		GinkgoWriter.Printf("Collector installed, output: %s\n", string(output))

		GinkgoWriter.Printf("Retrieving collector pod name\n")
		collectorPodName, err = framework.GetCollectorPodName(resourceManager)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during getting collector pod name: %v\n", err)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Collector pod name: %q\n", collectorPodName)

		applyErr := resourceManager.Apply([]client.Object{ns})
		if applyErr != nil {
			GinkgoWriter.Printf("ERROR occurred during applying resource to namespace %q, error: %v\n", ns, applyErr)
		}
		Expect(applyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q applied successfully\n", namespace)

		GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
		applyFromFilesErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyFromFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource from files %v to namespace %q, error: %s\n",
				files,
				namespace,
				applyFromFilesErr,
			)
		}
		Expect(applyFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q\n", namespace)

		GinkgoWriter.Printf("Waiting for apps to be ready in namespace %q\n", namespace)
		waitingErr := resourceManager.WaitForAppsToBeReady(namespace)
		if waitingErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for apps to be ready in namespace %q, error: %s\n",
				namespace,
				waitingErr,
			)
		}
		Expect(waitingErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Apps are ready in namespace %q\n", namespace)

		GinkgoWriter.Printf("Retrieving ready NGINX pods in namespace %q\n", namespace)
		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, namespace, timeoutConfig.GetStatusTimeout)
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for NginxPods to be ready in namespace %q, error: %s\n",
				namespace,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Found NGINX pod %v in namespace %q\n", nginxPodNames, namespace)
		Expect(nginxPodNames).To(HaveLen(1))

		setUpPortForward(nginxPodNames[0], namespace)
		GinkgoWriter.Printf("Port-forward set for NGINX pod %s in namespace %q\n", nginxPodNames[0], namespace)

		url := "http://foo.example.com"
		helloURL = url + "/hello"
		worldURL = url + "/world"
		helloworldURL = url + "/helloworld"
		if portFwdPort != 0 {
			helloURL = fmt.Sprintf("%s:%d/hello", url, portFwdPort)
			worldURL = fmt.Sprintf("%s:%d/world", url, portFwdPort)
			helloworldURL = fmt.Sprintf("%s:%d/helloworld", url, portFwdPort)
		}
		setUpEnvironment := fmt.Sprintf(
			"helloURL: %q, worldURL: %q, helloworldURL: %q",
			helloURL,
			worldURL,
			helloworldURL,
		)
		GinkgoWriter.Printf("Set up environment: %s\n", setUpEnvironment)
	})

	AfterEach(func() {
		GinkgoWriter.Printf("\nAdding NGINX logs and events to report for namespace %q\n", namespace)
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		GinkgoWriter.Printf("Uninstalling tracing collector in namespace %q\n", namespace)
		output, err := framework.UninstallCollector(resourceManager)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during uninstalling collector: %s\n", err)
		}
		Expect(err).ToNot(HaveOccurred(), string(output))
		GinkgoWriter.Printf("Collector uninstalled, output: %q\n", string(output))

		GinkgoWriter.Printf("Cleaning up port-forward\n")
		cleanUpPortForward()

		GinkgoWriter.Printf("Deleting resources from files: %v in namespace %q\n", files, namespace)
		deleteFromFilesErr := resourceManager.DeleteFromFiles(files, namespace)
		if deleteFromFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during deleting from files %v in namespace %q, error: %s\n",
				files,
				namespace,
				deleteFromFilesErr,
			)
		}
		Expect(deleteFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files: %v deleted successfully from namespace %q\n", files, namespace)

		GinkgoWriter.Printf(
			"Deleting policy resources: [%q, %q] in namespace %q\n",
			policySingleFile,
			policyMultipleFile,
			namespace,
		)
		deleteFromFileWithPoliciesErr := resourceManager.DeleteFromFiles(
			[]string{policySingleFile, policyMultipleFile},
			namespace,
		)
		if deleteFromFileWithPoliciesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during deleting policy files in namespace %q: %q and %q, error: %s\n",
				namespace,
				policySingleFile,
				policyMultipleFile,
				deleteFromFileWithPoliciesErr,
			)
		}
		Expect(deleteFromFileWithPoliciesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Policy resources deleted successfully from namespace %q\n", namespace)

		GinkgoWriter.Printf("Deleting namespace %q\n", namespace)
		deleteNSErr := resourceManager.DeleteNamespace(namespace)
		if deleteNSErr != nil {
			GinkgoWriter.Printf("ERROR occurred during deleting namespace %q, error: %s\n", namespace, deleteNSErr)
		}
		Expect(deleteNSErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q deleted successfully\n", namespace)
	})

	AfterAll(func() {
		GinkgoWriter.Printf("Resetting telemetry spec for NginxProxy\n")
		updateNginxProxyTelemetrySpec(ngfAPIv1alpha2.Telemetry{})
	})

	sendRequests := func(url string, count int) {
		for range count {
			Eventually(
				func() error {
					status, _, err := framework.Get(url, address, timeoutConfig.RequestTimeout, nil, nil)
					requestDetails := fmt.Sprintf("URL: %s, Address: %s, Headers: %v, QueryParams: %v", url, address, nil, nil)

					if err != nil {
						GinkgoWriter.Printf("ERROR occurred during sending GET request %v, \nerror: %s\n", requestDetails, err)

						return err
					}
					if status != http.StatusOK {
						statusError := fmt.Errorf("status not 200; got %d", status)
						GinkgoWriter.Printf(
							"ERROR: Returned status is not OK for request %v\nresponse status: %v, error: %s\n",
							requestDetails,
							status,
							statusError,
						)

						return statusError
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
		GinkgoWriter.Printf("\nSending %d requests to generate traces for URL %q\n", 25, helloURL)
		sendRequests(helloURL, 25)
		GinkgoWriter.Printf("Sending %d requests to generate traces for URL %q\n", 25, worldURL)
		sendRequests(worldURL, 25)
		GinkgoWriter.Printf("Sending %d requests to generate traces for URL %q\n", 25, helloworldURL)
		sendRequests(helloworldURL, 25)

		GinkgoWriter.Printf("Getting pod %q logs\n", collectorPodName)
		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during getting pod logs for collectorPodName: %q, error: %s\n",
				collectorPodName,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Logs from pod %q received\n", collectorPodName)
		expectedTraceLogLine := "service.name: Str(ngf:helloworld:gateway:my-test-svc)"
		GinkgoWriter.Printf("Looking for expected trace log line: %q\n", expectedTraceLogLine)
		isContainingLogLine := strings.Contains(logs, expectedTraceLogLine)
		GinkgoWriter.Printf("Found expected trace log line: %v\n", isContainingLogLine)

		return isContainingLogLine
	}

	checkStatusAndTraces := func() {
		GinkgoWriter.Printf("Check status and traces\n")
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
		GinkgoWriter.Printf("Waiting for traces to show up in collector pod logs\n")
		Eventually(findTraces, "5m", "5s").Should(BeTrue())
	}

	It("sends tracing spans for one policy attached to one route", func() {
		GinkgoWriter.Printf("\nSending %d tracing spans for one policy attached to one route %q\n", 5, helloURL)
		sendRequests(helloURL, 5)

		// verify that no traces exist yet
		GinkgoWriter.Printf("Verifying that no traces exist in collector pod logs\n")
		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during getting pod logs for collectorPodName: %q, error: %s\n",
				collectorPodName,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Logs from pod %q received\n", collectorPodName)
		notExpectedLogsLine := "service.name: Str(ngf:helloworld:gateway:my-test-svc)"
		Expect(logs).ToNot(ContainSubstring(notExpectedLogsLine))
		GinkgoWriter.Printf("No traces of %q found in collector pod logs\n", notExpectedLogsLine)

		// install tracing configuration
		traceFiles := []string{
			policySingleFile,
		}
		GinkgoWriter.Printf("Applying tracing configuration from files: %v to namespace %q\n", traceFiles, namespace)
		Expect(resourceManager.ApplyFromFiles(traceFiles, namespace)).To(Succeed())

		checkStatusAndTraces()

		logs, err = resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during getting pod logs for collectorPodName: %q, error: %s\n",
				collectorPodName,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Logs from pod %q received\n", collectorPodName)

		GinkgoWriter.Printf("\nVerifying traces %s in collector pod logs\n", "http.method: Str(GET)")
		Expect(logs).To(ContainSubstring("http.method: Str(GET)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "http.target: Str(/hello)")
		Expect(logs).To(ContainSubstring("http.target: Str(/hello)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "testkey1: Str(testval1)")
		Expect(logs).To(ContainSubstring("testkey1: Str(testval1)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "testkey2: Str(testval2)")
		Expect(logs).To(ContainSubstring("testkey2: Str(testval2)"))
		GinkgoWriter.Printf("All traces are found\n")

		// verify traces don't exist for other apps
		GinkgoWriter.Printf(
			"\nVerifying that traces %s for other apps do not exist in collector pod logs\n",
			"http.target: Str(/world)",
		)
		Expect(logs).ToNot(ContainSubstring("http.target: Str(/world)"))
		GinkgoWriter.Printf(
			"Verifying that traces %s for other apps do not exist in collector pod logs\n",
			"http.target: Str(/helloworld)",
		)
		Expect(logs).ToNot(ContainSubstring("http.target: Str(/helloworld)"))
		GinkgoWriter.Printf("No traces of other apps found in collector pod logs\n")
	})

	It("sends tracing spans for one policy attached to multiple routes", func() {
		// install tracing configuration
		traceFiles := []string{
			policyMultipleFile,
		}
		GinkgoWriter.Printf(
			"Applying tracing configuration from files: %v to namespace %q\n",
			traceFiles,
			namespace,
		)
		applyFromFilesErr := resourceManager.ApplyFromFiles(traceFiles, namespace)
		if applyFromFilesErr != nil {
			GinkgoWriter.Printf("ERROR occurred during applying resource from file: %v\n", applyFromFilesErr)
		}
		Expect(applyFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf(
			"Tracing configuration applied successfully from files: %v to namespace %q\n",
			traceFiles,
			namespace,
		)

		checkStatusAndTraces()

		GinkgoWriter.Printf("Verifying traces in collector pod logs\n")
		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, collectorPodName, &core.PodLogOptions{})
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during getting pod logs for collectorPodName: %s, error: %s\n",
				collectorPodName,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Logs from pod %q received\n", collectorPodName)

		GinkgoWriter.Printf("\nVerifying traces %s in collector pod logs\n", "http.method: Str(GET)")
		Expect(logs).To(ContainSubstring("http.method: Str(GET)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "http.target: Str(/hello)")
		Expect(logs).To(ContainSubstring("http.target: Str(/hello)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "http.target: Str(/world)")
		Expect(logs).To(ContainSubstring("http.target: Str(/world)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "testkey1: Str(testval1)")
		Expect(logs).To(ContainSubstring("testkey1: Str(testval1)"))
		GinkgoWriter.Printf("Verifying traces %s in collector pod logs\n", "testkey2: Str(testval2)")
		Expect(logs).To(ContainSubstring("testkey2: Str(testval2)"))
		GinkgoWriter.Printf("All traces are found\n")

		// verify traces don't exist for helloworld apps
		GinkgoWriter.Printf(
			"\nVerifying that traces for %q app do not exist in collector pod logs\n",
			"http.target: Str(/helloworld)",
		)
		Expect(logs).ToNot(ContainSubstring("http.target: Str(/helloworld)"))
		GinkgoWriter.Printf(
			"No traces of %q app found in collector pod logs\n",
			"http.target: Str(/helloworld)",
		)
	})
})

func verifyGatewayClassResolvedRefs() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var gc gatewayv1.GatewayClass
	GinkgoWriter.Printf("Verifying GatewayClass %q is resolved\n", gatewayClassName)
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, &gc); err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting k8s object: %v\n", err)

		return err
	}

	GinkgoWriter.Printf("Comparing GatewayClass conditions\n")
	for _, cond := range gc.Status.Conditions {
		if cond.Type == string(conditions.GatewayClassResolvedRefs) && cond.Status == metav1.ConditionTrue {
			return nil
		}
	}

	resolvedRefErr := errors.New("ResolvedRefs status not set to true on GatewayClass")
	GinkgoWriter.Printf("ERROR occurred during resolving References: %v\n", resolvedRefErr)
	return resolvedRefErr
}

func verifyPolicyStatus() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var pol ngfAPIv1alpha2.ObservabilityPolicy
	key := types.NamespacedName{Name: "test-observability-policy", Namespace: "helloworld"}
	GinkgoWriter.Printf("Verifying ObservabilityPolicy %q is accepted\n", key)
	if err := k8sClient.Get(ctx, key, &pol); err != nil {
		GinkgoWriter.Printf(
			"ERROR occurred during getting k8s object with ObservabilityPolicy key: %v, error: %s\n",
			key,
			err,
		)

		return err
	}

	var count int
	GinkgoWriter.Printf("Comparing ObservabilityPolicy conditions\n")
	for _, ancestor := range pol.Status.Ancestors {
		for _, cond := range ancestor.Conditions {
			if cond.Type == string(gatewayv1alpha2.PolicyConditionAccepted) && cond.Status == metav1.ConditionTrue {
				count++
			}
		}
	}

	GinkgoWriter.Printf("Found %d accepted conditions in ObservabilityPolicy %q\n", count, key)
	if count != len(pol.Status.Ancestors) {
		err := fmt.Errorf(
			"Policy not accepted; expected %d accepted conditions, got %d",
			len(pol.Status.Ancestors),
			count,
		)
		GinkgoWriter.Printf("Error in policies: %v\n", err)

		return err
	}
	GinkgoWriter.Printf("Policy %q is accepted\n", key)

	return nil
}
