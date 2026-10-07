package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("AccessPolicy", Ordered, Label("functional", "access-policy"), func() {
	var (
		baseFiles = []string{
			"access-policy/nginx-proxy.yaml",
			"access-policy/apps.yaml",
			"access-policy/gateway.yaml",
			"access-policy/routes.yaml",
		}

		namespace    string
		nginxPodName string
		apFilePrefix string
		coffeeURL    string
		teaURL       string
		redirectURL  string
		corsURL      string
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("access-policy-%d", GinkgoParallelProcess())
		apFilePrefix = fmt.Sprintf("/etc/nginx/includes/AccessPolicy_%s", namespace)

		ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(baseFiles, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		pods, err := resourceManager.GetReadyNginxPodNames(namespace, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(pods).To(HaveLen(1))
		nginxPodName = pods[0]

		setUpPortForward(nginxPodName, namespace)

		port := helpers.BuildPortFwdPort(80, portFwdPort)
		coffeeURL = helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
		teaURL = helpers.BuildPortFwdURL("cafe.example.com/tea", port)
		redirectURL = helpers.BuildPortFwdURL("cafe.example.com/redirect-me", port)
		corsURL = helpers.BuildPortFwdURL("cafe.example.com/cors-path", port)
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		cleanUpPortForward()
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	Context("when an AccessPolicy is attached to only a Gateway or a Route", func() {
		When("the policy is attached to the Gateway only", func() {
			policyFiles := []string{"access-policy/gateway-allow-policy.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			})
			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			})

			Specify("the policy is accepted and all affected resources have the AccessPolicyAffected condition", func() {
				Expect(waitForAccessPolicyAccepted(
					types.NamespacedName{Name: "gateway-allow", Namespace: namespace},
				)).To(Succeed())
				Expect(waitForGatewayAccessPolicyAffected(
					types.NamespacedName{Name: "gateway", Namespace: namespace},
				)).To(Succeed())

				for _, route := range []string{"coffee", "tea"} {
					Expect(waitForHTTPRouteAccessPolicyAffected(
						types.NamespacedName{Name: route, Namespace: namespace},
					)).To(Succeed())
				}
			})

			It("allows requests from the trusted range to all Routes on that Gateway", func() {
				eventuallyExpect(coffeeURL, http.StatusOK, "192.0.2.1")
				eventuallyExpect(teaURL, http.StatusOK, "192.0.2.1")
			})

			Context("nginx directives", func() {
				var conf *framework.Payload

				BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

				It("includes the allow directive and terminal deny all in the server block", func() {
					policyFile := fmt.Sprintf("%s_gateway-allow_server.conf", apFilePrefix)
					terminalFile := "/etc/nginx/includes/AccessPolicy_terminal_deny_all_server.conf"

					for _, field := range []framework.ExpectedNginxField{
						{Directive: "include", Value: policyFile, File: "http.conf", Server: "cafe.example.com"},
						{Directive: "allow", Value: "192.0.2.0/24", File: policyFile},
						{Directive: "include", Value: terminalFile, File: "http.conf", Server: "cafe.example.com"},
						{Directive: "deny", Value: "all", File: terminalFile},
					} {
						Expect(framework.ValidateNginxFieldExists(conf, field)).To(Succeed())
					}
				})
			})
		})

		When("the policy is attached to a Route only", func() {
			policyFiles := []string{"access-policy/route-deny-policy.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			})
			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			})

			Specify("the policy is accepted and the coffee HTTPRoute has the AccessPolicyAffected condition", func() {
				Expect(waitForAccessPolicyAccepted(
					types.NamespacedName{Name: "route-deny", Namespace: namespace},
				)).To(Succeed())
				Expect(waitForHTTPRouteAccessPolicyAffected(
					types.NamespacedName{Name: "coffee", Namespace: namespace},
				)).To(Succeed())
			})

			Context("when traffic is sent to each Route", func() {
				It("blocks requests to the targeted coffee route", func() {
					eventuallyExpect(coffeeURL, http.StatusForbidden, "203.0.113.10")
				})

				It("allows requests to the untargeted tea route", func() {
					eventuallyExpect(teaURL, http.StatusOK, "203.0.113.10")
				})
			})

			Context("nginx directives", func() {
				var conf *framework.Payload

				BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

				It("includes the deny directive in the coffee location block, not in the server block", func() {
					policyFile := fmt.Sprintf("%s_route-deny_location.conf", apFilePrefix)
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     policyFile,
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/coffee",
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "deny",
						Value:     "203.0.113.10",
						File:      policyFile,
					})).To(Succeed())
				})
			})
		})
	})

	Context("when AccessPolicies are attached at both Gateway and Route level", func() {
		When("a Gateway Deny coexists with a Route Allow", func() {
			policyFiles := []string{"access-policy/gateway-deny-route-allow-policies.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			})
			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			})

			Specify("both policies are accepted and all affected resources have the AccessPolicyAffected condition", func() {
				for _, name := range []string{"gateway-deny", "coffee-allow"} {
					Expect(waitForAccessPolicyAccepted(
						types.NamespacedName{Name: name, Namespace: namespace},
					)).To(Succeed(), fmt.Sprintf("%s was not accepted", name))
				}
				Expect(waitForGatewayAccessPolicyAffected(
					types.NamespacedName{Name: "gateway", Namespace: namespace},
				)).To(Succeed())

				for _, route := range []string{"coffee", "tea"} {
					Expect(waitForHTTPRouteAccessPolicyAffected(
						types.NamespacedName{Name: route, Namespace: namespace},
					)).To(Succeed())
				}
			})

			It("blocks all Routes because the Gateway Deny takes precedence over any Route Allow", func() {
				eventuallyExpect(coffeeURL, http.StatusForbidden, "203.0.113.10")
				eventuallyExpect(teaURL, http.StatusForbidden, "203.0.113.10")
			})

			Context("nginx directives", func() {
				var conf *framework.Payload

				BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

				It("emits the Gateway deny before the Route allow in the coffee location, followed by deny all", func() {
					gwDenyFile := fmt.Sprintf("%s_gateway-deny_location.conf", apFilePrefix)
					routeAllowFile := fmt.Sprintf("%s_coffee-allow_location.conf", apFilePrefix)
					terminalFile := "/etc/nginx/includes/AccessPolicy_terminal_deny_all_location.conf"

					for _, field := range []framework.ExpectedNginxField{
						{Directive: "include", Value: gwDenyFile, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
						{Directive: "deny", Value: "203.0.113.10", File: gwDenyFile},
						{Directive: "include", Value: routeAllowFile, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
						{Directive: "allow", Value: "203.0.113.0/24", File: routeAllowFile},
						{Directive: "include", Value: terminalFile, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
						{Directive: "deny", Value: "all", File: terminalFile},
					} {
						Expect(framework.ValidateNginxFieldExists(conf, field)).To(Succeed())
					}
				})
			})
		})

		When("a Gateway Allow coexists with a Route Allow", func() {
			policyFiles := []string{"access-policy/gateway-allow-route-allow-policies.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			})
			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			})

			Specify("both policies are accepted and all affected resources have the AccessPolicyAffected condition", func() {
				for _, name := range []string{"gateway-allow-net1", "coffee-allow-net2"} {
					Expect(waitForAccessPolicyAccepted(
						types.NamespacedName{Name: name, Namespace: namespace},
					)).To(Succeed(), fmt.Sprintf("%s was not accepted", name))
				}
				Expect(waitForGatewayAccessPolicyAffected(
					types.NamespacedName{Name: "gateway", Namespace: namespace},
				)).To(Succeed())

				for _, route := range []string{"coffee", "tea"} {
					Expect(waitForHTTPRouteAccessPolicyAffected(
						types.NamespacedName{Name: route, Namespace: namespace},
					)).To(Succeed())
				}
			})

			Context("when traffic arrives at each Route", func() {
				It("allows requests to the coffee route from an IP in the Route Allow range", func() {
					eventuallyExpect(coffeeURL, http.StatusOK, "198.51.100.1")
				})

				It("blocks requests to the coffee route from an IP in the Gateway "+
					"Allow range because the Route Allow replaced the Gateway Allow", func() {
					eventuallyExpect(coffeeURL, http.StatusForbidden, "192.0.2.1")
				})

				It("allows requests to the tea route from an IP in the Gateway "+
					"Allow range because tea inherits the Gateway Allow", func() {
					eventuallyExpect(teaURL, http.StatusOK, "192.0.2.1")
				})

				It("blocks requests to the tea route from an IP outside the Gateway Allow range", func() {
					eventuallyExpect(teaURL, http.StatusForbidden, "198.51.100.1")
				})
			})

			Context("nginx directives", func() {
				var conf *framework.Payload

				BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

				It("emits only the Route Allow in the coffee location and Gateway Allow is absent", func() {
					routeAllowFile := fmt.Sprintf("%s_coffee-allow-net2_location.conf", apFilePrefix)
					gwAllowFile := fmt.Sprintf("%s_gateway-allow-net1_location.conf", apFilePrefix)

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     routeAllowFile,
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/coffee",
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "allow",
						Value:     "198.51.100.0/24",
						File:      routeAllowFile,
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     gwAllowFile,
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/coffee",
					})).ToNot(Succeed())
				})

				It("emits the Gateway Allow in the server block so that tea inherits it", func() {
					gwAllowServerFile := fmt.Sprintf("%s_gateway-allow-net1_server.conf", apFilePrefix)

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     gwAllowServerFile,
						File:      "http.conf",
						Server:    "cafe.example.com",
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "allow",
						Value:     "192.0.2.0/24",
						File:      gwAllowServerFile,
					})).To(Succeed())
				})
			})
		})
	})

	Context("when an AccessPolicy targets a Route with a special location type", func() {
		When("the route has a RequestRedirect filter", func() {
			policyFiles := []string{"access-policy/redirect-policy.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			})
			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			})

			Specify("the policy is accepted and the redirect HTTPRoute has the AccessPolicyAffected condition", func() {
				Expect(waitForAccessPolicyAccepted(
					types.NamespacedName{Name: "redirect-deny", Namespace: namespace},
				)).To(Succeed())
				Expect(waitForHTTPRouteAccessPolicyAffected(
					types.NamespacedName{Name: "redirect-route", Namespace: namespace},
				)).To(Succeed())
			})

			It("returns 403 instead of the 3xx redirect response when the client IP is denied", func() {
				eventuallyExpect(redirectURL, http.StatusForbidden, "203.0.113.10")
			})

			Context("nginx directives", func() {
				var conf *framework.Payload

				BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

				It("includes the geo block file in the HTTP context", func() {
					geoFile := fmt.Sprintf("%s_redirect-deny_geo.conf", apFilePrefix)
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     geoFile,
						File:      "http.conf",
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive:             "geo",
						File:                  geoFile,
						Value:                 "$ngf_ap",
						ValueSubstringAllowed: true,
					})).To(Succeed())
				})

				It("includes the if block inside the redirect location to enforce access in the rewrite phase", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     fmt.Sprintf("%s_redirect-deny_if_location.conf", apFilePrefix),
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/redirect-me",
					})).To(Succeed())
				})
			})
		})

		When("the route has a CORS filter", func() {
			policyFiles := []string{"access-policy/cors-policy.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
			})
			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			})

			Specify("the policy is accepted and the CORS HTTPRoute has the AccessPolicyAffected condition", func() {
				Expect(waitForAccessPolicyAccepted(
					types.NamespacedName{Name: "cors-deny", Namespace: namespace},
				)).To(Succeed())

				Expect(waitForHTTPRouteAccessPolicyAffected(
					types.NamespacedName{Name: "cors-route", Namespace: namespace},
				)).To(Succeed())
			})

			It("returns 403 on GET requests from denied IPs", func() {
				eventuallyExpect(corsURL, http.StatusForbidden, "203.0.113.10")
			})

			It("returns 403 on OPTIONS preflight requests from denied IPs", func() {
				Eventually(func() error {
					resp, err := framework.OptionsRequest(framework.Request{
						URL:     corsURL,
						Address: address,
						Timeout: timeoutConfig.GetTimeout,
						Headers: map[string]string{
							"Origin":                        "https://example.com",
							"Access-Control-Request-Method": "GET",
						},
						XForwardedFor: "203.0.113.10",
					})
					if err != nil {
						return fmt.Errorf("OPTIONS %s: %w", corsURL, err)
					}
					if resp.StatusCode != http.StatusForbidden {
						return fmt.Errorf("OPTIONS %s: expected status %d, got %d", corsURL, http.StatusForbidden, resp.StatusCode)
					}
					return nil
				}).WithTimeout(timeoutConfig.TestForTrafficTimeout).WithPolling(500 * time.Millisecond).Should(Succeed())
			})

			Context("nginx directives", func() {
				var conf *framework.Payload

				BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

				It("includes the geo block file in the HTTP context", func() {
					geoFile := fmt.Sprintf("%s_cors-deny_geo.conf", apFilePrefix)
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     geoFile,
						File:      "http.conf",
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive:             "geo",
						File:                  geoFile,
						Value:                 "$ngf_ap",
						ValueSubstringAllowed: true,
					})).To(Succeed())
				})

				It("includes the if block inside the CORS location for OPTIONS preflight enforcement", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     fmt.Sprintf("%s_cors-deny_if_location.conf", apFilePrefix),
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/cors-path",
					})).To(Succeed())
				})

				It("includes the allow/deny file inside the CORS location for proxied request enforcement", func() {
					policyFile := fmt.Sprintf("%s_cors-deny_location.conf", apFilePrefix)
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     policyFile,
						File:      "http.conf",
						Server:    "cafe.example.com",
						Location:  "/cors-path",
					})).To(Succeed())

					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "deny",
						Value:     "203.0.113.10",
						File:      policyFile,
					})).To(Succeed())
				})
			})
		})
	})

	When("multiple AccessPolicies target the same Route they are merged", func() {
		policyFiles := []string{
			"access-policy/multiple-deny-policies.yaml",
			"access-policy/multiple-route-allow-policies.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})
		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		Specify("all policies are accepted and the coffee HTTPRoute has the AccessPolicyAffected condition", func() {
			for _, name := range []string{"deny-host", "deny-range", "allow-net1", "allow-net2"} {
				Expect(waitForAccessPolicyAccepted(
					types.NamespacedName{Name: name, Namespace: namespace},
				)).To(Succeed(), fmt.Sprintf("%s was not accepted", name))
			}
			Expect(waitForHTTPRouteAccessPolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
			)).To(Succeed())
		})

		Context("when traffic is sent to the coffee route", func() {
			It("blocks requests matching the deny-host rule", func() {
				eventuallyExpect(coffeeURL, http.StatusForbidden, "203.0.113.10")
			})

			It("blocks requests matching the deny-range rule", func() {
				eventuallyExpect(coffeeURL, http.StatusForbidden, "203.0.113.50")
			})

			It("allows requests matching the allow-net1 rule", func() {
				eventuallyExpect(coffeeURL, http.StatusOK, "192.0.2.1")
			})

			It("allows requests matching the allow-net2 rule", func() {
				eventuallyExpect(coffeeURL, http.StatusOK, "198.51.100.1")
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

			It("emits deny and allow directives from all policies in the coffee location block", func() {
				denyHostFile := fmt.Sprintf("%s_deny-host_location.conf", apFilePrefix)
				denyRangeFile := fmt.Sprintf("%s_deny-range_location.conf", apFilePrefix)
				allowNet1File := fmt.Sprintf("%s_allow-net1_location.conf", apFilePrefix)
				allowNet2File := fmt.Sprintf("%s_allow-net2_location.conf", apFilePrefix)

				for _, field := range []framework.ExpectedNginxField{
					{Directive: "include", Value: denyHostFile, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
					{Directive: "deny", Value: "203.0.113.10", File: denyHostFile},
					{Directive: "include", Value: denyRangeFile, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
					{Directive: "deny", Value: "203.0.113.0/24", File: denyRangeFile},
					{Directive: "include", Value: allowNet1File, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
					{Directive: "allow", Value: "192.0.2.0/24", File: allowNet1File},
					{Directive: "include", Value: allowNet2File, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
					{Directive: "allow", Value: "198.51.100.0/24", File: allowNet2File},
				} {
					Expect(framework.ValidateNginxFieldExists(conf, field)).To(Succeed())
				}
			})
		})
	})

	When("an AccessPolicy has both IPv4 and IPv6 address rules", func() {
		policyFiles := []string{"access-policy/ipv6-policy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})
		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		Specify("the policy is accepted and the coffee HTTPRoute has the AccessPolicyAffected condition", func() {
			Expect(waitForAccessPolicyAccepted(
				types.NamespacedName{Name: "ipv4-ipv6-allow", Namespace: namespace},
			)).To(Succeed())
			Expect(waitForHTTPRouteAccessPolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
			)).To(Succeed())
		})

		It("allows requests from the IPv4 host address", func() {
			eventuallyExpect(coffeeURL, http.StatusOK, "192.0.2.1")
		})

		It("allows requests from the IPv6 loopback address via X-Forwarded-For", func() {
			eventuallyExpect(coffeeURL, http.StatusOK, "::1")
		})

		It("blocks requests from an address that matches neither rule", func() {
			eventuallyExpect(coffeeURL, http.StatusForbidden, "203.0.113.50")
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() { conf = getNginxConf(nginxPodName, namespace) })

			It("emits both the IPv4 and IPv6 allow directives in the coffee location", func() {
				policyFile := fmt.Sprintf("%s_ipv4-ipv6-allow_location.conf", apFilePrefix)
				for _, field := range []framework.ExpectedNginxField{
					{Directive: "include", Value: policyFile, File: "http.conf", Server: "cafe.example.com", Location: "/coffee"},
					{Directive: "allow", Value: "192.0.2.1", File: policyFile},
					{Directive: "allow", Value: "::1/128", File: policyFile},
				} {
					Expect(framework.ValidateNginxFieldExists(conf, field)).To(Succeed())
				}
			})
		})
	})

	When("an AccessPolicy is removed", func() {
		policyFiles := []string{"access-policy/route-deny-policy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})
		AfterAll(func() {
			// policy may already be deleted by the removal It block
			_ = resourceManager.DeleteFromFiles(policyFiles, namespace)
		})

		Specify("the policy is accepted and the coffee HTTPRoute has the AccessPolicyAffected condition", func() {
			Expect(waitForAccessPolicyAccepted(
				types.NamespacedName{Name: "route-deny", Namespace: namespace},
			)).To(Succeed())
			Expect(waitForHTTPRouteAccessPolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
			)).To(Succeed())
		})

		It("enforces the deny rule before removal", func() {
			eventuallyExpect(coffeeURL, http.StatusForbidden, "203.0.113.10")
		})

		It("after the policy is deleted, access rules are removed and coffee returns 200", func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
			eventuallyExpect(coffeeURL, http.StatusOK, "203.0.113.10")
		})

		It("removes the AccessPolicyAffected condition from the coffee HTTPRoute", func() {
			Expect(waitForHTTPRouteAccessPolicyAffectedGone(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
			)).To(Succeed())
		})
	})

	When("an AccessPolicy contains an invalid IP address", func() {
		policyFiles := []string{"access-policy/invalid-address-policy.yaml"}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
		})
		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
		})

		Specify("the policy is rejected with Accepted=False and the error message identifies the invalid address", func() {
			Expect(waitForAccessPolicyRejectedWithMessage(
				types.NamespacedName{Name: "invalid-address", Namespace: namespace},
				"must be a valid IPv4/IPv6 address or CIDR range",
			)).To(Succeed())
		})

		It("does not disrupt existing NGINX config and coffee continues to return 200", func() {
			eventuallyExpect(coffeeURL, http.StatusOK)
		})
	})
})

