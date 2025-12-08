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

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var invalidSPErrMsgs = "[spec.rules[0].sessionPersistence.type: Unsupported value: \"Header\": " +
	"supported values: \"Cookie\", spec.rules[0].sessionPersistence.idleTimeout: " +
	"Forbidden: IdleTimeout, spec.rules[0].sessionPersistence.absoluteTimeout: " +
	"Invalid value: \"10000h\": duration is too large for NGINX format (exceeds 9999h), " +
	"spec.rules[0].sessionPersistence: Invalid value: \"spec.rules[0].sessionPersistence\":" +
	" session persistence is ignored because there are errors in the configuration]"

var _ = Describe("SessionPersistence OSS", Ordered, Label("functional", "session-persistence-oss"), func() {
	var (
		files = []string{
			"session-persistence/cafe.yaml",
			"session-persistence/grpc-backends.yaml",
			"session-persistence/gateway.yaml",
			"session-persistence/routes-oss.yaml",
		}

		namespace   = "session-persistence-oss"
		gatewayName = "gateway"

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

	When("LoadBalancingMethod `ip-hash` is used for session affinity", func() {
		uspFiles := []string{
			"session-persistence/usp.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(uspFiles, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(uspFiles, namespace)).To(Succeed())
		})

		Specify("upstreamSettingsPolicies are accepted", func() {
			usPolicies := []string{
				"usp-ip-hash",
				"usp-ip-hash-grpc",
			}

			for _, name := range usPolicies {
				uspolicyNsName := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForUSPolicyStatus(
					uspolicyNsName,
					gatewayName,
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return 200 response for HTTPRoute `coffee` from the same backend", func() {
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseCoffeeURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")

				Eventually(
					func() error {
						return expectRequestToSucceedAndRespondFromTheSameBackend(baseCoffeeURL, address, "URI: /coffee", 11)
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
				Entry("HTTP upstream", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "session-persistence-oss_coffee_80",
						File:      "http.conf",
					},
					{
						Directive: "ip_hash",
						Upstream:  "session-persistence-oss_coffee_80",
						File:      "http.conf",
					},
				}),
				Entry("GRPC upstream", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "session-persistence-oss_grpc-backend_8080",
						File:      "http.conf",
					},
					{
						Directive: "ip_hash",
						Upstream:  "session-persistence-oss_grpc-backend_8080",
						File:      "http.conf",
					},
				}),
			)
		})
	})
})

