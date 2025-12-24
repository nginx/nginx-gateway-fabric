package main

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("SnippetsPolicy", Ordered, Label("functional", "snippets-policy"), func() {
	var (
		files = []string{
			"snippets-policy/cafe.yaml",
			"snippets-policy/gateway.yaml",
		}

		namespace = "snippets-policy"

		nginxPodName string
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

		nginxPodName = nginxPodNames[0]

		setUpPortForward(nginxPodName, namespace)
	})

	AfterAll(func() {
		cleanUpPortForward()

		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("SnippetsPolicies are applied to the resources", func() {
		snippetsPolicy := []string{
			"snippets-policy/valid-sp.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(snippetsPolicy, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			Expect(resourceManager.DeleteFromFiles(snippetsPolicy, namespace)).To(Succeed())
		})
		Specify("snippetsPolicies are accepted", func() {
			snippetsPolicyNames := []string{
				"valid-sp",
			}
			for _, name := range snippetsPolicyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				Eventually(checkForSnippetsPolicyToBeAccepted).
					WithArguments(nsname).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500*time.Millisecond).
					Should(Succeed(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Specify("empty snippets policy is accepted", func() {
			files := []string{"snippets-policy/empty-snippets-sp.yaml"}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			nsname := types.NamespacedName{Name: "empty-snippets-sp", Namespace: namespace}
			Eventually(checkForSnippetsPolicyToBeAccepted).
				WithArguments(nsname).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoute", func() {
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")

				Eventually(
					func() error {
						return expectRequestToSucceed(baseURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("SnippetsPolicy", []framework.ExpectedNginxField{
					{
						Directive: "worker_priority",
						Value:     "0",
						File:      "SnippetsPolicy_main_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "aio",
						Value:     "off",
						File:      "SnippetsPolicy_http_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "auth_delay",
						Value:     "0s",
						File:      "SnippetsPolicy_server_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "allow",
						Value:     "127.0.0.1",
						File:      "SnippetsPolicy_location_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_main_snippets-policy-valid-sp.conf",
						File:      "main.conf",
					},
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_http_snippets-policy-valid-sp.conf",
						File:      "http.conf",
					},
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_server_snippets-policy-valid-sp.conf",
						File:      "http.conf",
						Server:    "cafe.example.com",
					},
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_location_snippets-policy-valid-sp.conf",
						File:      "http.conf",
						Location:  "/coffee",
					},
				}),
			)
		})
	})
})

func checkForSnippetsPolicyToBeAccepted(snippetsPolicyNsNames types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Checking for SnippetsPolicy %q to have the condition Accepted/True/Accepted\n",
		snippetsPolicyNsNames,
	)

	var sp ngfAPI.SnippetsPolicy
	var err error

	if err = resourceManager.Get(ctx, snippetsPolicyNsNames, &sp); err != nil {
		return err
	}

	if len(sp.Status.Ancestors) == 0 {
		return fmt.Errorf("snippetsPolicy has no ancestors")
	}

	if len(sp.Status.Ancestors[0].Conditions) == 0 {
		return fmt.Errorf("snippetsPolicy ancestor has no conditions")
	}

	condition := sp.Status.Ancestors[0].Conditions[0]
	if condition.Type != string(v1.PolicyConditionAccepted) {
		wrongTypeErr := fmt.Errorf("expected condition type to be Accepted, got %s", condition.Type)
		GinkgoWriter.Printf("ERROR: %v\n", wrongTypeErr)

		return wrongTypeErr
	}

	if condition.Status != metav1.ConditionTrue {
		wrongStatusErr := fmt.Errorf("expected condition status to be %s, got %s", metav1.ConditionTrue, condition.Status)
		GinkgoWriter.Printf("ERROR: %v\n", wrongStatusErr)

		return wrongStatusErr
	}

	if condition.Reason != string(v1.PolicyReasonAccepted) {
		wrongReasonErr := fmt.Errorf(
			"expected condition reason to be %s, got %s",
			v1.PolicyReasonAccepted,
			condition.Reason,
		)
		GinkgoWriter.Printf("ERROR: %v\n", wrongReasonErr)

		return wrongReasonErr
	}

	return nil
}