// eventuallyExpect asserts that url returns the expected HTTP status code within
// timeoutConfig.TestForTrafficTimeout. An optional X-Forwarded-For value may be
// provided to forge the client IP via the NginxProxy rewriteClientIP configuration.
func eventuallyExpect(url string, code int, xff ...string) {
	GinkgoHelper()
	var forwardedFor string
	if len(xff) > 0 {
		forwardedFor = xff[0]
	}
	Eventually(func() error {
		resp, err := framework.Get(framework.Request{
			URL:           url,
			Address:       address,
			Timeout:       timeoutConfig.GetTimeout,
			XForwardedFor: forwardedFor,
		})
		if err != nil {
			return fmt.Errorf("GET %s: %w", url, err)
		}
		if resp.StatusCode != code {
			return fmt.Errorf("GET %s: expected status %d, got %d", url, code, resp.StatusCode)
		}
		return nil
	}).WithTimeout(timeoutConfig.TestForTrafficTimeout).WithPolling(500 * time.Millisecond).Should(Succeed())
}

// getNginxConf fetches and parses the live NGINX configuration from the given pod.
func getNginxConf(podName, ns string) *framework.Payload {
	GinkgoHelper()
	conf, err := resourceManager.GetNginxConfig(podName, ns, "")
	Expect(err).ToNot(HaveOccurred())
	return conf
}

