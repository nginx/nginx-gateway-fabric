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
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
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

		namespace = "proxy-settings"

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

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"gateway-proxy-settings",
				"coffee-http-proxy-settings",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForPSPolicyStatus(
					nsname,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				baseCoffeeURL := baseURL + "/coffee"
				baseTeaURL := baseURL + "/tea"

				Eventually(
					func() error {
						return expectRequestToSucceed(baseCoffeeURL, address, "Coffee chunk", framework.WithContextDisabled())
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				Eventually(
					func() error {
						return expectRequestToSucceed(baseTeaURL, address, "URI: /tea")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx config", func() {
			var conf *framework.Payload
			filePrefix := fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)

			BeforeAll(func() {
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("is set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway policy", []framework.ExpectedNginxField{
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
				}),
				Entry("coffee route policy", []framework.ExpectedNginxField{
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

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"gateway-proxy-settings",
				"coffee-grpc-proxy-settings",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForPSPolicyStatus(
					nsname,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("nginx config", func() {
			var conf *framework.Payload
			filePrefix := fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)

			BeforeAll(func() {
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("is set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway policy", []framework.ExpectedNginxField{
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
				}),
				Entry("grpc route policy", []framework.ExpectedNginxField{
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

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"coffee-http-proxy-settings",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForPSPolicyStatus(
					nsname,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				baseCoffeeURL := baseURL + "/coffee"
				baseTeaURL := baseURL + "/tea"

				Eventually(
					func() error {
						return expectRequestToSucceed(baseCoffeeURL, address, "Coffee chunk", framework.WithContextDisabled())
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				Eventually(
					func() error {
						return expectRequestToSucceed(baseTeaURL, address, "URI: /tea")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx config", func() {
			var conf *framework.Payload
			filePrefix := fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)

			BeforeAll(func() {
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("is set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("coffee route policy", []framework.ExpectedNginxField{
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
				}),
			)
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

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseURL = fmt.Sprintf("http://cafe.example.com:%d", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
		})

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"gateway-proxy-settings",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForPSPolicyStatus(
					nsname,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response only for HTTPRoute tea, and fail for coffee", func() {
				baseCoffeeURL := baseURL + "/coffee"
				baseTeaURL := baseURL + "/tea"

				Eventually(
					func() error {
						return expectRequestToSucceed(baseCoffeeURL, address, "Coffee chunk", framework.WithContextDisabled())
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					ShouldNot(Succeed())

				Eventually(
					func() error {
						return expectRequestToSucceed(baseTeaURL, address, "URI: /tea")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx config", func() {
			var conf *framework.Payload
			filePrefix := fmt.Sprintf("/etc/nginx/includes/ProxySettingsPolicy_%s", namespace)

			BeforeAll(func() {
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("is set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway policy", []framework.ExpectedNginxField{
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
				}),
			)
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

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"coffee-http-proxy-settings",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForPSPolicyStatus(
					nsname,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
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
			type test struct {
				desc            string
				nsname          types.NamespacedName
				conditionStatus metav1.ConditionStatus
				conditionReason gatewayv1.PolicyConditionReason
			}

			DescribeTable("Accepted and Conflicted conditions are set properly",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
						Eventually(waitForPSPolicyStatus).
							WithArguments(
								test.nsname,
								test.conditionStatus,
								test.conditionReason,
							).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(500 * time.Millisecond).
							Should(Succeed())
					}
				},
				Entry("conditions expectations", []test{
					{
						desc:            "http-proxy-settings-1 Accepted",
						nsname:          types.NamespacedName{Name: "http-proxy-settings-1", Namespace: namespace},
						conditionStatus: metav1.ConditionTrue,
						conditionReason: gatewayv1.PolicyReasonAccepted,
					},
					{
						desc:            "http-proxy-settings-2 Conflicted",
						nsname:          types.NamespacedName{Name: "http-proxy-settings-2", Namespace: namespace},
						conditionStatus: metav1.ConditionFalse,
						conditionReason: gatewayv1.PolicyReasonConflicted,
					},
					{
						desc:            "http-proxy-settings-3 Conflicted",
						nsname:          types.NamespacedName{Name: "http-proxy-settings-3", Namespace: namespace},
						conditionStatus: metav1.ConditionFalse,
						conditionReason: gatewayv1.PolicyReasonConflicted,
					},
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

		Specify("they are not accepted by the target resource", func() {
			policyNames := []string{
				"coffee-http-proxy-settings",
				"gateway-proxy-settings",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForPSPolicyStatus(
					nsname,
					metav1.ConditionFalse,
					gatewayv1.PolicyReasonInvalid,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})
	})
})

func waitForPSPolicyStatus(
	psPolicyNsName types.NamespacedName,
	condStatus metav1.ConditionStatus,
	condReason gatewayv1.PolicyConditionReason,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout*2)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for ProxySettings Policy %q to have the condition %q/%q\n",
		psPolicyNsName,
		condStatus,
		condReason,
	)

	return wait.PollUntilContextCancel(
		ctx,
		2000*time.Millisecond,
		true,
		func(ctx context.Context) (bool, error) {
			var psPolicy ngfAPI.ProxySettingsPolicy

			if err := resourceManager.Get(ctx, psPolicyNsName, &psPolicy); err != nil {
				return false, err
			}

			// ProxySettingsPolicy can have 1 or more ancestors
			if len(psPolicy.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("ProxySettingsPolicy %q does not have an ancestor status yet\n", psPolicy)

				return false, nil
			}

			ancestors := psPolicy.Status.Ancestors

			for _, ancestor := range ancestors {
				if err := ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason); err != nil {
					return false, err
				}
			}
			return true, nil
		},
	)
}