var _ = Describe("SessionPersistence Plus", Ordered, Label("functional", "session-persistence-plus"), func() {
	var (
		files = []string{
			"session-persistence/cafe.yaml",
			"session-persistence/grpc-backends.yaml",
			"session-persistence/gateway.yaml",
			"session-persistence/routes-plus.yaml",
		}

		namespace = "session-persistence-plus"

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

	When("sticky cookies are used for session persistence in NGINX Plus", func() {
		var baseCoffeeURL, baseTeaURL string

		BeforeAll(func() {
			port := 80
			if portFwdPort != 0 {
				port = portFwdPort
			}

			baseCoffeeURL = fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
			baseTeaURL = fmt.Sprintf("http://cafe.example.com:%d%s", port, "/tea/location/flavors")
		})

		Context("verify working traffic", func() {
			It("should return 200 responses from the same backend for HTTPRoutes `coffee` and `tea`", func() {
				if !*plusEnabled {
					Skip("Skipping Session Persistence Plus tests on NGINX OSS deployment")
				}
				Eventually(
					func() error {
						return expectRequestToSucceedAndReuseCookie(baseCoffeeURL, address, "URI: /coffee", 11)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				Eventually(
					func() error {
						return expectRequestToSucceedAndReuseCookie(baseTeaURL, address, "URI: /tea/location/flavors", 11)
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
					if !*plusEnabled {
						Skip("Skipping Session Persistence Plus tests on NGINX OSS deployment")
					}
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("HTTP upstreams", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "session-persistence-plus_coffee_80_coffee_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "sticky",
						Value:     "cookie sp_coffee_session-persistence-plus_0 expires=48h path=/coffee",
						Upstream:  "session-persistence-plus_coffee_80_coffee_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "state",
						Value:     "/var/lib/nginx/state/session-persistence-plus_coffee_80.conf",
						Upstream:  "session-persistence-plus_coffee_80_coffee_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "upstream",
						Value:     "session-persistence-plus_tea_80_tea_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "sticky",
						Value:     "cookie tea-cookie",
						Upstream:  "session-persistence-plus_tea_80_tea_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "state",
						Value:     "/var/lib/nginx/state/session-persistence-plus_tea_80.conf",
						Upstream:  "session-persistence-plus_tea_80_tea_session-persistence-plus_0",
						File:      "http.conf",
					},
				}),
				Entry("GRPC upstream", []framework.ExpectedNginxField{
					{
						Directive: "upstream",
						Value:     "session-persistence-plus_grpc-backend_8080_grpc-route_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "sticky",
						Value:     "cookie sp_grpc-route_session-persistence-plus_0 expires=24h",
						Upstream:  "session-persistence-plus_grpc-backend_8080_grpc-route_session-persistence-plus_0",
						File:      "http.conf",
					},
					{
						Directive: "state",
						Upstream:  "session-persistence-plus_grpc-backend_8080_grpc-route_session-persistence-plus_0",
						Value:     "/var/lib/nginx/state/session-persistence-plus_grpc-backend_8080.conf",
						File:      "http.conf",
					},
				}),
			)
		})
	})

	When("Routes have an invalid session persistence configuration", func() {
		BeforeAll(func() {
			routeFile := "session-persistence/route-invalid-sp-config.yaml"
			Expect(resourceManager.ApplyFromFiles([]string{routeFile}, namespace)).To(Succeed())
		})

		It("HTTPRoute reports the errors in its status", func() {
			if !*plusEnabled {
				Skip("Skipping Session Persistence Plus tests on NGINX OSS deployment")
			}
			routeNsName := types.NamespacedName{Name: "route-invalid-sp", Namespace: namespace}
			err := waitForHTTPRouteToHaveErrorMessage(routeNsName)
			Expect(err).ToNot(HaveOccurred(), "expected route to report invalid session persistence configuration")
		})

		It("GRPCRoute reports the errors in its status", func() {
			if !*plusEnabled {
				Skip("Skipping Session Persistence Plus tests on NGINX OSS deployment")
			}
			routeNsName := types.NamespacedName{Name: "grpc-route-invalid-sp", Namespace: namespace}
			err := waitForGRPCRouteToHaveErrorMessage(routeNsName)
			Expect(err).ToNot(HaveOccurred(), "expected route to report invalid session persistence configuration")
		})
	})
})

func waitForHTTPRouteToHaveErrorMessage(routeNsName types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for %q to have the condition Accepted/True/Accepted with the right error message\n",
		routeNsName,
	)

	return waitForHTTPRouteErrorMessage(
		ctx,
		routeNsName,
		gatewayv1.RouteConditionAccepted,
		metav1.ConditionTrue,
		invalidSPErrMsgs,
	)
}

func waitForHTTPRouteErrorMessage(
	ctx context.Context,
	routeNsName types.NamespacedName,
	conditionType gatewayv1.RouteConditionType,
	condStatus metav1.ConditionStatus,
	expectedReasonSubstring string,
) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var route gatewayv1.HTTPRoute
			if err := resourceManager.Get(ctx, routeNsName, &route); err != nil {
				return false, err
			}

			return checkRouteStatus(route.Status.RouteStatus, conditionType, condStatus, expectedReasonSubstring)
		},
	)
}

func waitForGRPCRouteToHaveErrorMessage(routeNsName types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for %q to have the condition Accepted/True/Accepted with the right error message\n",
		routeNsName,
	)

	return waitForGRPCRouteErrorMessage(
		ctx,
		routeNsName,
		gatewayv1.RouteConditionAccepted,
		metav1.ConditionTrue,
		invalidSPErrMsgs,
	)
}

func waitForGRPCRouteErrorMessage(
	ctx context.Context,
	routeNsName types.NamespacedName,
	conditionType gatewayv1.RouteConditionType,
	condStatus metav1.ConditionStatus,
	expectedReasonSubstring string,
) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var route gatewayv1.GRPCRoute
			if err := resourceManager.Get(ctx, routeNsName, &route); err != nil {
				return false, err
			}

			return checkRouteStatus(route.Status.RouteStatus, conditionType, condStatus, expectedReasonSubstring)
		},
	)
}

func checkRouteStatus(
	rs gatewayv1.RouteStatus,
	conditionType gatewayv1.RouteConditionType,
	condStatus metav1.ConditionStatus,
	expectedReasonSubstring string,
) (bool, error) {
	var err error
	if len(rs.Parents) == 0 {
		GinkgoWriter.Printf("route does not have a status yet\n")
		return false, nil
	}
	if len(rs.Parents) != 1 {
		err := fmt.Errorf("route has %d parents, expected 1", len(rs.Parents))
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return false, err
	}

	parent := rs.Parents[0]
	if parent.Conditions == nil {
		err := fmt.Errorf("route has no conditions in its status")
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return false, err
	}
	if len(parent.Conditions) != 2 {
		err := fmt.Errorf("expected route to have only two conditions, instead has %d", len(parent.Conditions))
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return false, err
	}

	cond := parent.Conditions[1]
	if cond.Type != string(conditionType) &&
		cond.Status != condStatus &&
		!strings.Contains(cond.Reason, expectedReasonSubstring) {
		err := fmt.Errorf(
			"expected route condition to be Type=%s, Status=%s, "+
				"Reason contains=%s; instead got Type=%s, Status=%s, Reason=%s",
			conditionType, condStatus, expectedReasonSubstring, cond.Type, cond.Status, cond.Reason,
		)
		GinkgoWriter.Printf("ERROR: %v\n", err)
		return false, err
	}

	return err == nil, nil
}