// waitForAccessPolicyAccepted polls until all of the AccessPolicy's ancestors are
// accepted.
func waitForAccessPolicyAccepted(nsName types.NamespacedName) error {
	return resourceManager.WaitForPolicyToBeAccepted(nsName, timeoutConfig.GetStatusTimeout,
		func(ctx context.Context) ([]gatewayv1.PolicyAncestorStatus, error) {
			var ap ngfAPI.AccessPolicy
			if err := resourceManager.Get(ctx, nsName, &ap); err != nil {
				return nil, err
			}

			if len(ap.Status.Ancestors) == 0 {
				return nil, nil
			}

			expectedCount := len(ap.Spec.TargetRefs)
			if len(ap.Status.Ancestors) != expectedCount {
				return nil, fmt.Errorf("policy has %d ancestors, expected %d", len(ap.Status.Ancestors), expectedCount)
			}

			for _, ancestor := range ap.Status.Ancestors {
				tr, ok := findTargetRefForAncestor(ancestor, ap.Spec.TargetRefs)
				if !ok {
					return nil, fmt.Errorf("no targetRef found for ancestor %v", ancestor.AncestorRef)
				}
				if err := ancestorMustEqualTargetRef(ancestor, tr, nsName.Namespace); err != nil {
					return nil, err
				}
			}

			return ap.Status.Ancestors, nil
		},
	)
}

