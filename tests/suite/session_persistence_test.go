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

func expectSessionPersistenceCookieTraffic(
	namespace string,
	baseCoffeeURL string,
	baseTeaURL string,
	coffeeRequestCount int,
) {
	coffeeCookie := expectedStickyCookie{
		name:     "sp_coffee_" + namespace + "_0",
		path:     "/coffee",
		lifetime: 48 * time.Hour,
	}
	Eventually(
		func() error {
			return expectRequestToSucceedAndReuseCookie(
				baseCoffeeURL,
				address,
				"URI: /coffee",
				coffeeRequestCount,
				coffeeCookie,
			)
		}).
		WithTimeout(timeoutConfig.RequestTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	// lifetimeType Session: no expiry. The route uses a regex match and does not set an explicit
	// cookie path, so the cookie path defaults to "/".
	teaCookie := expectedStickyCookie{name: "tea-cookie", path: "/"}
	Eventually(
		func() error {
			return expectRequestToSucceedAndReuseCookie(
				baseTeaURL,
				address,
				"URI: /tea/location/flavors",
				11,
				teaCookie,
			)
		}).
		WithTimeout(timeoutConfig.RequestTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}

var _ = Describe(
	"SessionPersistence",
	Ordered,
	Label("functional", "session-persistence"),
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
					Value:     "cookie tea-cookie path=/",
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
					Value:     "cookie sp_grpc-route_" + namespace + "_0 expires=24h path=/",
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
					expectSessionPersistenceCookieTraffic(namespace, baseCoffeeURL, baseTeaURL, 10)
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

	for _, cond := range parent.Conditions {
		if cond.Type != string(conditionType) {
			continue
		}
		if cond.Status != condStatus || !strings.Contains(cond.Message, expectedReasonSubstring) {
			err := fmt.Errorf(
				"expected route condition to be Type=%s, Status=%s, "+
					"Message contains=%s; instead got Type=%s, Status=%s, Message=%s",
				conditionType, condStatus, expectedReasonSubstring, cond.Type, cond.Status, cond.Message,
			)
			GinkgoWriter.Printf("ERROR: %v\n", err)
			return false, err
		}
		return true, nil
	}

	err = fmt.Errorf("route condition %s not found", conditionType)
	GinkgoWriter.Printf("ERROR: %v\n", err)
	return false, err
}

// expectedStickyCookie is the Set-Cookie NGINX should send for a route's session persistence.
type expectedStickyCookie struct {
	name string
	path string
	// lifetime is the configured absoluteTimeout; zero means a session cookie with no expiry.
	lifetime time.Duration
}

// expectRequestToSucceedAndReuseCookie checks that the route spreads cookie-less requests over
// several backends, sets the expected sticky cookie, and keeps every request carrying that
// cookie on the same backend.
func expectRequestToSucceedAndReuseCookie(
	appURL string,
	address string,
	responseBodyMessage string,
	totalRequests int,
	want expectedStickyCookie,
) error {
	if err := expectRequestsWithoutCookieToSpread(appURL, address, responseBodyMessage); err != nil {
		return err
	}

	var (
		firstServerName string
		cookie          *http.Cookie
	)

	for i := range totalRequests {
		headers := map[string]string{}
		if cookie != nil {
			headers["Cookie"] = cookie.Name + "=" + cookie.Value
		}

		serverName, resp, err := getServerName(appURL, address, responseBodyMessage, headers)
		if err != nil {
			return fmt.Errorf("request %d: %w", i+1, err)
		}

		if i == 0 {
			cookie, err = stickyCookieFromResponse(resp.Headers, want)
			if err != nil {
				return fmt.Errorf("request %d: %w", i+1, err)
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

// expectRequestsWithoutCookieToSpread proves the cookie, not chance, pins requests: without it,
// requests must reach more than one of the route's backends.
func expectRequestsWithoutCookieToSpread(appURL, address, responseBodyMessage string) error {
	const maxRequests = 15

	seen := map[string]struct{}{}
	for range maxRequests {
		serverName, _, err := getServerName(appURL, address, responseBodyMessage, nil)
		if err != nil {
			return fmt.Errorf("request without cookie: %w", err)
		}
		seen[serverName] = struct{}{}
		if len(seen) > 1 {
			return nil
		}
	}

	return fmt.Errorf("%d requests without a cookie all reached the same backend; cannot prove stickiness", maxRequests)
}

func getServerName(
	appURL string,
	address string,
	responseBodyMessage string,
	headers map[string]string,
) (string, framework.Response, error) {
	resp, err := framework.Get(framework.Request{
		URL:     appURL,
		Address: address,
		Timeout: timeoutConfig.RequestTimeout,
		Headers: headers,
	})
	if err != nil {
		return "", resp, fmt.Errorf("request to %s failed: %w", appURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", resp, fmt.Errorf("http status was not 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, responseBodyMessage) {
		return "", resp, fmt.Errorf("expected response body to contain %q, got: %s", responseBodyMessage, resp.Body)
	}

	serverName, err := extractServerName(resp.Body)
	if err != nil {
		return "", resp, fmt.Errorf("failed to extract server name: %w; body: %s", err, resp.Body)
	}

	return serverName, resp, nil
}

// stickyCookieFromResponse returns the sticky cookie after checking its name, path and lifetime.
func stickyCookieFromResponse(h http.Header, want expectedStickyCookie) (*http.Cookie, error) {
	for _, raw := range h.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(raw)
		if err != nil {
			return nil, fmt.Errorf("malformed Set-Cookie header %q: %w", raw, err)
		}
		if cookie.Name != want.name {
			continue
		}

		if cookie.Value == "" {
			return nil, fmt.Errorf("cookie %q has an empty value", cookie.Name)
		}
		if cookie.Path != want.path {
			return nil, fmt.Errorf("cookie %q: expected Path %q, got %q", cookie.Name, want.path, cookie.Path)
		}

		expiry, hasExpiry, err := cookieExpiry(raw, cookie)
		if err != nil {
			return nil, fmt.Errorf("cookie %q: %w", cookie.Name, err)
		}

		if want.lifetime == 0 {
			if hasExpiry {
				return nil, fmt.Errorf("cookie %q: expected a session cookie, got %q", cookie.Name, raw)
			}
			return cookie, nil
		}

		if !hasExpiry {
			return nil, fmt.Errorf("cookie %q: expected an expiry about %s from now, got %q", cookie.Name, want.lifetime, raw)
		}

		// Allow for clock drift between the test runner and the NGINX pod.
		const tolerance = 5 * time.Minute
		wantExpiry := time.Now().Add(want.lifetime)
		if expiry.Before(wantExpiry.Add(-tolerance)) || expiry.After(wantExpiry.Add(tolerance)) {
			return nil, fmt.Errorf(
				"cookie %q: expected expiry about %s from now, got %s",
				cookie.Name, want.lifetime, expiry,
			)
		}

		return cookie, nil
	}

	return nil, fmt.Errorf("no Set-Cookie named %q in %q", want.name, h.Values("Set-Cookie"))
}

// cookieExpiryLayouts are the date formats tried when parsing a Set-Cookie Expires attribute.
// NGINX writes Expires with a two-digit year (for example "Fri, 23-Oct-26 10:26:23 GMT"), which RFC 6265
// allows but http.ParseSetCookie leaves as a zero time, so the raw attribute is parsed here.
var cookieExpiryLayouts = []string{
	time.RFC1123,
	"Mon, 02-Jan-2006 15:04:05 MST",
	"Mon, 02-Jan-06 15:04:05 MST",
}

// cookieExpiry returns when the cookie expires and whether it sets an expiry at all.
func cookieExpiry(raw string, cookie *http.Cookie) (time.Time, bool, error) {
	if cookie.MaxAge != 0 {
		// A negative MaxAge means the header asked for immediate expiry.
		return time.Now().Add(time.Duration(max(cookie.MaxAge, 0)) * time.Second), true, nil
	}
	if !cookie.Expires.IsZero() {
		return cookie.Expires, true, nil
	}

	for _, attr := range strings.Split(raw, ";")[1:] {
		name, value, _ := strings.Cut(strings.TrimSpace(attr), "=")
		if !strings.EqualFold(name, "expires") {
			continue
		}
		for _, layout := range cookieExpiryLayouts {
			if t, err := time.Parse(layout, value); err == nil {
				return t, true, nil
			}
		}
		return time.Time{}, true, fmt.Errorf("unparseable Expires attribute %q", value)
	}

	return time.Time{}, false, nil
}
