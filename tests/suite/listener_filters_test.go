package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

const (
	listenerFilterAuthHeader1  = "X-Auth-1"
	listenerFilterAuthHeader2  = "X-Auth-2"
	listenerFilterListenerName = "http"
)

func expectGatewayListenerCondition(
	nsName types.NamespacedName,
	condType string,
	condStatus metav1.ConditionStatus,
	condReason string,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var gw gatewayv1.Gateway
		g.Expect(resourceManager.Get(context.Background(), nsName, &gw)).To(Succeed())

		var listener *gatewayv1.ListenerStatus
		for i := range gw.Status.Listeners {
			if gw.Status.Listeners[i].Name == gatewayv1.SectionName(listenerFilterListenerName) {
				listener = &gw.Status.Listeners[i]
				break
			}
		}
		g.Expect(listener).ToNot(BeNil(), "listener %q not found in gateway status", listenerFilterListenerName)

		cond := meta.FindStatusCondition(listener.Conditions, condType)
		g.Expect(cond).ToNot(BeNil(), "condition %q not yet present", condType)
		g.Expect(cond.Status).To(Equal(condStatus), "condition %q status", condType)
		g.Expect(cond.Reason).To(Equal(condReason), "condition %q reason", condType)
	}).
		WithTimeout(timeoutConfig.GetStatusTimeout).
		WithPolling(500*time.Millisecond).
		Should(Succeed(),
			"gateway %s listener %s did not reach condition %s=%s/%s",
			nsName, listenerFilterListenerName, condType, condStatus, condReason)
}

