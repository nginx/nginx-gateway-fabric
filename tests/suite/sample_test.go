package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("Basic test example", Label("functional"), func() {
	var (
		files = []string{
			"hello-world/apps.yaml",
			"hello-world/gateway.yaml",
			"hello-world/routes.yaml",
		}

		namespace = "helloworld"
	)

	BeforeEach(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		GinkgoWriter.Printf("Creating namespace %q,\n", namespace)
		applyErr := resourceManager.Apply([]client.Object{ns})
		if applyErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying namespace %q, error: %v\n",
				namespace,
				applyErr,
			)
		}
		Expect(applyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q applied successfully,\n", namespace)
		applyFilesErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resources from files %v to namespace %q, error: %v\n",
				files,
				namespace,
				applyFilesErr,
			)
		}
		Expect(applyFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)
		waitErr := resourceManager.WaitForAppsToBeReady(namespace)
		if waitErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for apps to be ready in namespace %s, error: %v\n",
				namespace,
				waitErr,
			)
		}
		Expect(waitErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Apps are ready in namespace %q,\n", namespace)

		GinkgoWriter.Printf("Waiting for NGINX pod to be ready in namespace %q,\n", namespace)
		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, namespace, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		GinkgoWriter.Printf("Found NGINX pod %q in namespace %q,\n", nginxPodNames, namespace)

		setUpPortForward(nginxPodNames[0], namespace)
		GinkgoWriter.Printf("Port-forward set for NGINX pod %q in namespace %q,\n", nginxPodNames[0], namespace)
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Adding NGINX logs and events to Report in namespace %q,\n", namespace)
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		GinkgoWriter.Printf("Cleaning up port-forward for NGINX pod in namespace %q,\n", namespace)
		cleanUpPortForward()

		GinkgoWriter.Printf("Deleting resources from files %v in namespace %q,\n", files, namespace)
		Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		GinkgoWriter.Printf("Resources from files %v deleted successfully from namespace %s,\n", namespace)
		GinkgoWriter.Printf("Deleting namespace %q,\n", namespace)
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
		GinkgoWriter.Printf("Namespace %q deleted successfully,\n", namespace)
	})

	It("sends traffic", func() {
		url := "http://foo.example.com/hello"
		if portFwdPort != 0 {
			url = fmt.Sprintf("http://foo.example.com:%s/hello", strconv.Itoa(portFwdPort))
		}

		Eventually(
			func() error {
				requestDetails := fmt.Sprintf(
					"Requesting url: %q, Address: %q, headers: %v, queryParams: %v",
					url,
					address,
					nil,
					nil,
				)
				GinkgoWriter.Printf("\nSending request: %s\n", requestDetails)
				status, body, err := framework.Get(url, address, timeoutConfig.RequestTimeout, nil, nil)
				GinkgoWriter.Printf("Response status: %d, body: %s\n", status, body)
				if err != nil {
					GinkgoWriter.Printf("ERROR occurred while sending request: %s\n", err)

					return err
				}
				if status != http.StatusOK {
					statusErr := fmt.Errorf("status not 200; got %d", status)
					GinkgoWriter.Printf("ERROR %s\n", statusErr)

					return statusErr
				}
				expBody := "URI: /hello"
				if !strings.Contains(body, expBody) {
					bodyErr := fmt.Errorf("bad body: got %s; expected %s", body, expBody)
					GinkgoWriter.Printf("ERROR: %s\n", bodyErr)

					return bodyErr
				}

				return nil
			}).
			WithTimeout(timeoutConfig.RequestTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())
	})
})
