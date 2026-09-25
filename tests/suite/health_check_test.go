package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("HealthCheck", Ordered, Label("functional", "health-check"), func() {
	var (
		files = []string{
			"health-check/cafe.yaml",
			"health-check/gateway.yaml",
			"health-check/routes.yaml",
		}

		namespace    = "healthcheck"
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

	Context("verify working traffic", func() {
		It("should return a 200 response", func() {
			port := helpers.BuildPortFwdPort(80, portFwdPort)
			coffeeURL := helpers.BuildPortFwdURL("cafe.example.com/coffee", port)

			Eventually(
				func() error {
					return framework.ExpectRequestToSucceed(
						timeoutConfig.RequestTimeout,
						coffeeURL,
						address,
						"URI: /coffee",
					)
				}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("passive health checks on Nginx OSS and Nginx Plus", func() {
		When("a passive health check configuration is applied to Nginx OSS or Nginx Plus", func() {
			passivePolicy := []string{"health-check/passive-health-check.yaml"}

			// The soda endpoint contains a deliberately unhealthy endpoint to verify passive
			// health checks stop routing traffic.
			sodaFiles := []string{"health-check/soda-route.yaml"}
			passiveSodaPolicy := []string{"health-check/passive-health-check-soda.yaml"}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(passivePolicy, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(passivePolicy, namespace)).To(Succeed())
				Expect(resourceManager.DeleteFromFiles(passiveSodaPolicy, namespace)).To(Succeed())
				Expect(resourceManager.DeleteFromFiles(sodaFiles, namespace)).To(Succeed())
			})

			It("generates the correct fields in Nginx OSS", func() {
				if *plusEnabled {
					Skip("Skipping NGINX OSS Health Check test on NGINX Plus")
				}

				expectedOSSDirectives := []framework.ExpectedNginxField{
					{
						Directive:             "server",
						Value:                 "max_fails=3 fail_timeout=5s",
						Upstream:              "healthcheck_coffee_80",
						File:                  "http.conf",
						ValueSubstringAllowed: true,
					},
				}

				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					for _, directive := range expectedOSSDirectives {
						if err := framework.ValidateNginxFieldExists(conf, directive); err != nil {
							return err
						}
					}
					return nil
				}).WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("generates the correct fields in Nginx Plus", func() {
				if !*plusEnabled {
					Skip("Skipping NGINX Plus Health Check test on NGINX OSS")
				}

				Eventually(func() error {
					state, err := resourceManager.GetNginxStateFile(
						context.Background(),
						nginxPodName,
						namespace,
						"healthcheck_coffee_80",
					)
					if err != nil {
						return err
					}

					if !regexp.MustCompile(`^server \S+ max_fails=3 fail_timeout=5s;$`).MatchString(strings.TrimSpace(state)) {
						return fmt.Errorf("unexpected NGINX state file contents: %q", state)
					}
					return nil
				}).WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("marks an endpoint as unavailable after maxFails failures within failTimeout", func() {
				// The healthy "soda" endpoint returns 200. The unhealthy "soda-bad" endpoint advertises
				// port 8080 but listens on port 9090.
				Expect(resourceManager.ApplyFromFiles(sodaFiles, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

				port := helpers.BuildPortFwdPort(80, portFwdPort)
				sodaURL := helpers.BuildPortFwdURL("cafe.example.com/soda", port)

				Expect(resourceManager.ApplyFromFiles(passiveSodaPolicy, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

				// All requests should be 200, because traffic to unhealthy route will be sent to
				// the healthy route instead.
				for range requestAttempts {
					Eventually(func() (int, error) {
						resp, err := framework.Get(framework.Request{
							URL:     sodaURL,
							Address: address,
							Timeout: timeoutConfig.RequestTimeout,
						})
						if err != nil {
							return 0, err
						}
						return resp.StatusCode, err
					}).Should(Equal(http.StatusOK))
				}

				Eventually(func() error {
					logs, err := resourceManager.GetPodLogs(
						namespace,
						nginxPodName,
						&core.PodLogOptions{Container: "nginx"},
					)
					if err != nil {
						return err
					}

					if !strings.Contains(logs, "upstream server temporarily disabled") {
						return fmt.Errorf("passive health check did not disable the failed endpoint")
					}
					return nil
				}).WithTimeout(failTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})
	})

	// APPLYING ACTIVE HEALTH CHECKS
	Context("active health checks on Nginx OSS and Nginx Plus", func() {
		activePolicy := []string{
			"health-check/active-health-check.yaml",
		}

		When("an active health check configuration is applied on Nginx OSS", func() {
			BeforeAll(func() {
				if *plusEnabled {
					Skip("Skipping NGINX OSS Health Check test on NGINX Plus")
				}

				Expect(resourceManager.ApplyFromFiles(activePolicy, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				if *plusEnabled {
					return
				}

				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(activePolicy, namespace)).To(Succeed())
			})

			It("rejects the policy", func() {
				policyName := types.NamespacedName{
					Name:      "example-active-hc",
					Namespace: namespace,
				}
				Expect(waitForUSPolicyStatus(
					policyName,
					"gateway",
					metav1.ConditionFalse,
					gatewayv1.PolicyReasonInvalid,
				)).To(Succeed())

				Eventually(func() (string, error) {
					var policy ngfAPI.UpstreamSettingsPolicy
					if err := resourceManager.Get(context.Background(), policyName, &policy); err != nil {
						return "", err
					}

					for _, ancestor := range policy.Status.Ancestors {
						for _, condition := range ancestor.Conditions {
							if condition.Type == string(gatewayv1.PolicyConditionAccepted) {
								return condition.Message, nil
							}
						}
					}

					return "", nil
				}).WithTimeout(timeoutConfig.GetStatusTimeout).Should(Equal(
					"spec.healthCheck.active: Forbidden: active health checks are only supported with NGINX Plus",
				))
			})

			It("does not affect the upstream", func() {
				expectedDirectives := []framework.ExpectedNginxField{
					{
						Directive:             "health_check",
						Value:                 "interval=10s jitter=3s",
						File:                  "http.conf",
						ValueSubstringAllowed: true,
					},
					{
						Directive:             "health_check",
						Value:                 "interval=10s jitter=3s",
						File:                  "http.conf",
						Block:                 "server",
						Location:              "@hc-healthcheck_coffee_80",
						ValueSubstringAllowed: true,
					},
				}

				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					for _, directive := range expectedDirectives {
						if err := framework.ValidateNginxFieldExists(conf, directive); err == nil {
							return fmt.Errorf("active health check configuration detected in OSS")
						}
					}
					return nil
				}).WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		When("an active health check configuration is applied on Nginx Plus", func() {
			BeforeAll(func() {
				if !*plusEnabled {
					Skip("Skipping NGINX Plus Health Check test on NGINX OSS")
				}

				Expect(resourceManager.ApplyFromFiles(activePolicy, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				if !*plusEnabled {
					return
				}

				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(activePolicy, namespace)).To(Succeed())
			})

			It("generates the correct fields in Nginx Plus", func() {
				expectedPlusDirectives := []framework.ExpectedNginxField{
					{
						Directive:             "health_check",
						Value:                 "interval=10s jitter=3s",
						File:                  "http.conf",
						Block:                 "server",
						Location:              "@hc-healthcheck_coffee_80",
						ValueSubstringAllowed: true,
					},
				}

				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					for _, directive := range expectedPlusDirectives {
						if err := framework.ValidateNginxFieldExists(conf, directive); err != nil {
							return err
						}
					}
					return nil
				}).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		When("an active health check targets a HTTPS backend with BackendTLSPolicy", func() {
			backendFiles := []string{"health-check/https-backend.yaml"}
			policyFiles := []string{"health-check/https-backend-policies.yaml"}
			httpsHostname := "https-backend.healthcheck.svc.cluster.local"

			BeforeAll(func() {
				if !*plusEnabled {
					Skip("Skipping NGINX Plus Health Check test on NGINX OSS")
				}

				ca, err := framework.GenerateSelfSignedCACert("Health Check Backend CA")
				Expect(err).ToNot(HaveOccurred())
				serverCert, err := framework.GenerateSignedServerCert(ca, httpsHostname, []string{httpsHostname})
				Expect(err).ToNot(HaveOccurred())

				tlsSecret := &core.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "https-backend-tls", Namespace: namespace},
					Type:       core.SecretTypeTLS,
					Data: map[string][]byte{
						core.TLSCertKey:       serverCert.CertPEM,
						core.TLSPrivateKeyKey: serverCert.KeyPEM,
					},
				}
				caConfigMap := &core.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "https-backend-ca", Namespace: namespace},
					Data:       map[string]string{"ca.crt": string(ca.CertPEM)},
				}

				Expect(resourceManager.Apply([]client.Object{tlsSecret, caConfigMap})).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(backendFiles, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(policyFiles, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				if !*plusEnabled {
					return
				}

				Expect(resourceManager.DeleteFromFiles(policyFiles, namespace)).To(Succeed())
				Expect(resourceManager.DeleteFromFiles(backendFiles, namespace)).To(Succeed())
				Expect(resourceManager.DeleteResources([]client.Object{
					&core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "https-backend-tls", Namespace: namespace}},
					&core.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "https-backend-ca", Namespace: namespace}},
				})).To(Succeed())
			})

			It("uses BackendTLSPolicy TLS settings in the active health check location", func() {
				expectedFields := []framework.ExpectedNginxField{
					{
						Directive: "proxy_pass",
						Value:     "https://healthcheck_https-backend_443",
						File:      "http.conf",
						Block:     "server",
						Location:  "@hc-healthcheck_https-backend_443",
					},
					{
						Directive: "proxy_ssl_server_name",
						Value:     "on",
						File:      "http.conf",
						Block:     "server",
						Location:  "@hc-healthcheck_https-backend_443",
					},
					{
						Directive: "proxy_ssl_verify",
						Value:     "on",
						File:      "http.conf",
						Block:     "server",
						Location:  "@hc-healthcheck_https-backend_443",
					},
					{
						Directive: "proxy_ssl_name",
						Value:     httpsHostname,
						File:      "http.conf",
						Block:     "server",
						Location:  "@hc-healthcheck_https-backend_443",
					},
					{
						Directive:             "proxy_ssl_trusted_certificate",
						Value:                 "/etc/nginx/secrets/",
						ValueSubstringAllowed: true,
						File:                  "http.conf",
						Block:                 "server",
						Location:              "@hc-healthcheck_https-backend_443",
					},
				}

				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					for _, field := range expectedFields {
						if err := framework.ValidateNginxFieldExists(conf, field); err != nil {
							return err
						}
					}
					return nil
				}).WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		When("an endpoint is unhealthy", func() {
			sodaFiles := []string{"health-check/soda-route.yaml"}
			activeSodaPolicy := []string{"health-check/active-health-check-soda.yaml"}

			BeforeAll(func() {
				if !*plusEnabled {
					Skip("Skipping NGINX Plus Health Check test on NGINX OSS")
				}

				Expect(resourceManager.ApplyFromFiles(sodaFiles, namespace)).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(activeSodaPolicy, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				if !*plusEnabled {
					return
				}

				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(sodaFiles, namespace)).To(Succeed())
				Expect(resourceManager.DeleteFromFiles(activeSodaPolicy, namespace)).To(Succeed())
			})

			It("is marked as unhealthy after `fails` consecutive failed checks", func() {
				Eventually(func() error {
					logs, err := resourceManager.GetPodLogs(
						namespace,
						nginxPodName,
						&core.PodLogOptions{Container: "nginx"},
					)
					if err != nil {
						return err
					}

					if !regexp.MustCompile(
						`peer is unhealthy while connecting to upstream .*in upstream "healthcheck_soda_80"`,
					).MatchString(logs) {
						return fmt.Errorf("active health check did not disable the failed endpoint")
					}
					return nil
				}).WithTimeout(failTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		When("gRPC health checks are configured", func() {
			grpcFiles := []string{"health-check/grpc-backend.yaml", "health-check/grpc-route.yaml"}
			grpcHealthCheck := []string{"health-check/grpc-health-check.yaml"}

			BeforeAll(func() {
				if !*plusEnabled {
					Skip("Skipping NGINX Plus Health Check test on NGINX OSS")
				}

				Expect(resourceManager.ApplyFromFiles(grpcFiles, namespace)).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(grpcHealthCheck, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				if !*plusEnabled {
					return
				}

				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(grpcFiles, namespace)).To(Succeed())
				Expect(resourceManager.DeleteFromFiles(grpcHealthCheck, namespace)).To(Succeed())
			})

			Specify("health checks behave as expected", func() {
				policyName := types.NamespacedName{
					Name:      "example-grpc-hc",
					Namespace: namespace,
				}
				Expect(waitForUSPolicyStatus(
					policyName,
					"gateway",
					metav1.ConditionTrue,
					gatewayv1.PolicyReasonAccepted,
				)).To(Succeed())

				expectedPlusDirectives := []framework.ExpectedNginxField{
					{
						Directive:             "health_check",
						Value:                 "type=grpc grpc_service=helloworld.Greeter grpc_status=12",
						File:                  "http.conf",
						Block:                 "server",
						Location:              "@hc-healthcheck_grpc-backend_8080",
						ValueSubstringAllowed: true,
					},
					{
						Directive: "grpc_pass",
						File:      "http.conf",
						Block:     "server",
						Location:  "@hc-healthcheck_grpc-backend_8080",
						Value:     "grpc://healthcheck_grpc-backend_8080",
					},
				}
				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					for _, directive := range expectedPlusDirectives {
						if err := framework.ValidateNginxFieldExists(conf, directive); err != nil {
							return err
						}
					}
					return nil
				}).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				Eventually(func() error {
					logs, err := resourceManager.GetPodLogs(
						namespace,
						nginxPodName,
						&core.PodLogOptions{Container: "nginx"},
					)
					if err != nil {
						return err
					}

					grpcResponseError := regexp.MustCompile(
						`upstream sent no valid HTTP/1\.0 header while reading response header from ` +
							`upstream \(status 0\).*healthcheck_grpc-backend_8080`,
					)

					if grpcResponseError.MatchString(logs) {
						return fmt.Errorf("grpc health check failed to respond")
					}
					return nil
				}).WithTimeout(failTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})
	})
})

const (
	// requestAttempts is arbritrary, must be large enough so enough requests are sent to soda-bad endpoint.
	requestAttempts = 15
	failTimeout     = 5 * time.Second
)