var _ = Describe(
	"ListenerFilters",
	Ordered,
	Label("functional", "listener-filters"),
	func() {
		var (
			files = []string{
				"listener-filters/headers-echo.yaml",
				"listener-filters/external-auth.yaml",
				"listener-filters/cafe-routes.yaml",
			}

			gatewayFile = "listener-filters/gateway.yaml"

			namespace string
			gwNsName  types.NamespacedName

			port         int
			nginxPodName string
		)

		expectResponseCode := func(url string, code int, headers map[string]string) error {
			resp, err := framework.Get(
				framework.Request{
					URL:     url,
					Address: address,
					Headers: headers,
					Timeout: timeoutConfig.RequestTimeout,
				},
			)
			if err != nil {
				return err
			}
			if resp.StatusCode != code {
				return fmt.Errorf("expected http status %d, got %d", code, resp.StatusCode)
			}
			return nil
		}

		coffeeURL := func() string {
			return helpers.BuildPortFwdURL("cafe.example.com/coffee", port)
		}

		headersURL := func() string {
			return helpers.BuildPortFwdURL("cafe.example.com/headers", port)
		}

		vipURL := func() string {
			return helpers.BuildPortFwdURL("cafe.example.com/vip", port)
		}

		// echoHeaders returns the lower-cased headers the echo backend received.
		echoHeaders := func(g Gomega, body string) map[string]string {
			var parsed struct {
				Headers map[string]string `json:"headers"`
			}
			g.Expect(json.Unmarshal([]byte(body), &parsed)).To(Succeed())

			got := make(map[string]string, len(parsed.Headers))
			for k, v := range parsed.Headers {
				got[strings.ToLower(k)] = v
			}
			return got
		}

		BeforeAll(func() {
			namespace = fmt.Sprintf("listener-filters-%d", GinkgoParallelProcess())
			gwNsName = types.NamespacedName{Namespace: namespace, Name: "gateway"}

			ns := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

			backendObjects, err := framework.GenerateBackendAppObjects([]string{"coffee"})
			Expect(err).ToNot(HaveOccurred())
			for _, obj := range backendObjects {
				obj.SetNamespace(namespace)
			}
			Expect(resourceManager.Apply(backendObjects)).To(Succeed())

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
			Expect(resourceManager.ApplyFromFiles([]string{gatewayFile}, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

			nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
				namespace,
				timeoutConfig.GetStatusTimeout,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(nginxPodNames).To(HaveLen(1))

			nginxPodName = nginxPodNames[0]

			setUpPortForward(nginxPodName, namespace)
			port = helpers.BuildPortFwdPort(80, portFwdPort)
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			cleanUpPortForward()

			Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
		})

		It("accepts and programs the listener with pre-routing filters", func() {
			expectGatewayListenerCondition(
				gwNsName, string(gatewayv1.ListenerConditionAccepted),
				metav1.ConditionTrue, string(gatewayv1.ListenerReasonAccepted),
			)
			expectGatewayListenerCondition(
				gwNsName, string(gatewayv1.ListenerConditionProgrammed),
				metav1.ConditionTrue, string(gatewayv1.ListenerReasonProgrammed),
			)
			expectGatewayListenerCondition(
				gwNsName, string(gatewayv1.ListenerConditionResolvedRefs),
				metav1.ConditionTrue, string(gatewayv1.ListenerReasonResolvedRefs),
			)
		})

		It("runs the filters in the configured order", func() {
			// With no credentials at all, whichever filter runs first decides the status code:
			// auth-1 rejects with 403 and auth-2 with 401. Carrying only the second filter's
			// credential still gets a 403, proving the chain runs in the exact configured order.
			Eventually(func() error {
				if err := expectResponseCode(coffeeURL(), 403, nil); err != nil {
					return err
				}
				return expectResponseCode(
					coffeeURL(), 403,
					map[string]string{listenerFilterAuthHeader2: "pass"},
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("rejects requests missing the second filter's credential", func() {
			Eventually(func() error {
				return expectResponseCode(
					coffeeURL(), 401,
					map[string]string{listenerFilterAuthHeader1: "pass"},
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("routes requests authorized by both filters", func() {
			Eventually(func() error {
				return framework.ExpectRequestToSucceed(
					timeoutConfig.RequestTimeout,
					coffeeURL(), address, "coffee",
					framework.WithRequestHeaders(map[string]string{
						listenerFilterAuthHeader1: "pass",
						listenerFilterAuthHeader2: "pass",
					}),
				)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("sets merged auth response headers on the proxied request and ignores spoofed values", func() {
			Eventually(func(g Gomega) {
				resp, err := framework.Get(
					framework.Request{
						URL:     headersURL(),
						Address: address,
						Headers: map[string]string{
							listenerFilterAuthHeader1: "pass",
							listenerFilterAuthHeader2: "pass",
							"X-User-Id":               "spoofed",
							"X-Shared":                "spoofed",
						},
						Timeout: timeoutConfig.RequestTimeout,
					},
				)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(resp.StatusCode).To(Equal(200))

				got := echoHeaders(g, resp.Body)

				// X-User-Id comes only from auth-1; X-Shared is set by both filters and the
				// last non-empty value (auth-2) must win. Client-supplied values must be replaced.
				g.Expect(got["x-user-id"]).To(Equal("alice"))
				g.Expect(got["x-shared"]).To(Equal("from-auth2"))
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("matches routes on the headers set by the filters", func() {
			// The /vip route only matches requests whose X-User-Id header is alice, a value that
			// only auth-1 sets. Matching therefore proves the filters ran before route selection.
			Eventually(func(g Gomega) {
				resp, err := framework.Get(
					framework.Request{
						URL:     vipURL(),
						Address: address,
						Headers: map[string]string{
							listenerFilterAuthHeader1: "pass",
							listenerFilterAuthHeader2: "pass",
						},
						Timeout: timeoutConfig.RequestTimeout,
					},
				)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(resp.StatusCode).To(Equal(200))
				g.Expect(echoHeaders(g, resp.Body)["x-user-id"]).To(Equal("alice"))
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("discards spoofed auth headers before route matching", func() {
			// A client-supplied X-User-Id must not be used for route matching; the filter replaces
			// it with the value from the authorization server.
			Eventually(func(g Gomega) {
				resp, err := framework.Get(
					framework.Request{
						URL:     vipURL(),
						Address: address,
						Headers: map[string]string{
							listenerFilterAuthHeader1: "pass",
							listenerFilterAuthHeader2: "pass",
							"X-User-Id":               "mallory",
						},
						Timeout: timeoutConfig.RequestTimeout,
					},
				)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(resp.StatusCode).To(Equal(200))
				g.Expect(echoHeaders(g, resp.Body)["x-user-id"]).To(Equal("alice"))
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		Context("when a filter backendRef cannot be resolved", func() {
			setFilterBackend := func(name string) {
				var gw gatewayv1.Gateway
				Expect(resourceManager.Get(context.Background(), gwNsName, &gw)).To(Succeed())
				gw.Spec.Listeners[0].Filters.Requests[0].ExternalAuth.BackendRef.Name = gatewayv1.ObjectName(name)
				Expect(resourceManager.Apply([]client.Object{&gw})).To(Succeed())
			}

			BeforeAll(func() {
				setFilterBackend("ext-auth-missing")
			})

			AfterAll(func() {
				setFilterBackend("ext-auth-1")
			})

			It("keeps the listener accepted but fails closed for all requests", func() {
				expectGatewayListenerCondition(
					gwNsName, string(gatewayv1.ListenerConditionAccepted),
					metav1.ConditionTrue, string(gatewayv1.ListenerReasonAccepted),
				)
				expectGatewayListenerCondition(
					gwNsName, string(gatewayv1.ListenerConditionResolvedRefs),
					metav1.ConditionFalse, "InvalidListenerFilterRef",
				)

				Eventually(func() error {
					return expectResponseCode(
						coffeeURL(), 500,
						map[string]string{
							listenerFilterAuthHeader1: "pass",
							listenerFilterAuthHeader2: "pass",
						},
					)
				}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})
	})
