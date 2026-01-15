package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

var _ = Describe("RateLimitPolicy", Ordered, Label("functional", "rate-limit-policy"), func() {
	var (
		files = []string{
			"rate-limit-policy/apps.yaml",
			"rate-limit-policy/gateway.yaml",
			"rate-limit-policy/routes.yaml",
		}

		namespace    = "rate-limit-policy"
		nginxPodName string
		filePrefix   = "/etc/nginx/includes/RateLimitPolicy_rate-limit-policy"
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

	When("RateLimitPolicy is applied to Gateway, HTTPRoute and GRPCRoute", func() {
		rlpFiles := []string{
			"rate-limit-policy/gateway-rate-limit-policy.yaml",
			"rate-limit-policy/route-rate-limit-policies.yaml",
		}
		var baseCoffeeURL string

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(rlpFiles, namespace)).To(Succeed())

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			baseCoffeeURL = fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
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

				err := waitForRateLimitPolicyStatus(
					rlpNsName,
					1,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", rlp))
			}
		})

		Context("verify working traffic", func() {
			It("should return 200 response for HTTPRoute `coffee`", func() {
				Eventually(
					func() error {
						return expectHTTPCodeWithParallelRequests(baseCoffeeURL, address, 466)
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
				Entry("gateway policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
						File:      "http.conf",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
						Directive: "limit_req_zone",
						Value:     "$binary_remote_addr zone=rate-limit-policy_rl_gateway-rate-limit_rule0:10m rate=15r/m",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
						Directive: "limit_req",
						Value:     "zone=rate-limit-policy_rl_gateway-rate-limit_rule0 burst=3",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
						Directive: "limit_req_log_level",
						Value:     "info",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_gateway-rate-limit_gateway.conf"),
						Directive: "limit_req_status",
						Value:     "429",
					},
				}),
				Entry("httproute policy", []framework.ExpectedNginxField{
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
						Value:     "$binary_remote_addr zone=rate-limit-policy_rl_httproute-rate-limit_rule0:20m rate=1r/m",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_httproute-rate-limit_route.conf"),
						Directive: "limit_req",
						Value:     "zone=rate-limit-policy_rl_httproute-rate-limit_rule0 burst=5",
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
				}),
				Entry("grpcroute policy", []framework.ExpectedNginxField{
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
						Value:     "$binary_remote_addr zone=rate-limit-policy_rl_grpcroute-rate-limit_rule0:20m rate=5r/s",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_grpcroute-rate-limit_route.conf"),
						Directive: "limit_req",
						Value:     "zone=rate-limit-policy_rl_grpcroute-rate-limit_rule0 burst=2",
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

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			baseCoffeeURL = fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(rlpFiles, namespace)).To(Succeed())
		})

		Specify("rateLimitPolicy is accepted", func() {
			rateLimitPolicy := "rlp-multiple-targets"
			rlpNsName := types.NamespacedName{Name: rateLimitPolicy, Namespace: namespace}

			err := waitForRateLimitPolicyStatus(
				rlpNsName,
				2,
				metav1.ConditionTrue,
				gatewayv1.PolicyReasonAccepted,
			)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", rateLimitPolicy))
		})

		Context("verify working traffic", func() {
			It("should return 200 response for HTTPRoute `coffee`", func() {
				Eventually(
					func() error {
						return expectHTTPCodeWithParallelRequests(baseCoffeeURL, address, 429)
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
				Entry("route policy", []framework.ExpectedNginxField{
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-targets_internal_http.conf"),
						File:      "http.conf",
					},
					{
						Directive: "limit_req_zone",
						Value:     "$binary_remote_addr zone=rate-limit-policy_rl_rlp-multiple-targets_rule0:10m rate=2r/m",
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
						Value:     "zone=rate-limit-policy_rl_rlp-multiple-targets_rule0 burst=3",
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

			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}
			baseCoffeeURL = fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(rlpFiles, namespace)).To(Succeed())
		})

		Specify("rateLimitPolicy is accepted", func() {
			rateLimitPolicy := "rlp-multiple-rules"
			rlpNsName := types.NamespacedName{Name: rateLimitPolicy, Namespace: namespace}

			err := waitForRateLimitPolicyStatus(
				rlpNsName,
				1,
				metav1.ConditionTrue,
				gatewayv1.PolicyReasonAccepted,
			)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", rateLimitPolicy))
		})

		Context("verify working traffic", func() {
			It("should return 200 response for HTTPRoute `coffee`", func() {
				Eventually(
					func() error {
						return expectHTTPCodeWithParallelRequests(baseCoffeeURL, address, 466)
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
				Entry("route policy", []framework.ExpectedNginxField{
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
						Value:     "$binary_remote_addr zone=rate-limit-policy_rl_rlp-multiple-rules_rule0:20m rate=1r/m",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_internal_http.conf"),
						Directive: "limit_req_zone",
						Value:     "$binary_remote_addr zone=rate-limit-policy_rl_rlp-multiple-rules_rule1:10m rate=1r/m",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
						Directive: "limit_req",
						Value:     "zone=rate-limit-policy_rl_rlp-multiple-rules_rule0 burst=2 nodelay",
					},
					{
						File:      fmt.Sprintf("%s%s", filePrefix, "_rlp-multiple-rules_route.conf"),
						Directive: "limit_req",
						Value:     "zone=rate-limit-policy_rl_rlp-multiple-rules_rule1 burst=1 nodelay",
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
				}),
			)
		})
	})
})

// waitForRateLimitPolicyStatus waits until the RateLimitPolicy has the
// specified number of ancestors and the specified condition status and reason.
func waitForRateLimitPolicyStatus(
	rlpNsName types.NamespacedName,
	ancestorCount int,
	condStatus metav1.ConditionStatus,
	condReason gatewayv1.PolicyConditionReason,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout*2)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for RateLimitPolicy %q to have the condition %q/%q\n",
		rlpNsName,
		condStatus,
		condReason,
	)

	return wait.PollUntilContextCancel(
		ctx,
		2000*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var rlp ngfAPI.RateLimitPolicy
			var err error

			if err := resourceManager.Get(ctx, rlpNsName, &rlp); err != nil {
				return false, err
			}

			if len(rlp.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("RateLimitPolicy %q does not have an ancestor status yet\n", rlp)

				return false, nil
			}

			if len(rlp.Status.Ancestors) != ancestorCount {
				tooManyAncestorsErr := fmt.Errorf("policy has %d ancestors, expected %d", len(rlp.Status.Ancestors), ancestorCount)
				GinkgoWriter.Printf("ERROR: %v\n", tooManyAncestorsErr)

				return false, tooManyAncestorsErr
			}

			ancestors := rlp.Status.Ancestors

			for _, ancestor := range ancestors {
				tr, ok := findTargetRefForAncestor(ancestor, rlp.GetTargetRefs())
				if !ok {
					err = fmt.Errorf("could not find targetRef for ancestor %v", ancestor.AncestorRef)
					GinkgoWriter.Printf("ERROR: %v\n", err)

					return false, err
				}
				if err := ancestorMustEqualTargetRef(ancestor, tr, rlp.Namespace); err != nil {
					GinkgoWriter.Printf("ERROR: %v\n", err)

					return false, err
				}

				err = ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason)
			}
			return err == nil, err
		},
	)
}

// findTargetRefForAncestor finds the LocalPolicyTargetReference in
// list of targets that matches the given ancestor.
func findTargetRefForAncestor(
	ancestor gatewayv1.PolicyAncestorStatus,
	targets []gatewayv1.LocalPolicyTargetReference,
) (gatewayv1.LocalPolicyTargetReference, bool) {
	if ancestor.AncestorRef.Group == nil || ancestor.AncestorRef.Kind == nil {
		return gatewayv1.LocalPolicyTargetReference{}, false
	}
	for _, tr := range targets {
		if tr.Name == ancestor.AncestorRef.Name &&
			tr.Kind == *ancestor.AncestorRef.Kind &&
			tr.Group == *ancestor.AncestorRef.Group {
			return tr, true
		}
	}
	return gatewayv1.LocalPolicyTargetReference{}, false
}

// expectHTTPCodeWithParallelRequests sends parallel requests to the given URL
// and expects to receive the expected HTTP status code in at least one of them.
func expectHTTPCodeWithParallelRequests(appURL, address string, expectedCode int) error {
	const (
		parallelWorkers   = 5
		requestsPerWorker = 35
	)

	req := framework.Request{
		URL:     appURL,
		Address: address,
		Timeout: timeoutConfig.RequestTimeout,
	}

	foundCh := make(chan struct{}, 1)

	var (
		wg sync.WaitGroup
		// stop is a one-time latch.
		stop           atomic.Bool
		lastStatusCode atomic.Int64

		mu      sync.Mutex
		lastErr error
	)

	worker := func() {
		defer wg.Done()

		for range requestsPerWorker {
			if stop.Load() {
				return
			}

			resp, err := framework.Get(req)
			if err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
				continue
			}

			lastStatusCode.Store(int64(resp.StatusCode))
			if resp.StatusCode == expectedCode {
				// CompareAndSwap(false, true) ensures only the first goroutine that
				// sees the expected status signals success and tells all others to stop.
				if stop.CompareAndSwap(false, true) {
					select {
					case foundCh <- struct{}{}:
					default:
					}
				}
				return
			}
		}
	}

	wg.Add(parallelWorkers)
	for range parallelWorkers {
		go worker()
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-foundCh:
		<-doneCh
		return nil

	case <-doneCh:
		mu.Lock()
		err := lastErr
		mu.Unlock()

		ls := lastStatusCode.Load()
		if err != nil {
			return fmt.Errorf("did not observe HTTP StatusCode %d from %s (last status: %d)w last error: %w",
				expectedCode, appURL, ls, err)
		}
		return fmt.Errorf("did not observe HTTP StatusCode %d from %s (last status: %d)",
			expectedCode, appURL, ls)
	}
}
