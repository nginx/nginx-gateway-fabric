package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
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
			Expect(resourceManager.DeleteFromFiles(snippetsPolicy, namespace)).To(Succeed())
		})
		Specify("snippetsPolicies are accepted", func() {
			snippetsPolicyNames := []string{
				"valid-sp",
			}
			for _, name := range snippetsPolicyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}
				Expect(waitForSnippetsPolicyToBeAccepted(nsname)).To(
					Succeed(), fmt.Sprintf("%s was not accepted", name),
				)
			}

			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace},
				string(conditions.SnippetsPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Specify("empty snippets policy is accepted", func() {
			files := []string{"snippets-policy/empty-snippets-sp.yaml"}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			nsname := types.NamespacedName{Name: "empty-snippets-sp", Namespace: namespace}
			Expect(waitForSnippetsPolicyToBeAccepted(nsname)).To(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoute", func() {
				port := helpers.BuildPortFwdPort(80, portFwdPort)
				baseURL := helpers.BuildPortFwdURL("cafe.example.com/coffee", port)

				Eventually(
					func() error {
						return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, baseURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("should forward TCPRoute traffic to tcp-coffee", func() {
				port := helpers.BuildPortFwdPort(81, portFwdPort)

				Eventually(func() error {
					conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), timeoutConfig.RequestTimeout)
					if err != nil {
						return err
					}
					defer conn.Close()

					return nil
				}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("should return a 200 response with the policy header for a header-matched HTTPRoute", func() {
				expectSnippetsResponseHeaders(
					"/tea",
					map[string]string{"version": "v1"},
					map[string]string{"X-Snippets-Policy": "valid"},
					nil,
				)
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
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
						Directive: "resolver_timeout",
						Value:     "10s",
						File:      "SnippetsPolicy_stream_snippets-policy-valid-sp.conf",
					},
					{
						Directive: "proxy_timeout",
						Value:     "7s",
						File:      "SnippetsPolicy_stream_server_snippets-policy-valid-sp.conf",
						AnyServer: true,
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
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_stream_snippets-policy-valid-sp.conf",
						File:      "stream.conf",
					},
					{
						Directive: "include",
						Value:     "/etc/nginx/includes/SnippetsPolicy_stream_server_snippets-policy-valid-sp.conf",
						File:      "stream.conf",
						AnyServer: true,
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
				Expect(waitForSnippetsPolicyToBeAccepted(nsname)).To(Succeed())
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

			expectSnippetsResponseHeaders(
				"/coffee",
				nil,
				map[string]string{"X-Snippets-Policy": "valid", "X-Second-Policy": "true"},
				nil,
			)
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

			// Traffic should still work, without the deleted policy's header.
			expectSnippetsResponseHeaders("/coffee", nil, nil, []string{"X-Snippets-Policy"})
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

func waitForSnippetsPolicyToBeAccepted(nsName types.NamespacedName) error {
	return resourceManager.WaitForPolicyStatus(
		nsName,
		&ngfAPI.SnippetsPolicy{},
		framework.PolicyAccepted,
		framework.AncestorsMatchTargetRefs,
	)
}

// expectSnippetsResponseHeaders polls cafe.example.com<path> until it returns 200 from that path with
// exactly the wanted header values and none of the absent headers.
func expectSnippetsResponseHeaders(
	path string,
	requestHeaders map[string]string,
	wantHeaders map[string]string,
	absentHeaders []string,
) {
	GinkgoHelper()
	port := helpers.BuildPortFwdPort(80, portFwdPort)
	request := framework.Request{
		URL:     helpers.BuildPortFwdURL("cafe.example.com"+path, port),
		Address: address,
		Headers: requestHeaders,
		Timeout: timeoutConfig.RequestTimeout,
	}

	Eventually(func(g Gomega) {
		resp, err := framework.Get(request)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
		g.Expect(resp.Body).To(ContainSubstring("URI: " + path))
		for name, value := range wantHeaders {
			g.Expect(resp.Headers.Values(name)).To(Equal([]string{value}), "header %s", name)
		}
		for _, name := range absentHeaders {
			g.Expect(resp.Headers.Values(name)).To(BeEmpty(), "header %s should be absent", name)
		}
	}).
		WithTimeout(timeoutConfig.GetStatusTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}