func expectRequestToSucceedAndRespondFromTheSameBackend(
	appURL,
	address,
	responseBodyMessage string,
	totalRequests int,
) error {
	var firstServerName string

	for i := range totalRequests {
		request := framework.Request{
			URL:     appURL,
			Address: address,
			Timeout: timeoutConfig.RequestTimeout,
		}
		resp, err := framework.Get(request)
		if err != nil {
			return fmt.Errorf("request %d to %s failed: %w", i+1, appURL, err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("request %d: http status was not 200, got %d", i+1, resp.StatusCode)
		}

		if !strings.Contains(resp.Body, responseBodyMessage) {
			return fmt.Errorf("request %d: expected response body to contain %q, got: %s", i+1, responseBodyMessage, resp.Body)
		}

		serverName, err := extractServerName(resp.Body)
		if err != nil {
			return fmt.Errorf("request %d: failed to extract server name: %w; body: %s", i+1, err, resp.Body)
		}

		if i == 0 {
			firstServerName = serverName
			continue
		}

		// subsequent replies must come from the same backend.
		if serverName != firstServerName {
			return fmt.Errorf(
				"request %d: expected server name %q, got %q resulting in `ip-hash` stickiness failure",
				i+1, firstServerName, serverName,
			)
		}
	}

	return nil
}

func expectRequestToSucceedAndReuseCookie(
	appURL,
	address,
	responseBodyMessage string,
	totalRequests int,
) error {
	var firstServerName string
	cookieAttr := make(map[string]string, 0)

	for i := range totalRequests {
		headers := make(map[string]string, 0)

		// send cookie token after first response
		if i > 0 {
			if cookieAttr == nil {
				return fmt.Errorf("request %d: cookie attributes are nil after first response", i+1)
			}

			headers["Cookie"] = fmt.Sprintf("%s=%s", cookieAttr["name"], cookieAttr["value"])
		}

		request := framework.Request{
			URL:     appURL,
			Address: address,
			Timeout: timeoutConfig.RequestTimeout,
			Headers: headers,
		}

		resp, err := framework.Get(request)
		if err != nil {
			return fmt.Errorf("request %d to %s failed: %w", i+1, appURL, err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("request %d: http status was not 200, got %d", i+1, resp.StatusCode)
		}

		if !strings.Contains(resp.Body, responseBodyMessage) {
			return fmt.Errorf(
				"request %d: expected response body to contain %q, got: %s",
				i+1, responseBodyMessage, resp.Body,
			)
		}

		serverName, err := extractServerName(resp.Body)
		if err != nil {
			return fmt.Errorf(
				"request %d: failed to extract server name: %w; body: %s",
				i+1, err, resp.Body,
			)
		}

		// get the cookie token from the first response
		if i == 0 {
			cookieAttr, err = extractCookieInformationFromResponseHeaders(resp.Headers)
			if err != nil {
				return fmt.Errorf(
					"request %d: failed to extract cookie from response headers: %w; body: %s",
					i+1, err, resp.Body,
				)
			}

			firstServerName = serverName
			continue
		}

		if serverName != firstServerName {
			return fmt.Errorf(
				"request %d: expected server name %q, got %q (session persistence failed)",
				i+1, firstServerName, serverName,
			)
		}
	}

	return nil
}

func extractCookieInformationFromResponseHeaders(h http.Header) (map[string]string, error) {
	values := h.Values("Set-Cookie")
	if len(values) == 0 {
		return nil, fmt.Errorf("no Set-Cookie header found in response")
	}

	raw := strings.TrimSpace(values[0])
	if raw == "" {
		return nil, fmt.Errorf("empty Set-Cookie header")
	}

	parts := strings.Split(raw, ";")
	if len(parts) == 0 {
		return nil, fmt.Errorf("malformed Set-Cookie header: %q", raw)
	}

	// first part is cookie-name=value
	pair := strings.TrimSpace(parts[0])
	nv := strings.SplitN(pair, "=", 2)
	if len(nv) != 2 {
		return nil, fmt.Errorf("malformed Set-Cookie header (no name=value): %q", raw)
	}

	name := strings.TrimSpace(nv[0])
	value := strings.TrimSpace(nv[1])
	if name == "" || value == "" {
		return nil, fmt.Errorf("malformed Set-Cookie header (empty name or value): %q", raw)
	}

	result := map[string]string{
		"name":  name,
		"value": value,
	}

	return result, nil
}
