package main

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("AdvancedRouting", Ordered, Label("functional", "routing"), func() {
	var (
		files = []string{
			"advanced-routing/cafe.yaml",
			"advanced-routing/gateway.yaml",
			"advanced-routing/grpc-backends.yaml",
			"advanced-routing/routes.yaml",
		}

		namespace = "routing"
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		GinkgoWriter.Printf("Creating namespace %q\n", ns)

		applyErr := resourceManager.Apply([]client.Object{ns})
		if applyErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource to namespace %q, error: %s\n",
				ns,
				applyErr,
			)
		}
		Expect(applyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q applied successfully,\n", ns)

		GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
		applyFromFilesErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyFromFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource from files %v, in namespace %q, error: %s\n",
				files,
				namespace,
				applyFromFilesErr,
			)
		}
		Expect(applyFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)

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
		GinkgoWriter.Printf("Apps are ready in namespace %q,\n", namespace)

		GinkgoWriter.Printf("Retrieving ready NGINX pods in namespace %q\n", namespace)
		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, namespace, timeoutConfig.GetStatusTimeout)
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for NginxPods to be ready with namespace %q, error: %v\n",
				namespace,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Nginx pods in namespace %q: %v\n", namespace, nginxPodNames)
		Expect(nginxPodNames).To(HaveLen(1))

		GinkgoWriter.Printf(
			"Setting up port-forward to nginx pod %s in namespace %q\n",
			nginxPodNames[0],
			namespace,
		)
		setUpPortForward(nginxPodNames[0], namespace)
	})

	AfterAll(func() {
		GinkgoWriter.Printf("Cleaning up resources in namespace %q\n", namespace)
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		GinkgoWriter.Printf("Cleaning up portForward")
		cleanUpPortForward()

		GinkgoWriter.Printf("Deleting resources from files: \n%v \nin namespace %q\n", files, namespace)
		deleteFromFilesErr := resourceManager.DeleteFromFiles(files, namespace)
		if deleteFromFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during deleting from files \n%v \nin namespace %q, error: %s\n",
				files,
				namespace,
				deleteFromFilesErr,
			)
		}
		Expect(deleteFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files: %v deleted successfully from namespace %q,\n", files, namespace)

		GinkgoWriter.Printf("Deleting namespace %q\n", namespace)
		deleteNSErr := resourceManager.DeleteNamespace(namespace)
		if deleteNSErr != nil {
			GinkgoWriter.Printf("ERROR occurred during deleting namespace %q, error: %s\n", namespace, deleteNSErr)
		}
		Expect(deleteNSErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q deleted successfully,\n", namespace)
	})

	When("valid advanced routing settings are configured for Routes", func() {
		var baseURL string
		BeforeAll(func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			GinkgoWriter.Printf("Setting up base URL for tests: http://cafe.example.com:%d\n", port)
			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
		})

		DescribeTable("verify working traffic for HTTPRoute",
			func(uri string, serverName string, headers map[string]string, queryParams map[string]string) {
				url := baseURL + uri
				Eventually(
					func() error {
						requestDetails := fmt.Sprintf(
							"URL: %s, Address: %s, serverName: %s, Headers: %v, QueryParams: %v\n",
							url,
							address,
							serverName,
							headers,
							queryParams,
						)
						GinkgoWriter.Printf("\nSending request %v\n", requestDetails)
						err := expectRequestToRespondFromExpectedServer(url, address, serverName, headers, queryParams)
						if err != nil {
							GinkgoWriter.Printf("ERROR occurred during getting response %verror: %s\n", requestDetails, err)
						}

						return err
					}).
					WithTimeout(timeoutConfig.GetTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			},
			Entry("request with no headers or params", "/coffee", "coffee-v1", nil, nil),
			Entry("request with Exact match header", "/coffee", "coffee-v2", map[string]string{"version": "v2"}, nil),
			Entry("request with Exact match query param", "/coffee", "coffee-v2", nil, map[string]string{"TEST": "v2"}),
			Entry(
				"request with RegularExpression match header",
				"/coffee",
				"coffee-v3",
				map[string]string{"headerRegex": "header-regex"},
				nil,
			),
			Entry(
				"request with RegularExpression match query param",
				"/coffee",
				"coffee-v3",
				nil,
				map[string]string{"queryRegex": "query-regex"},
			),
			Entry(
				"request with non-matching regex header",
				"/coffee",
				"coffee-v1",
				map[string]string{"headerRegex": "headerInvalid"},
				nil,
			),
			Entry(
				"request with non-matching regex query param",
				"/coffee",
				"coffee-v1",
				nil,
				map[string]string{"queryRegex": "queryInvalid"},
			),
		)
	})
})

func expectRequestToRespondFromExpectedServer(
	appURL, address, expServerName string,
	headers, queryParams map[string]string,
) error {
	requestDetails := fmt.Sprintf(
		"URL: %s, Address: %s, ServerName: %s, Headers: %v, QueryParams: %v\n",
		appURL,
		address,
		expServerName,
		headers,
		queryParams,
	)
	status, body, err := framework.Get(appURL, address, timeoutConfig.RequestTimeout, headers, queryParams)
	GinkgoWriter.Printf(
		"For the request %vReceived response: status: %d, body: %s\n",
		requestDetails,
		status,
		body,
	)
	if err != nil {
		GinkgoWriter.Printf("\nEROOR occurred during getting response: %v\n", err)

		return err
	}

	if status != http.StatusOK {
		statusErr := errors.New("http status was not 200")
		GinkgoWriter.Printf(
			"ERROR: Returned status is not OK for request %v it is: %v, returning error: %s\n",
			requestDetails,
			status,
			statusErr,
		)

		return statusErr
	}

	actualServerName, err := extractServerName(body)
	if err != nil {
		GinkgoWriter.Printf(
			"ERROR occurred during extracting server name from body: %v \nfor request: %verror: %v\n",
			body,
			requestDetails,
			err,
		)

		return err
	}

	if !strings.Contains(actualServerName, expServerName) {
		snErr := errors.New("expected response body to contain correct server name")
		GinkgoWriter.Printf(
			"Server name %s is not the same as expected %s, error: %s\n",
			actualServerName,
			expServerName,
			snErr,
		)

		return snErr
	}

	return nil
}

func extractServerName(responseBody string) (string, error) {
	re := regexp.MustCompile(`Server name:\s*(\S+)`)
	matches := re.FindStringSubmatch(responseBody)
	if len(matches) < 2 {
		return "", errors.New("server name not found")
	}

	return matches[1], nil
}
