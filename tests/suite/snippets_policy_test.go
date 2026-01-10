package main

import (
	"context"
	"fmt"
	"strings"
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

			It("should return a 200 response for HTTPRoute with header match (internal location)", func() {
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/tea")

				request := framework.Request{
					URL:     baseURL,
					Address: address,
					Headers: map[string]string{"version": "v1"},
					Timeout: timeoutConfig.RequestTimeout,
				}

				Eventually(
					func() error {
						resp, err := framework.Get(request)
						if err != nil {
							return err
						}
						if resp.StatusCode != 200 {
							return fmt.Errorf("expected 200, got %d", resp.StatusCode)
						}
						return nil
					},
				).
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
						Directive: "timer_resolution",
						Value:     "100ms",
						File:      "SnippetsPolicy_main_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "gzip",
						Value:     "off",
						File:      "SnippetsPolicy_http_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "server_name_in_redirect",
						Value:     "off",
						File:      "SnippetsPolicy_server_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "add_header",
						Value:     "X-Snippets-Policy valid",
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
						Server:    "cafe.example.com",
					},
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_location_snippets-policy-valid-sp.conf",
						File:      "http.conf",
						Location:  "/_ngf-internal-rule1-route0",
						Server:    "cafe.example.com",
					},
				}),
			)
		})
	})

	When("Multiple SnippetsPolicies are applied", func() {
		policies := []string{
			"snippets-policy/valid-sp.yaml",
			"snippets-policy/second-sp.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		It("should both be accepted and applied in order", func() {
			for _, name := range []string{"valid-sp", "second-sp"} {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}
				Eventually(checkForSnippetsPolicyToBeAccepted).
					WithArguments(nsname).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			}

			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					return err
				}

				if err := framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "add_header",
					Value:     "X-Second-Policy true",
					File:      "SnippetsPolicy_location_snippets-policy-second-sp.conf",
				}); err != nil {
					return err
				}

				return framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "add_header",
					Value:     "X-Snippets-Policy valid",
					File:      "SnippetsPolicy_location_snippets-policy-valid-sp.conf",
				})
			}).WithTimeout(timeoutConfig.GetStatusTimeout).WithPolling(500 * time.Millisecond).Should(Succeed())
		})
	})

	When("SnippetsPolicy is deleted", func() {
		policy := []string{"snippets-policy/valid-sp.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policy, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		It("should remove configuration and maintain traffic", func() {
			Expect(resourceManager.DeleteFromFiles(policy, namespace)).To(Succeed())

			Eventually(func() error {
				conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					return err
				}

				for _, config := range conf.Config {
					if strings.Contains(config.File, "SnippetsPolicy_location_snippets-policy-valid-sp.conf") {
						return fmt.Errorf("expected SnippetsPolicy config file to be removed, but it still exists")
					}
				}

				return nil
			}).WithTimeout(timeoutConfig.GetStatusTimeout).WithPolling(500 * time.Millisecond).Should(Succeed())

			// Traffic should still work
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			baseURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
			Expect(expectRequestToSucceed(baseURL, address, "URI: /coffee")).To(Succeed())
		})
	})

	When("SnippetsPolicy is invalid", func() {
		Specify("if syntax is invalid", func() {
			files := []string{"snippets-policy/invalid-syntax-sp.yaml"}
			gatewayNsName := types.NamespacedName{Name: "gateway", Namespace: namespace}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			Eventually(checkGatewayToHaveProgrammedCond).
				WithArguments(gatewayNsName, metav1.ConditionFalse, string(v1.GatewayReasonInvalid)).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})
	})
})

func checkGatewayToHaveProgrammedCond(
	gatewayNsName types.NamespacedName,
	status metav1.ConditionStatus,
	reason string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var gw v1.Gateway
	if err := resourceManager.Get(ctx, gatewayNsName, &gw); err != nil {
		return err
	}

	for _, cond := range gw.Status.Conditions {
		if cond.Type == string(v1.GatewayConditionProgrammed) {
			if cond.Status == status && cond.Reason == reason {
				return nil
			}
			return fmt.Errorf(
				"expected Programmed condition to be %s/%s, got %s/%s",
				status,
				reason,
				cond.Status,
				cond.Reason,
			)
		}
	}

	return fmt.Errorf("Programmed condition not found")
}

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
