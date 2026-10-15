package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("ProxySettingsPolicy", Ordered, Label("functional", "proxy-settings"), func() {
	var (
		files = []string{
			"proxy-settings-policy/app.yaml",
			"proxy-settings-policy/gateway.yaml",
			"proxy-settings-policy/routes.yaml",
			"proxy-settings-policy/grpc-backend.yaml",
		}

		namespace string

		nginxPodName string
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("proxy-settings-%d", GinkgoParallelProcess())

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

	When("valid ProxySettingsPolicies are created for both: Gateway and HTTPRoute", func() {
		var (
			policies = []string{
				"proxy-settings-policy/gateway-proxy-settings.yaml",
				"proxy-settings-policy/coffee-http-proxy-settings.yaml",
			}

			baseURL string
		)

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseURL = helpers.BuildPortFwdURL("cafe.example.com", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policies are Accepted", func() {
			policyExpectations := []policyStatusExpectation{
				createPolicyExpectation(
					"gateway-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
				createPolicyExpectation(
					"coffee-http-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
			}
			waitForPoliciesVerification(policyExpectations)

			condType := string(conditions.ProxySettingsPolicyAffected)
			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				framework.VerifyRequest(
					timeoutConfig,
					baseURL+"/coffee",
					address,
					framework.RequestSucceedsWith("Coffee chunk"),
					framework.WithContextDisabled(),
				)
				framework.VerifyRequest(timeoutConfig, baseURL+"/tea", address, framework.RequestSucceedsWith("URI: /tea"))
			})
		})

		Context("nginx config", func() {
			var (
				conf       *framework.Payload
				filePrefix string
			)

			BeforeAll(func() {
				filePrefix = fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("is set properly for",
				func(getExpCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getExpCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
							File:      "http.conf",
						},
						{
							Directive: "proxy_buffer_size",
							Value:     "4k",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_buffers",
							Value:     "8 4k",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_busy_buffers_size",
							Value:     "16k",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
					}
				}),
				Entry("coffee route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_coffee-http-proxy-settings.conf", filePrefix),
							File:      "http.conf",
							Server:    "cafe.example.com",
							Location:  "/coffee",
						},
						{
							Directive: "proxy_buffer_size",
							Value:     "16k",
							File:      fmt.Sprintf("%s_coffee-http-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_buffers",
							Value:     "16 64k",
							File:      fmt.Sprintf("%s_coffee-http-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_busy_buffers_size",
							Value:     "128k",
							File:      fmt.Sprintf("%s_coffee-http-proxy-settings.conf", filePrefix),
						},
					}
				}),
			)
		})
	})

	When("valid ProxySettingsPolicies are created for both: Gateway and GRPCRoute", func() {
		policies := []string{
			"proxy-settings-policy/gateway-and-coffee-enabled-grpc-proxy-settings.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policies are Accepted", func() {
			policyExpectations := []policyStatusExpectation{
				createPolicyExpectation(
					"gateway-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
				createPolicyExpectation(
					"coffee-grpc-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
			}
			waitForPoliciesVerification(policyExpectations)

			condType := string(conditions.ProxySettingsPolicyAffected)
			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForGRPCRoutePolicyAffected(
				types.NamespacedName{Name: "grpc-route", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("nginx config", func() {
			var (
				conf       *framework.Payload
				filePrefix string
			)

			BeforeAll(func() {
				filePrefix = fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("is set properly for",
				func(getExpCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getExpCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
							File:      "http.conf",
						},
						{
							Directive: "proxy_buffering",
							Value:     "on",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_buffer_size",
							Value:     "16k",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_buffers",
							Value:     "16 64k",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_busy_buffers_size",
							Value:     "128k",
							File:      fmt.Sprintf("%s_gateway-proxy-settings.conf", filePrefix),
						},
					}
				}),
				Entry("grpc route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_coffee-grpc-proxy-settings.conf", filePrefix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/helloworld.Greeter/SayHello",
						},
						{
							Directive: "proxy_buffering",
							Value:     "on",
							File:      fmt.Sprintf("%s_coffee-grpc-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_buffer_size",
							Value:     "4k",
							File:      fmt.Sprintf("%s_coffee-grpc-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_buffers",
							Value:     "8 4k",
							File:      fmt.Sprintf("%s_coffee-grpc-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_busy_buffers_size",
							Value:     "16k",
							File:      fmt.Sprintf("%s_coffee-grpc-proxy-settings.conf", filePrefix),
						},
					}
				}),
			)
		})
	})

	When("valid ProxySettingsPolicies are created for HTTPRoute only", func() {
		var (
			policies = []string{
				"proxy-settings-policy/coffee-http-proxy-settings.yaml",
			}

			baseURL string
		)

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseURL = helpers.BuildPortFwdURL("cafe.example.com", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policy is Accepted", func() {
			waitForPoliciesVerification([]policyStatusExpectation{
				createPolicyExpectation(
					"coffee-http-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
			})

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
				string(conditions.ProxySettingsPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				framework.VerifyRequest(
					timeoutConfig,
					baseURL+"/coffee",
					address,
					framework.RequestSucceedsWith("Coffee chunk"),
					framework.WithContextDisabled(),
				)
				framework.VerifyRequest(timeoutConfig, baseURL+"/tea", address, framework.RequestSucceedsWith("URI: /tea"))
			})
		})
	})

	When("valid ProxySettingsPolicies are created for Gateway only", func() {
		var (
			policies = []string{
				"proxy-settings-policy/gateway-proxy-settings.yaml",
			}

			baseURL string
		)

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseURL = helpers.BuildPortFwdURL("cafe.example.com", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policy is Accepted", func() {
			waitForPoliciesVerification([]policyStatusExpectation{
				createPolicyExpectation(
					"gateway-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
			})

			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace},
				string(conditions.ProxySettingsPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response only for HTTPRoute tea, and fail for coffee", func() {
				framework.VerifyRequest(
					timeoutConfig,
					baseURL+"/coffee",
					address,
					framework.RequestFails(),
					framework.WithContextDisabled(),
				)
				framework.VerifyRequest(timeoutConfig, baseURL+"/tea", address, framework.RequestSucceedsWith("URI: /tea"))
			})
		})
	})

	When("valid HTTPRoute ProxySettingsPolicies with more than one TargetRef", func() {
		policies := []string{
			"proxy-settings-policy/two-targetrefs-http-proxy-settings.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policy is Accepted", func() {
			waitForPoliciesVerification([]policyStatusExpectation{
				createPolicyExpectation(
					"coffee-http-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
			})

			condType := string(conditions.ProxySettingsPolicyAffected)
			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "tea", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})
	})

	When("invalid conflicting HTTPRoute ProxySettingsPolicies", func() {
		policies := []string{
			"proxy-settings-policy/conflicting-http-proxy-settings.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Context("verify that conflicting HTTPRoute ProxySettingsPolicies are not accepted", func() {
			Specify("policies have correct status", func() {
				policyExpectations := []policyStatusExpectation{
					createPolicyExpectation(
						"http-proxy-settings-1",
						namespace,
						metav1.ConditionTrue,
						gatewayv1.PolicyReasonAccepted,
					),
					createPolicyExpectation(
						"http-proxy-settings-2",
						namespace,
						metav1.ConditionFalse,
						gatewayv1.PolicyReasonConflicted,
					),
					createPolicyExpectation(
						"http-proxy-settings-3",
						namespace,
						metav1.ConditionFalse,
						gatewayv1.PolicyReasonConflicted,
					),
				}
				waitForPoliciesVerification(policyExpectations)
			})
		})
	})

	When("valid ProxySettingsPolicies with timeout fields are created for Gateway", func() {
		var (
			policies = []string{
				"proxy-settings-policy/gateway-timeout-proxy-settings.yaml",
			}

			baseURL string
		)

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseURL = helpers.BuildPortFwdURL("cafe.example.com", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policy is Accepted", func() {
			waitForPoliciesVerification([]policyStatusExpectation{
				createPolicyExpectation(
					"gateway-timeout-proxy-settings",
					namespace,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				),
			})

			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace},
				string(conditions.ProxySettingsPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				framework.VerifyRequest(timeoutConfig, baseURL+"/tea", address, framework.RequestSucceedsWith("URI: /tea"))
			})
		})

		Context("nginx config", func() {
			var (
				conf       *framework.Payload
				filePrefix string
			)

			BeforeAll(func() {
				filePrefix = fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("is set properly for",
				func(getExpCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getExpCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway timeout policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_gateway-timeout-proxy-settings.conf", filePrefix),
							File:      "http.conf",
						},
						{
							Directive: "proxy_connect_timeout",
							Value:     "5s",
							File:      fmt.Sprintf("%s_gateway-timeout-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_read_timeout",
							Value:     "60s",
							File:      fmt.Sprintf("%s_gateway-timeout-proxy-settings.conf", filePrefix),
						},
						{
							Directive: "proxy_send_timeout",
							Value:     "30s",
							File:      fmt.Sprintf("%s_gateway-timeout-proxy-settings.conf", filePrefix),
						},
					}
				}),
			)
		})
	})

	When("invalid buffering Gateway and HTTPRoute ProxySettingsPolicies", func() {
		policies := []string{
			"proxy-settings-policy/gateway-and-coffee-invalid-buffers-proxy-settings.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("policies are Invalid", func() {
			policyExpectations := []policyStatusExpectation{
				createPolicyExpectation(
					"coffee-http-proxy-settings",
					namespace,
					metav1.ConditionFalse,
					gatewayv1.PolicyReasonInvalid,
				),
				createPolicyExpectation(
					"gateway-proxy-settings",
					namespace,
					metav1.ConditionFalse,
					gatewayv1.PolicyReasonInvalid,
				),
			}
			waitForPoliciesVerification(policyExpectations)
		})
	})
})

type policyStatusExpectation struct {
	nsname   types.NamespacedName
	expected framework.PolicyCondition
}

// createPolicyExpectation creates a single policy status expectation.
func createPolicyExpectation(
	name, ns string,
	status metav1.ConditionStatus,
	reason gatewayv1.PolicyConditionReason,
) policyStatusExpectation {
	return policyStatusExpectation{
		nsname:   types.NamespacedName{Name: name, Namespace: ns},
		expected: framework.PolicyCondition{Status: status, Reason: reason},
	}
}

// waitForPoliciesVerification waits for multiple ProxySettingsPolicies to reach
// the expected condition status.
func waitForPoliciesVerification(policyExpectations []policyStatusExpectation) {
	GinkgoHelper()
	for _, expectation := range policyExpectations {
		Expect(resourceManager.WaitForPolicyStatus(
			expectation.nsname,
			&ngfAPI.ProxySettingsPolicy{},
			expectation.expected,
			framework.AncestorsMatchTargetRefs,
		)).To(Succeed())
	}
}
