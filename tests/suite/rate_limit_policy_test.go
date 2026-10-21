package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("RateLimitPolicy", Ordered, Label("functional", "rate-limit-policy"), func() {
	var (
		files = []string{
			"rate-limit-policy/apps.yaml",
			"rate-limit-policy/gateway.yaml",
			"rate-limit-policy/routes.yaml",
		}

		namespace    string
		nginxPodName string
		filePrefix   string
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("rate-limit-policy-%d", GinkgoParallelProcess())
		filePrefix = fmt.Sprintf("/etc/nginx/includes/RateLimitPolicy_%s", namespace)

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

	When("RateLimitPolicy is applied to Gateway, HTTPRoute and GRPCRoute", func() {
		rlpFiles := []string{
			"rate-limit-policy/gateway-rate-limit-policy.yaml",
			"rate-limit-policy/route-rate-limit-policies.yaml",
		}
		var baseCoffeeURL string

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(rlpFiles, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseCoffeeURL = helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(rlpFiles, namespace)).To(Succeed())
		})

		Specify("rateLimitPolicies are accepted", func() {
			rateLimitPolicies := []string{
				"gateway-rate-limit",
				"grpcroute-rate-limit",
				"httproute-rate-limit",
			}
			for _, rlp := range rateLimitPolicies {
				rlpNsName := types.NamespacedName{Name: rlp, Namespace: namespace}
				Expect(waitForRateLimitPolicyToBeAccepted(rlpNsName)).To(
					Succeed(), fmt.Sprintf("%s was not accepted", rlp),
				)
			}

			condType := string(conditions.RateLimitPolicyAffected)
			Expect(resourceManager.WaitForGatewayPolicyAffected(
				types.NamespacedName{Name: "gateway", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForGRPCRoutePolicyAffected(
				types.NamespacedName{Name: "grpc-route", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return HTTP 200 initially and a custom error code once the rate limit is exceeded", func() {
				Eventually(
					func() error {
						return verifyRateLimitPolicyWorksAsExpected(baseCoffeeURL, address, "URI: /coffee", 466)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("are set properly for",
				func(getCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("gateway policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
							File:      "http.conf",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
							Directive: "limit_req_zone",
							Value:     fmt.Sprintf("$binary_remote_addr zone=%s_rl_gateway-rate-limit_rule0:10m rate=15r/m", namespace),
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway_server.conf"),
							File:      "http.conf",
							Server:    "cafe.example.com",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway_server.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_gateway-rate-limit_rule0 burst=3", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway_server.conf"),
							Directive: "limit_req_log_level",
							Value:     "info",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway_server.conf"),
							Directive: "limit_req_status",
							Value:     "429",
						},
					}
				}),
				Entry("httproute policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_internal_http.conf"),
							File:      "http.conf",
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_route.conf"),
							File:      "http.conf",
							Server:    "cafe.example.com",
							Location:  "/coffee",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_internal_http.conf"),
							Directive: "limit_req_zone",
							Value:     fmt.Sprintf("$binary_remote_addr zone=%s_rl_httproute-rate-limit_rule0:20m rate=1r/m", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_route.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_httproute-rate-limit_rule0 burst=5", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_route.conf"),
							Directive: "limit_req_log_level",
							Value:     "warn",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_route.conf"),
							Directive: "limit_req_status",
							Value:     "466",
						},
					}
				}),
				Entry("grpcroute policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_internal_http.conf"),
							File:      "http.conf",
						},
						{
							Directive: "include",
							File:      "http.conf",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_route.conf"),
							Server:    "*.example.com",
							Location:  "/helloworld.Greeter/SayHello",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_internal_http.conf"),
							Directive: "limit_req_zone",
							Value:     fmt.Sprintf("$binary_remote_addr zone=%s_rl_grpcroute-rate-limit_rule0:20m rate=5r/s", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_route.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_grpcroute-rate-limit_rule0 burst=2", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_route.conf"),
							Directive: "limit_req_log_level",
							Value:     "warn",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_route.conf"),
							Directive: "limit_req_status",
							Value:     "466",
						},
					}
				}),
			)
		})
	})

	When("RateLimitPolicy has multiple targetRefs", func() {
		rlpFiles := []string{
			"rate-limit-policy/rlp-multiple-targetRefs.yaml",
		}

		var baseCoffeeURL string
		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(rlpFiles, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseCoffeeURL = helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(rlpFiles, namespace)).To(Succeed())
		})

		Specify("rateLimitPolicy is accepted", func() {
			rateLimitPolicy := "rlp-multiple-targets"
			rlpNsName := types.NamespacedName{Name: rateLimitPolicy, Namespace: namespace}
			Expect(waitForRateLimitPolicyToBeAccepted(rlpNsName)).To(
				Succeed(), fmt.Sprintf("%s was not accepted", rateLimitPolicy),
			)

			condType := string(conditions.RateLimitPolicyAffected)
			Expect(resourceManager.WaitForGRPCRoutePolicyAffected(
				types.NamespacedName{Name: "grpc-route", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace}, condType, timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return HTTP 200 initially and a custom error code once the rate limit is exceeded", func() {
				Eventually(
					func() error {
						return verifyRateLimitPolicyWorksAsExpected(baseCoffeeURL, address, "URI: /coffee", 429)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("are set properly for",
				func(getCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_internal_http.conf"),
							File:      "http.conf",
						},
						{
							Directive: "limit_req_zone",
							Value:     fmt.Sprintf("$binary_remote_addr zone=%s_rl_rlp-multiple-targets_rule0:10m rate=2r/m", namespace),
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_internal_http.conf"),
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_route.conf"),
							File:      "http.conf",
							Location:  "/coffee",
							Server:    "cafe.example.com",
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_route.conf"),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/helloworld.Greeter/SayHello",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_route.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_rlp-multiple-targets_rule0 burst=3", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_route.conf"),
							Directive: "limit_req_log_level",
							Value:     "info",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_route.conf"),
							Directive: "limit_req_status",
							Value:     "429",
						},
					}
				}),
			)
		})
	})

	When("RateLimitPolicy has multiple rules", func() {
		rlpFiles := []string{
			"rate-limit-policy/rlp-multiple-rules.yaml",
		}

		var baseCoffeeURL string
		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(rlpFiles, namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseCoffeeURL = helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(rlpFiles, namespace)).To(Succeed())
		})

		Specify("rateLimitPolicy is accepted", func() {
			rateLimitPolicy := "rlp-multiple-rules"
			rlpNsName := types.NamespacedName{Name: rateLimitPolicy, Namespace: namespace}
			Expect(waitForRateLimitPolicyToBeAccepted(rlpNsName)).To(
				Succeed(), fmt.Sprintf("%s was not accepted", rateLimitPolicy),
			)

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "coffee", Namespace: namespace},
				string(conditions.RateLimitPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return HTTP 200 initially and a custom error code once the rate limit is exceeded", func() {
				Eventually(
					func() error {
						return verifyRateLimitPolicyWorksAsExpected(baseCoffeeURL, address, "URI: /coffee", 466)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("are set properly for",
				func(getCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_internal_http.conf"),
							File:      "http.conf",
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
							File:      "http.conf",
							Location:  "/coffee",
							Server:    "cafe.example.com",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_internal_http.conf"),
							Directive: "limit_req_zone",
							Value:     fmt.Sprintf("$binary_remote_addr zone=%s_rl_rlp-multiple-rules_rule0:20m rate=1r/m", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_internal_http.conf"),
							Directive: "limit_req_zone",
							Value:     fmt.Sprintf("$binary_remote_addr zone=%s_rl_rlp-multiple-rules_rule1:10m rate=1r/m", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_rlp-multiple-rules_rule0 burst=2 nodelay", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_rlp-multiple-rules_rule1 burst=1 nodelay", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
							Directive: "limit_req_log_level",
							Value:     "warn",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
							Directive: "limit_req_status",
							Value:     "466",
						},
					}
				}),
			)
		})
	})

	// Regression test: RateLimitPolicy targeting an HTTPRoute attached via a ListenerSet
	// previously caused an NGINX crash ("zero size shared memory zone") because the
	// limit_req_zone directive was not generated at the HTTP level, while the limit_req
	// directive was still generated at the location level.
	When("RateLimitPolicy is applied to an HTTPRoute attached via ListenerSet", func() {
		lsFiles := []string{
			"rate-limit-policy/listenerset.yaml",
			"rate-limit-policy/listenerset-route.yaml",
			"rate-limit-policy/listenerset-rlp.yaml",
		}

		var baseCoffeeURL string

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(lsFiles, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

			port := helpers.BuildPortFwdPort(80, portFwdPort)
			baseCoffeeURL = helpers.BuildPortFwdURL("ls.example.com/coffee", port)
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(lsFiles, namespace)).To(Succeed())
		})

		Specify("rateLimitPolicy is accepted", func() {
			rlpNsName := types.NamespacedName{Name: "ls-route-rate-limit", Namespace: namespace}
			Expect(waitForRateLimitPolicyToBeAccepted(rlpNsName)).To(
				Succeed(), "ls-route-rate-limit was not accepted",
			)

			Expect(resourceManager.WaitForHTTPRoutePolicyAffected(
				types.NamespacedName{Name: "ls-coffee", Namespace: namespace},
				string(conditions.RateLimitPolicyAffected),
				timeoutConfig.GetStatusTimeout,
			)).To(Succeed())
		})

		Context("verify working traffic", func() {
			It("should return HTTP 200 initially and a custom error code once the rate limit is exceeded", func() {
				Eventually(
					func() error {
						return verifyRateLimitPolicyWorksAsExpected(baseCoffeeURL, address, "URI: /coffee", 466)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload

			BeforeAll(func() {
				conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
			})

			DescribeTable("are set properly for",
				func(getCfgs func() []framework.ExpectedNginxField) {
					for _, expCfg := range getCfgs() {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("listenerset route policy", func() []framework.ExpectedNginxField {
					return []framework.ExpectedNginxField{
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_ls-route-rate-limit_internal_http.conf"),
							File:      "http.conf",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_ls-route-rate-limit_internal_http.conf"),
							Directive: "limit_req_zone",
							Value: fmt.Sprintf(
								"$binary_remote_addr zone=%s_rl_ls-route-rate-limit_rule0:10m rate=1r/m",
								namespace,
							),
						},
						{
							Directive: "include",
							Value:     fmt.Sprintf("%s%s", filePrefix, "_ls-route-rate-limit_route.conf"),
							File:      "http.conf",
							Server:    "ls.example.com",
							Location:  "/coffee",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_ls-route-rate-limit_route.conf"),
							Directive: "limit_req",
							Value:     fmt.Sprintf("zone=%s_rl_ls-route-rate-limit_rule0 burst=3", namespace),
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_ls-route-rate-limit_route.conf"),
							Directive: "limit_req_log_level",
							Value:     "warn",
						},
						{
							File:      fmt.Sprintf("%s%s", filePrefix, "_ls-route-rate-limit_route.conf"),
							Directive: "limit_req_status",
							Value:     "466",
						},
					}
				}),
			)
		})
	})
})

// waitForRateLimitPolicyToBeAccepted waits until the RateLimitPolicy has one
// ancestor per targetRef and all show Accepted=True/Accepted.
func waitForRateLimitPolicyToBeAccepted(rlpNsName types.NamespacedName) error {
	return resourceManager.WaitForPolicyStatus(
		rlpNsName,
		&ngfAPI.RateLimitPolicy{},
		framework.PolicyAccepted,
		framework.AncestorsMatchTargetRefs,
	)
}

//nolint:unparam // responseBodyMessage is kept as a parameter for readability at call sites
func verifyRateLimitPolicyWorksAsExpected(appURL, address, responseBodyMessage string, expectedCode int) error {
	err := framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, appURL, address, responseBodyMessage)
	if err != nil {
		return fmt.Errorf("initial request did not succeed: %w", err)
	}

	tally := sendParallelRequests(appURL, address, responseBodyMessage, expectedCode)
	if err := tally.check(expectedCode); err != nil {
		return fmt.Errorf("rate limit not enforced as configured on %s: %w", appURL, err)
	}

	return nil
}

