package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
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

var _ = Describe("ClientSettingsPolicy", Ordered, Label("functional", "cspolicy"), func() {
	var (
		files = []string{
			"clientsettings/cafe.yaml",
			"clientsettings/gateway.yaml",
			"clientsettings/cafe-routes.yaml",
			"clientsettings/grpc-route.yaml",
			"clientsettings/grpc-backend.yaml",
		}

		namespace string

		nginxPodName string
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("clientsettings-%d", GinkgoParallelProcess())

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

	When("valid ClientSettingsPolicies are created", func() {
		var (
			policies = []string{
				"clientsettings/valid-csps.yaml",
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

		Specify("they are accepted by the target resource", func() {
			policyNames := []string{
				"gw-csp",
				"coffee-route-csp",
				"tea-route-csp",
				"soda-route-csp",
				"grpc-route-csp",
			}

			for _, name := range policyNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForCSPolicyStatus(nsname, framework.PolicyAccepted)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}

			condType := string(conditions.ClientSettingsPolicyAffected)
			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			for _, route := range []string{"coffee", "tea", "soda"} {
				Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
					types.NamespacedName{Name: route, Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
				)).To(Succeed())
			}

			Expect(resourceManager.WaitForGRPCRoutePolicyAffected(
				types.NamespacedName{Name: "grpc-route", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoutes", func() {
				baseCoffeeURL := baseURL + "/coffee"
				baseTeaURL := baseURL + "/tea"

				Eventually(
					func() error {
						return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, baseCoffeeURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				Eventually(
					func() error {
						return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, baseTeaURL, address, "URI: /tea")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx config", func() {
			var (
				conf       *framework.Payload
				filePrefix string
			)

			BeforeAll(func() {
				filePrefix = fmt.Sprintf("/etc/nginx/includes/ClientSettingsPolicy_%s", namespace)
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
							Value:     fmt.Sprintf("%s_gw-csp.conf", filePrefix),
							File:      "http.conf",
							Server:    "*.example.com",
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_gw-csp.conf", filePrefix),
							File:      "http.conf",
							Server:    "cafe.example.com",
						},
						{
							Directive: "client_max_body_size",
							Value:     "1000",
							File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						},
						{
							Directive: "client_body_timeout",
							Value:     "30s",
							File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						},
						{
							Directive: "keepalive_requests",
							Value:     "100",
							File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						},
						{
							Directive: "keepalive_time",
							Value:     "5s",
							File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						},
						{
							Directive: "keepalive_min_timeout",
							Value:     "3s",
							File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						},
						{
							Directive: "keepalive_timeout",
							Value:     "2s 1s",
							File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						},
					}
				}),
				Entry("coffee route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_coffee-route-csp.conf", filePrefix),
							File:      "http.conf",
							Server:    "cafe.example.com",
							Location:  "/coffee",
						},
						{
							Directive: "client_max_body_size",
							Value:     "2000",
							File:      fmt.Sprintf("%s_coffee-route-csp.conf", filePrefix),
						},
					}
				}),
				Entry("tea route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_tea-route-csp.conf", filePrefix),
							File:      "http.conf",
							Server:    "cafe.example.com",
							Location:  "/tea",
						},
						{
							Directive: "keepalive_requests",
							Value:     "200",
							File:      fmt.Sprintf("%s_tea-route-csp.conf", filePrefix),
						},
					}
				}),
				Entry("soda route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_soda-route-csp.conf", filePrefix),
							File:      "http.conf",
							Server:    "cafe.example.com",
							Location:  "/soda",
						},
						{
							Directive: "client_max_body_size",
							Value:     "3000",
							File:      fmt.Sprintf("%s_soda-route-csp.conf", filePrefix),
						},
					}
				}),
				Entry("grpc route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s_grpc-route-csp.conf", filePrefix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/helloworld.Greeter/SayHello",
						},
						{
							Directive: "client_max_body_size",
							Value:     "0",
							File:      fmt.Sprintf("%s_grpc-route-csp.conf", filePrefix),
						},
					}
				}),
			)
		})

		// We only test that the client_max_body_size directive in this test is propagated correctly.
		// This is because we can easily verify this directive by sending requests with different sized payloads.
		DescribeTable("client_max_body_size requests work as expected",
			func(uri string, byteLengthOfRequestBody, expStatus int) {
				url := baseURL + uri

				payload := make([]byte, byteLengthOfRequestBody)
				_, err := rand.Read(payload)
				Expect(err).ToNot(HaveOccurred())

				request := framework.Request{
					URL:     url,
					Address: address,
					Body:    bytes.NewReader(payload),
					Timeout: timeoutConfig.RequestTimeout,
				}
				resp, err := framework.Post(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(HaveHTTPStatus(expStatus))

				if expStatus == http.StatusOK {
					Expect(resp).To(HaveHTTPBody(ContainSubstring(fmt.Sprintf("URI: %s", uri))))
				}
			},
			func(uri string, byteLengthOfRequestBody, expStatus int) string {
				return fmt.Sprintf(
					"request body of %d should return %d for %s",
					byteLengthOfRequestBody,
					expStatus,
					uri,
				)
			},
			Entry(nil, "/tea", 900, http.StatusOK),
			Entry(nil, "/tea", 1200, http.StatusRequestEntityTooLarge),
			Entry(nil, "/coffee", 1200, http.StatusOK),
			Entry(nil, "/coffee", 2500, http.StatusRequestEntityTooLarge),
			Entry(nil, "/soda", 2500, http.StatusOK),
			Entry(nil, "/soda", 3300, http.StatusRequestEntityTooLarge),
		)
	})

	When("a ClientSettingsPolicy targets an invalid resources", func() {
		Specify("their accepted condition is set to TargetNotFound", func() {
			files := []string{
				"clientsettings/invalid-route-csp.yaml",
			}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			nsname := types.NamespacedName{Name: "invalid-route-csp", Namespace: namespace}
			Expect(waitForCSPolicyStatus(nsname, framework.PolicyCondition{
				Status: metav1.ConditionFalse,
				Reason: v1.PolicyReasonTargetNotFound,
			})).To(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})
	})

	Context("Merging behavior", func() {
		When("multiple policies target the same resource", func() {
			Specify("policies that cannot be merged are marked as conflicted", func() {
				policies := []string{
					"clientsettings/merging-csps.yaml",
				}

				mergeablePolicyNames := []string{
					"hr-merge-1",
					"hr-merge-2",
					"hr-merge-3",
					"grpc-merge-1",
					"grpc-merge-2",
				}

				conflictedPolicyNames := []string{
					"z-hr-conflict-1",
					"z-hr-conflict-2",
					"z-hr-conflict-3",
					"z-grpc-conflict",
					"z-grpc-conflict-2",
				}

				Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())

				for _, name := range conflictedPolicyNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					err := waitForCSPolicyStatus(nsname, framework.PolicyCondition{
						Status: metav1.ConditionFalse,
						Reason: v1.PolicyReasonConflicted,
					})
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not marked as conflicted", name))
				}

				for _, name := range mergeablePolicyNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					err := waitForCSPolicyStatus(nsname, framework.PolicyAccepted)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
				}

				Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
			})
		})
	})

	// Regression test: overlap-route-1 and overlap-route-2 share the same
	// gateway:hostname:port/path ("overlap.example.com", PathPrefix "/") and
	// therefore overlap with each other. The CSP targets the coffee route
	// ("cafe.example.com", "/coffee"), which shares no path with either of
	// them. The overlap between the two non-targeted routes must not produce
	// a false-positive TargetConflict on the CSP.
	Context("TargetConflict regression", func() {
		When("two non-targeted routes overlap with each other but not with the policy target", func() {
			targetConflictFiles := []string{
				"clientsettings/target-conflict-overlapping-routes.yaml",
				"clientsettings/target-conflict-csp.yaml",
			}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(targetConflictFiles, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(targetConflictFiles, namespace)).To(Succeed())
			})

			Specify("the policy is accepted without a false-positive TargetConflict", func() {
				nsname := types.NamespacedName{Name: "target-conflict-csp", Namespace: namespace}
				Expect(waitForCSPolicyStatus(nsname, framework.PolicyAccepted)).To(Succeed())
			})
		})
	})
})

func waitForCSPolicyStatus(policyNsname types.NamespacedName, expected framework.PolicyCondition) error {
	return resourceManager.WaitForPolicyStatus(
		policyNsname,
		&ngfAPI.ClientSettingsPolicy{},
		expected,
		framework.AncestorsMatchTargetRefs,
	)
}
