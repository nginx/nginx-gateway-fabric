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

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var invalidSPErrMsgs = "[spec.rules[0].sessionPersistence.type: Unsupported value: \"Header\": " +
	"supported values: \"Cookie\", spec.rules[0].sessionPersistence.absoluteTimeout: " +
	"Invalid value: \"10000h\": duration is too large for NGINX format (exceeds 9999h), " +
	"spec.rules[0].sessionPersistence: Invalid value: \"spec.rules[0].sessionPersistence\":" +
	" session persistence is ignored because there are errors in the configuration]"

func expectSessionPersistenceCookieTraffic(baseCoffeeURL, baseTeaURL string, coffeeRequestCount int) {
	Eventually(
		func() error {
			return expectRequestToSucceedAndReuseCookie(baseCoffeeURL, address, "URI: /coffee", coffeeRequestCount)
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
}

var _ = Describe(
	"SessionPersistence",
	Ordered,
	Label("functional", "session-persistence-oss", "session-persistence-plus"),
	func() {
		var (
			files = []string{
				"session-persistence/cafe.yaml",
				"session-persistence/grpc-backends.yaml",
				"session-persistence/gateway.yaml",
				"session-persistence/routes.yaml",
			}

			namespace = "session-persistence"

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

		When("sticky cookies are used for session persistence", func() {
			var baseCoffeeURL, baseTeaURL string
			httpFields := []framework.ExpectedNginxField{
				{
					Directive: "upstream",
					Value:     namespace + "_coffee_80_coffee_" + namespace + "_0",
					File:      "http.conf",
				},
				{
					Directive: "sticky",
					Value:     "cookie sp_coffee_" + namespace + "_0 expires=48h path=/coffee",
					Upstream:  namespace + "_coffee_80_coffee_" + namespace + "_0",
					File:      "http.conf",
				},
				{
					Directive: "upstream",
					Value:     namespace + "_tea_80_tea_" + namespace + "_0",
					File:      "http.conf",
				},
				{
					Directive: "sticky",
					Value:     "cookie tea-cookie",
					Upstream:  namespace + "_tea_80_tea_" + namespace + "_0",
					File:      "http.conf",
				},
			}

			grpcFields := []framework.ExpectedNginxField{
				{
					Directive: "upstream",
					Value:     namespace + "_grpc-backend_8080_grpc-route_" + namespace + "_0",
					File:      "http.conf",
				},
				{
					Directive: "sticky",
					Value:     "cookie sp_grpc-route_" + namespace + "_0 expires=24h",
					Upstream:  namespace + "_grpc-backend_8080_grpc-route_" + namespace + "_0",
					File:      "http.conf",
				},
			}

			if *plusEnabled {
				httpFields = append(httpFields,
					framework.ExpectedNginxField{
						Directive: "state",
						Value:     "/var/lib/nginx/state/" + namespace + "_coffee_80.conf",
						Upstream:  namespace + "_coffee_80_coffee_" + namespace + "_0",
						File:      "http.conf",
					},
					framework.ExpectedNginxField{
						Directive: "state",
						Value:     "/var/lib/nginx/state/" + namespace + "_tea_80.conf",
						Upstream:  namespace + "_tea_80_tea_" + namespace + "_0",
						File:      "http.conf",
					},
				)

				grpcFields = append(grpcFields,
					framework.ExpectedNginxField{
						Directive: "state",
						Upstream:  namespace + "_grpc-backend_8080_grpc-route_" + namespace + "_0",
						Value:     "/var/lib/nginx/state/" + namespace + "_grpc-backend_8080.conf",
						File:      "http.conf",
					},
				)
			}

			BeforeAll(func() {
				port := helpers.BuildPortFwdPort(80, portFwdPort)
				baseCoffeeURL = helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
				baseTeaURL = helpers.BuildPortFwdURL("cafe.example.com/tea/location/flavors", port)
			})

			Context("verify working traffic", func() {
				It("should return 200 responses from the same backend for HTTPRoutes `coffee` and `tea`", func() {
					expectSessionPersistenceCookieTraffic(baseCoffeeURL, baseTeaURL, 10)
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
					Entry("HTTP upstreams", httpFields),
					Entry("GRPC upstream", grpcFields),
				)
			})
		})

		When("Routes have an invalid session persistence configuration", func() {
			BeforeAll(func() {
				routeFile := "session-persistence/route-invalid-sp-config.yaml"
				Expect(resourceManager.ApplyFromFiles([]string{routeFile}, namespace)).To(Succeed())
			})

			It("updates the HTTPRoute status with all relevant validation errors", func() {
				routeNsName := types.NamespacedName{Name: "route-invalid-sp", Namespace: namespace}
				err := waitForHTTPRouteToHaveErrorMessage(routeNsName)
				Expect(err).ToNot(HaveOccurred(), "expected route to report invalid session persistence configuration")
			})

			It("updates the GRPCRoute status with all relevant validation errors", func() {
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

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var route gatewayv1.HTTPRoute
			if err := resourceManager.Get(ctx, routeNsName, &route); err != nil {
				return false, err
			}

			return checkRouteStatus(
				route.Status.RouteStatus,
				gatewayv1.RouteConditionAccepted,
				metav1.ConditionTrue,
				invalidSPErrMsgs,
			)
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

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var route gatewayv1.GRPCRoute
			if err := resourceManager.Get(ctx, routeNsName, &route); err != nil {
				return false, err
			}

			return checkRouteStatus(
				route.Status.RouteStatus,
				gatewayv1.RouteConditionAccepted,
				metav1.ConditionTrue,
				invalidSPErrMsgs)
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