// rateLimitTally counts the outcomes of a burst of requests against a rate-limited route.
type rateLimitTally struct {
	unexpected map[string]int
	served     int
	rejected   int
	// timedOut counts requests NGINX held in the burst queue past the client timeout.
	timedOut int
}

func (t rateLimitTally) check(expectedCode int) error {
	// Every policy on /coffee allows at most 2r/m, so within the burst window NGINX can serve one
	// request from a refilled token plus one nodelay burst slot; an unlimited route serves them all.
	const maxServed = 2

	if len(t.unexpected) > 0 {
		return fmt.Errorf("unexpected responses %v (served %d, rejected %d)", t.unexpected, t.served, t.rejected)
	}
	if t.rejected == 0 {
		return fmt.Errorf("no request was rejected with %d (served %d, timed out %d)", expectedCode, t.served, t.timedOut)
	}
	if t.served > maxServed {
		return fmt.Errorf("served %d requests, expected at most %d (rejected %d)", t.served, maxServed, t.rejected)
	}

	return nil
}

// sendParallelRequests sends a fixed burst of parallel requests and tallies every outcome.
func sendParallelRequests(appURL, address, responseBodyMessage string, expectedCode int) rateLimitTally {
	const (
		parallelWorkers   = 5
		requestsPerWorker = 6
	)

	req := framework.Request{
		URL:     appURL,
		Address: address,
		Timeout: timeoutConfig.GetTimeout,
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		tally = rateLimitTally{unexpected: map[string]int{}}
	)

	worker := func() {
		defer wg.Done()

		for range requestsPerWorker {
			resp, err := framework.Get(req)

			mu.Lock()
			switch {
			case err != nil && errors.Is(err, context.DeadlineExceeded):
				tally.timedOut++
			case err != nil:
				tally.unexpected[err.Error()]++
			case resp.StatusCode == expectedCode:
				tally.rejected++
			case resp.StatusCode == http.StatusOK && strings.Contains(resp.Body, responseBodyMessage):
				tally.served++
			default:
				tally.unexpected[fmt.Sprintf("HTTP %d", resp.StatusCode)]++
			}
			mu.Unlock()
		}
	}

	wg.Add(parallelWorkers)
	for range parallelWorkers {
		go worker()
	}
	wg.Wait()

	return tally
}