// waitForAccessPolicyRejectedWithMessage polls until the AccessPolicy has Accepted=False
// with reason Invalid and a message containing messageSubstr.
func waitForAccessPolicyRejectedWithMessage(nsName types.NamespacedName, messageSubstr string) error {
	GinkgoWriter.Printf("Waiting for AccessPolicy %q to be rejected with message containing %q\n", nsName, messageSubstr)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var ap ngfAPI.AccessPolicy
		if err := resourceManager.Get(ctx, nsName, &ap); err != nil {
			return false, err
		}

		if len(ap.Status.Ancestors) == 0 {
			GinkgoWriter.Printf("AccessPolicy %q has no ancestor status yet\n", nsName)
			return false, nil
		}

		for _, ancestor := range ap.Status.Ancestors {
			for _, cond := range ancestor.Conditions {
				if cond.Type != string(gatewayv1.PolicyConditionAccepted) {
					continue
				}
				if cond.Status != metav1.ConditionFalse {
					return false, fmt.Errorf("expected Accepted=False, got %s", cond.Status)
				}
				if cond.Reason != string(gatewayv1.PolicyReasonInvalid) {
					return false, fmt.Errorf("expected reason %s, got %s", gatewayv1.PolicyReasonInvalid, cond.Reason)
				}
				if !strings.Contains(cond.Message, messageSubstr) {
					return false, fmt.Errorf("expected message to contain %q, got: %s", messageSubstr, cond.Message)
				}
				return true, nil
			}
		}
		return false, nil
	})
}

func waitForGatewayAccessPolicyAffected(nsName types.NamespacedName) error {
	condType := string(conditions.AccessPolicyAffected)
	return resourceManager.WaitForGatewayPolicyAffected(nsName, condType, timeoutConfig.GetStatusTimeout)
}

func waitForHTTPRouteAccessPolicyAffected(nsName types.NamespacedName) error {
	condType := string(conditions.AccessPolicyAffected)
	return resourceManager.WaitForHTTPRoutePolicyAffected(nsName, condType, timeoutConfig.GetStatusTimeout)
}

func waitForHTTPRouteAccessPolicyAffectedGone(nsName types.NamespacedName) error {
	condType := string(conditions.AccessPolicyAffected)
	return resourceManager.WaitForHTTPRoutePolicyAffectedGone(nsName, condType, timeoutConfig.GetStatusTimeout)
}
