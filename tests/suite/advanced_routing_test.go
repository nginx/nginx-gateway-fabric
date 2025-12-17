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

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
			namespace,
			timeoutConfig.GetStatusTimeout,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))

		setUpPortForward(nginxPodNames[0], namespace)
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		cleanUpPortForward()

		Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("valid advanced routing settings are configured for Routes", func() {
		var baseURL string
		BeforeAll(func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
		})

		DescribeTable("verify working traffic for HTTPRoute",
			func(uri string, serverName string, headers map[string]string, queryParams map[string]string) {
				url := baseURL + uri
				Eventually(
					func() error {
						return expectRequestToRespondFromExpectedServer(url, address, serverName, headers, queryParams)
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
	GinkgoWriter.Printf("Expecting request to respond from the server %q\n", expServerName)

	request := framework.Request{
		URL:         appURL,
		Address:     address,
		Timeout:     timeoutConfig.RequestTimeout,
		Headers:     headers,
		QueryParams: queryParams,
	}

	resp, err := framework.Get(request)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		statusErr := errors.New("http status was not 200")
		GinkgoWriter.Printf("ERROR: %v\n", statusErr)

		return statusErr
	}

	actualServerName, err := extractServerName(resp.Body)
	if err != nil {
		GinkgoWriter.Printf("ERROR extracting server name from response body: %v\n", err)

		return err
	}

	if !strings.Contains(actualServerName, expServerName) {
		return errors.New("expected response body to contain correct server name")
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
