package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("AuthenticationFilter", Ordered, Label("functional", "auth-filter"), func() {
	var (
		files = []string{
			"authentication-filter/cafe.yaml",
			"authentication-filter/gateway.yaml",
			"authentication-filter/grpc-backend.yaml",
		}

		namespace = "authentication-filter"

		port         int
		nginxPodName string
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

		// Generate self-signed TLS certificate for the Gateway's HTTPS listener
		cafeCert, err := framework.GenerateSelfSignedCACert("cafe.example.com")
		Expect(err).ToNot(HaveOccurred())

		cafeTLSSecret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cafe-tls-secret",
				Namespace: namespace,
			},
			Type: core.SecretTypeTLS,
			Data: map[string][]byte{
				core.TLSCertKey:       cafeCert.CertPEM,
				core.TLSPrivateKeyKey: cafeCert.KeyPEM,
			},
		}

		Expect(resourceManager.Apply([]client.Object{cafeTLSSecret})).To(Succeed())
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
		port = 80
		if portFwdPort != 0 {
			port = portFwdPort
		}
	})

	AfterAll(func() {
		cleanUpPortForward()

		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	Context("Basic Auth AuthenticationFilter", func() {
		When("valid Basic AuthenticationFilters are applied to the resources", func() {
			authenticationFilters := []string{
				"authentication-filter/basic-valid-auth.yaml",
			}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(authenticationFilters, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(authenticationFilters, namespace)).To(Succeed())
			})

			Specify("authenticationFilters are accepted", func() {
				authenticationFilterNames := []string{
					"basic-auth1",
					"basic-auth2",
					"basic-auth-grpc",
				}

				for _, name := range authenticationFilterNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					Eventually(checkForAuthenticationFilterToBeAccepted).
						WithArguments(nsname).
						WithTimeout(timeoutConfig.GetStatusTimeout).
						WithPolling(500*time.Millisecond).
						Should(Succeed(), fmt.Sprintf("%s was not accepted", name))
				}
			})

			Context("verify traffic with valid Basic AuthenticationFilter configurations for HTTPRoutes", func() {
				type test struct {
					desc         string
					url          string // since port is not available at this point, we build full URL in the test
					path         string
					headers      map[string]string
					expected     string
					responseCode int
				}

				DescribeTable("Authenticated and unauthenticated requests",
					func(tests []test) {
						for _, test := range tests {
							GinkgoWriter.Printf("Test case: %s, expected response code: %d\n", test.desc, test.responseCode)
							if test.responseCode == 200 {
								Eventually(
									func() error {
										return framework.ExpectRequestToSucceed(
											timeoutConfig.RequestTimeout,
											fmt.Sprintf("%s%d%s", test.url, port, test.path),
											address,
											test.expected,
											framework.WithRequestHeaders(test.headers))
									}).
									WithTimeout(timeoutConfig.RequestTimeout).
									WithPolling(500 * time.Millisecond).
									Should(Succeed())
							} else {
								Eventually(
									func() error {
										return framework.ExpectUnauthenticatedRequest(
											timeoutConfig.RequestTimeout,
											fmt.Sprintf("%s%d%s", test.url, port, test.path),
											address,
											framework.WithRequestHeaders(test.headers))
									}).
									WithTimeout(timeoutConfig.RequestTimeout).
									WithPolling(500 * time.Millisecond).
									Should(Succeed())
							}
						}
					},
					Entry("Requests configurations", []test{
						// Expect 200 response code
						{
							desc: "Send https /coffee1 traffic with basic-auth1",
							url:  "http://cafe.example.com:",
							path: "/coffee1",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
							},
							expected:     "URI: /coffee1",
							responseCode: 200,
						},
						{
							desc: "Send https /coffee2 traffic with basic-auth1",
							url:  "http://cafe.example.com:",
							path: "/coffee2",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
							},
							expected:     "URI: /coffee2",
							responseCode: 200,
						},
						{
							desc: "Send https /tea traffic with basic-auth2",
							url:  "http://cafe.example.com:",
							path: "/tea",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							},
							expected:     "URI: /tea",
							responseCode: 200,
						},
						{
							desc:         "Send https /latte traffic without authentication",
							url:          "http://cafe.example.com:",
							path:         "/latte",
							headers:      nil,
							expected:     "URI: /latte",
							responseCode: 200,
						},
						// Expect 401 response code
						{
							desc: "Send https /coffee1 traffic with wrong authentication",
							url:  "http://cafe.example.com:",
							path: "/coffee1",
							headers: map[string]string{
								"Authorization": "Basic 0000",
							},
							responseCode: 401,
						},
						{
							desc:         "Send https /coffee1 traffic without authentication",
							url:          "http://cafe.example.com:",
							path:         "/coffee1",
							responseCode: 401,
						},
						{
							desc: "Send https /tea traffic with wrong authentication",
							url:  "http://cafe.example.com:",
							path: "/tea",
							headers: map[string]string{
								"Authorization": "Basic 0000",
							},
							responseCode: 401,
						},
						{
							desc:         "Send https /tea traffic without authentication",
							url:          "http://cafe.example.com:",
							path:         "/tea",
							responseCode: 401,
						},
					}),
				)
			})

			Context("verify traffic with valid Basic AuthenticationFilter configurations for GRPCRoutes", func() {
				type test struct {
					headers      map[string]string
					desc         string
					responseCode int
				}

				DescribeTable("Authenticated and unauthenticated requests",
					func(tests []test) {
						for _, test := range tests {
							GinkgoWriter.Printf("Test case: %s, expected response code: %d\n", test.desc, test.responseCode)
							if test.responseCode == 200 {
								Eventually(
									func() error {
										return framework.ExpectGRPCRequestToSucceed(
											timeoutConfig.RequestTimeout,
											fmt.Sprintf("%s:%d", address, port),
											framework.WithRequestHeaders(test.headers),
										)
									}).
									WithTimeout(timeoutConfig.RequestTimeout).
									WithPolling(500 * time.Millisecond).
									Should(Succeed())
							} else {
								Eventually(
									func() error {
										return framework.ExpectUnauthenticatedGRPCRequest(
											timeoutConfig.RequestTimeout,
											fmt.Sprintf("%s:%d", address, port),
											framework.WithRequestHeaders(test.headers),
										)
									}).
									WithTimeout(timeoutConfig.RequestTimeout).
									WithPolling(500 * time.Millisecond).
									Should(Succeed())
							}
						}
					},
					Entry("Requests with valid authentication", []test{
						// Expect 200 response code
						{
							desc: "Send gRPC request with basic-auth2",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							},
							responseCode: 200,
						},
						// Expect Unauthenticated response code
						{
							desc: "Send gRPC request with invalid authentication",
							headers: map[string]string{
								"Authorization": "Basic 00000",
							},
							responseCode: 204,
						},
						{
							desc:         "Send gRPC request without authentication",
							responseCode: 204,
						},
					}),
				)
			})

			Context("nginx directives", func() {
				var conf *framework.Payload
				filePrefix := fmt.Sprintf("/etc/nginx/secrets/%s", namespace)
				auth1Suffix := "basic-auth1"
				auth2Suffix := "basic-auth2"
				grpcSuffix := "basic-auth-grpc"

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
					Entry("HTTP authentication", []framework.ExpectedNginxField{
						{
							Directive: "auth_basic_user_file",
							Value:     fmt.Sprintf("%s_%s", filePrefix, auth1Suffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/coffee1",
						},
						{
							Directive: "auth_basic",
							Value:     fmt.Sprintf("Restricted %s", auth1Suffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/coffee1",
						},
						{
							Directive: "auth_basic_user_file",
							Value:     fmt.Sprintf("%s_%s", filePrefix, auth1Suffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/coffee2",
						},
						{
							Directive: "auth_basic",
							Value:     fmt.Sprintf("Restricted %s", auth1Suffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/coffee2",
						},
						{
							Directive: "auth_basic_user_file",
							Value:     fmt.Sprintf("%s_%s", filePrefix, auth2Suffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/tea",
						},
						{
							Directive: "auth_basic",
							Value:     fmt.Sprintf("Restricted %s", auth2Suffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/tea",
						},
					}),
					Entry("GRPC authentication", []framework.ExpectedNginxField{
						{
							Directive: "auth_basic_user_file",
							Value:     fmt.Sprintf("%s_%s", filePrefix, grpcSuffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/helloworld.Greeter/SayHello",
						},
						{
							Directive: "auth_basic",
							Value:     fmt.Sprintf("Restricted %s", grpcSuffix),
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/helloworld.Greeter/SayHello",
						},
					}),
				)
			})
		})

		When("invalid Basic AuthenticationFilters are applied to the resources", func() {
			var (
				invalidAuthenticationFilters = []string{
					"authentication-filter/basic-invalid-auth.yaml",
				}
				wrongWorkspaceAuthenticationFilter = []string{
					"authentication-filter/basic-valid-auth3.yaml",
				}
				wrongNamespace = "wrong-namespace"
			)

			BeforeAll(func() {
				wns := &core.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrongNamespace,
					},
				}
				Expect(resourceManager.Apply([]client.Object{wns})).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(wrongWorkspaceAuthenticationFilter, wrongNamespace)).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(invalidAuthenticationFilters, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
				Expect(resourceManager.DeleteFromFiles(invalidAuthenticationFilters, namespace)).To(Succeed())
				Expect(resourceManager.DeleteFromFiles(wrongWorkspaceAuthenticationFilter, wrongNamespace)).To(Succeed())
				Expect(resourceManager.DeleteNamespace(wrongNamespace)).To(Succeed())
			})

			Specify("authenticationFilters are accepted", func() {
				invalidAuthenticationFilterNames := []string{
					"basic-auth-wrong-key",
					"basic-auth-opaque",
				}
				validAuthenticationFilters := []string{
					"basic-auth1",
					"basic-auth2",
				}
				invalidNamespaceAuthenticationFilterNames := "basic-auth3"

				// Check that valid AuthenticationFilters are accepted regardless of invalid ones
				for _, name := range validAuthenticationFilters {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}
					Eventually(checkForAuthenticationFilterToBeAccepted).
						WithArguments(nsname).
						WithTimeout(timeoutConfig.GetStatusTimeout).
						WithPolling(500*time.Millisecond).
						Should(Succeed(), fmt.Sprintf("%s was not accepted", wrongWorkspaceAuthenticationFilter))
				}
				// Check that invalid AuthenticationFilters are not accepted
				for _, name := range invalidAuthenticationFilterNames {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}

					Eventually(checkForAuthenticationFilterToBeAccepted).
						WithArguments(nsname).
						WithTimeout(timeoutConfig.GetStatusTimeout).
						WithPolling(500*time.Millisecond).
						ShouldNot(Succeed(), fmt.Sprintf("%s was accepted", name))
				}

				// Check that valid AuthenticationFilter in wrong namespace is accepted
				Eventually(checkForAuthenticationFilterToBeAccepted).
					WithArguments(
						types.NamespacedName{Name: invalidNamespaceAuthenticationFilterNames, Namespace: wrongNamespace},
					).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500*time.Millisecond).
					Should(Succeed(), fmt.Sprintf("%s was not accepted", invalidNamespaceAuthenticationFilterNames))
			})

			Context("verify traffic for HTTPRoutes configured with valid and invalid Basic AuthenticationFilters", func() {
				type test struct {
					desc         string
					url          string // since port is not available at this point, we build full URL in the test
					path         string
					headers      map[string]string
					expected     string
					responseCode int
				}

				DescribeTable("Verification for setup with valid and invalid filters configuration",
					func(tests []test) {
						for _, test := range tests {
							GinkgoWriter.Printf("Test case: %s, expected response: %d\n", test.desc, test.responseCode)
							if test.responseCode == 200 {
								Eventually(
									func() error {
										return framework.ExpectRequestToSucceed(
											timeoutConfig.RequestTimeout,
											fmt.Sprintf("%s%d%s", test.url, port, test.path),
											address,
											test.expected,
											framework.WithRequestHeaders(test.headers))
									}).
									WithTimeout(timeoutConfig.RequestTimeout).
									WithPolling(500 * time.Millisecond).
									Should(Succeed())
							} else {
								Eventually(
									func() error {
										return framework.Expect500Response(
											timeoutConfig.RequestTimeout,
											fmt.Sprintf("%s%d%s", test.url, port, test.path),
											address,
											framework.WithRequestHeaders(test.headers))
									}).
									WithTimeout(timeoutConfig.RequestTimeout).
									WithPolling(500 * time.Millisecond).
									Should(Succeed())
							}
						}
					},
					Entry("Requests configurations", []test{
						// Expect 200 response code
						{
							desc: "Send https /tea traffic with valid basic-auth2",
							url:  "http://cafe.example.com:",
							path: "/tea",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							},
							expected:     "URI: /tea",
							responseCode: 200,
						},
						{
							desc:         "Send https /latte traffic without authentication",
							url:          "http://cafe.example.com:",
							path:         "/latte",
							headers:      nil,
							expected:     "URI: /latte",
							responseCode: 200,
						},
						// Expect 500 response code
						{
							desc: "Send https /coffee1 traffic with invalid Auth type",
							url:  "http://cafe.example.com:",
							path: "/coffee1",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
							},
							responseCode: 500,
						},
						{
							desc: "Send https /coffee2 traffic with invalid Auth type",
							url:  "http://cafe.example.com:",
							path: "/coffee2",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
							},
							responseCode: 500,
						},
						{
							desc: "Send https /soda traffic with basic-auth3 in different namespace",
							url:  "http://cafe.example.com:",
							path: "/soda",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjM6cGFzc3dvcmQz",
							},
							responseCode: 500,
						},
						{
							desc: "Send https /matcha traffic with not existing AuthenticationFilter",
							url:  "http://cafe.example.com:",
							path: "/matcha",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							},
							responseCode: 500,
						},
						{
							desc: "Send https /chocolate traffic with invalid key",
							url:  "http://cafe.example.com:",
							path: "/chocolate",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							},
							responseCode: 500,
						},
						{
							desc: "Send https /frappe traffic with twice configured AuthenticationFilters: auth1",
							url:  "http://cafe.example.com:",
							path: "/frappe",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
							},
							responseCode: 500,
						},
						{
							desc: "Send https /frappe traffic with twice configured AuthenticationFilters: auth2",
							url:  "http://cafe.example.com:",
							path: "/frappe",
							headers: map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							},
							responseCode: 500,
						},
					}),
				)
			})

			Context("verify 500 response for invalid filter configured on GRPCRoutes", func() {
				Specify("authenticationFilters are accepted", func() {
					GinkgoWriter.Printf("Test case: Send gRPC request with invalid key AuthFilter\n")
					Eventually(framework.Expect500GRPCResponse).
						WithArguments(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("%s:%d", address, port),
							framework.WithRequestHeaders(map[string]string{
								"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
							}),
						).
						WithTimeout(timeoutConfig.RequestTimeout).
						WithPolling(500 * time.Millisecond).
						Should(Succeed())
				})
			})
		})
	})

	Context("NGINX Plus", func() {
		keycloakFiles := []string{
			"authentication-filter/keycloak.yaml",
		}

		BeforeAll(func() {
			if !*plusEnabled {
				Skip("Skipping NGINX Plus AuthFilter tests on NGINX OSS")
			}

			// Generate self-signed CA and server TLS certificate for Keycloak
			keycloakDNS := fmt.Sprintf("keycloak.%s.svc.cluster.local", namespace)

			ca, err := framework.GenerateSelfSignedCACert("Keycloak Test CA")
			Expect(err).ToNot(HaveOccurred())

			serverCert, err := framework.GenerateSignedServerCert(ca, keycloakDNS, []string{keycloakDNS})
			Expect(err).ToNot(HaveOccurred())

			// Create the TLS secret for Keycloak to serve HTTPS
			keycloakTLSSecret := &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keycloak-tls-cert",
					Namespace: namespace,
				},
				Type: core.SecretTypeTLS,
				Data: map[string][]byte{
					core.TLSCertKey:       serverCert.CertPEM,
					core.TLSPrivateKeyKey: serverCert.KeyPEM,
				},
			}

			// Create the CA secret for the AuthenticationFilter's caCertificateRefs
			keycloakCASecret := &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keycloak-ca-secret",
					Namespace: namespace,
				},
				Type: core.SecretTypeOpaque,
				Data: map[string][]byte{
					"ca.crt": ca.CertPEM,
				},
			}

			Expect(resourceManager.Apply([]client.Object{keycloakTLSSecret, keycloakCASecret})).To(Succeed())

			// Deploy Keycloak (ConfigMap, Deployment, Service)
			Expect(resourceManager.ApplyFromFiles(keycloakFiles, namespace)).To(Succeed())

			// Wait for Keycloak to be ready (can take 60s+)
			keycloakCtx, keycloakCancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer keycloakCancel()

			Expect(resourceManager.WaitForPodsToBeReady(keycloakCtx, namespace)).To(Succeed())
		})

		AfterAll(func() {
			if !*plusEnabled {
				Skip("Skipping NGINX Plus AuthFilter tests on NGINX OSS")
			}

			Expect(resourceManager.DeleteFromFiles(keycloakFiles, namespace)).To(Succeed())
			Expect(resourceManager.DeleteResources([]client.Object{
				&core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keycloak-tls-cert", Namespace: namespace}},
				&core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keycloak-ca-secret", Namespace: namespace}},
			})).To(Succeed())
		})

		Context("OIDC AuthenticationFilter", func() {
			When("valid OIDC AuthenticationFilter is applied", Ordered, func() {
				var (
					oidcFiles = []string{
						"authentication-filter/oidc-valid-auth.yaml",
					}
					clientPodFiles = []string{
						"authentication-filter/oidc-client-pod.yaml",
					}

					savedDNSResolver *ngfAPIv1alpha2.DNSResolver
					// nginxServiceIP is the ClusterIP of the NGINX Gateway service, looked up in BeforeAll.
					// Used by curl --resolve to map cafe.example.com to the NGINX service.
					nginxServiceIP string

					// clientPodName is the name of the in-cluster curl pod used to perform the OIDC flow.
					clientPodName = "oidc-test-client"
				)

				BeforeAll(func() {
					// Look up the kube-dns service ClusterIP for NGINX resolver configuration.
					// The OIDC issuer uses an in-cluster DNS name, so NGINX needs an explicit resolver directive.
					ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.UpdateTimeout)
					defer cancel()

					var kubeDNSSvc core.Service
					kubeDNSKey := types.NamespacedName{Name: "kube-dns", Namespace: "kube-system"}
					Expect(resourceManager.Get(ctx, kubeDNSKey, &kubeDNSSvc)).To(Succeed())
					kubeDNSIP := kubeDNSSvc.Spec.ClusterIP
					Expect(kubeDNSIP).ToNot(BeEmpty(), "kube-dns ClusterIP should not be empty")

					// Patch NginxProxy with DNS resolver pointing to kube-dns
					proxyKey := types.NamespacedName{Name: "ngf-test-proxy-config", Namespace: "nginx-gateway"}
					var nginxProxy ngfAPIv1alpha2.NginxProxy
					Expect(resourceManager.Get(ctx, proxyKey, &nginxProxy)).To(Succeed())

					savedDNSResolver = nginxProxy.Spec.DNSResolver

					nginxProxy.Spec.DNSResolver = &ngfAPIv1alpha2.DNSResolver{
						Addresses: []ngfAPIv1alpha2.DNSResolverAddress{
							{
								Type:  ngfAPIv1alpha2.DNSResolverIPAddressType,
								Value: kubeDNSIP,
							},
						},
					}
					Expect(resourceManager.Update(ctx, &nginxProxy, nil)).To(Succeed())

					// Deploy OIDC manifests (Gateway, HTTPRoute, AuthenticationFilter, Secrets)
					Expect(resourceManager.ApplyFromFiles(oidcFiles, namespace)).To(Succeed())
					Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

					// Look up the NGINX service ClusterIP for use with curl --resolve.
					var nginxSvc core.Service
					nginxSvcKey := types.NamespacedName{Name: "auth-gateway-nginx", Namespace: namespace}
					Expect(resourceManager.Get(ctx, nginxSvcKey, &nginxSvc)).To(Succeed())
					nginxServiceIP = nginxSvc.Spec.ClusterIP
					Expect(nginxServiceIP).ToNot(BeEmpty(), "NGINX service ClusterIP should not be empty")
					GinkgoWriter.Printf("NGINX service ClusterIP: %s\n", nginxServiceIP)

					// Deploy the in-cluster curl client pod for performing the OIDC flow
					Expect(resourceManager.ApplyFromFiles(clientPodFiles, namespace)).To(Succeed())

					clientCtx, clientCancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
					defer clientCancel()

					Expect(resourceManager.WaitForPodsToBeReady(clientCtx, namespace)).To(Succeed())
				})

				AfterAll(func() {
					framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

					// Restore original DNS resolver on NginxProxy
					ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.UpdateTimeout)
					defer cancel()

					proxyKey := types.NamespacedName{Name: "ngf-test-proxy-config", Namespace: "nginx-gateway"}
					var nginxProxy ngfAPIv1alpha2.NginxProxy
					Expect(resourceManager.Get(ctx, proxyKey, &nginxProxy)).To(Succeed())

					nginxProxy.Spec.DNSResolver = savedDNSResolver

					Expect(resourceManager.Update(ctx, &nginxProxy, nil)).To(Succeed())

					Expect(resourceManager.DeleteFromFiles(clientPodFiles, namespace)).To(Succeed())
					Expect(resourceManager.DeleteFromFiles(oidcFiles, namespace)).To(Succeed())
				})

				Specify("OIDC authenticationFilters are accepted", func() {
					filterNames := []string{"oidc-auth-coffee", "oidc-auth-tea"}

					for _, name := range filterNames {
						nsname := types.NamespacedName{Name: name, Namespace: namespace}

						Eventually(checkForAuthenticationFilterToBeAccepted).
							WithArguments(nsname).
							WithTimeout(timeoutConfig.GetStatusTimeout).
							WithPolling(500*time.Millisecond).
							Should(Succeed(), fmt.Sprintf("%s was not accepted", name))
					}
				})

				DescribeTable("should successfully authenticate and log out via OIDC",
					func(path, logoutPath, expectedBody string) {
						// Login
						Eventually(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.RequestTimeout)
							defer cancel()

							statusCode, body, err := framework.PerformOIDCLoginInCluster(
								ctx,
								resourceManager.ClientGoClient,
								resourceManager.K8sConfig,
								namespace, clientPodName,
								nginxServiceIP, "cafe.example.com", path,
								"testuser", "testpassword",
							)
							if err != nil {
								return fmt.Errorf("OIDC login failed: %w", err)
							}
							if statusCode != 200 {
								return fmt.Errorf("expected status 200, got %d, body: %s", statusCode, body)
							}
							if !strings.Contains(body, expectedBody) {
								return fmt.Errorf("expected response body to contain %q, got: %s", expectedBody, body)
							}
							return nil
						}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(5 * time.Second).
							Should(Succeed())

						// Logout
						Eventually(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.RequestTimeout)
							defer cancel()

							statusCode, body, err := framework.PerformOIDCLogoutInCluster(
								ctx,
								resourceManager.ClientGoClient,
								resourceManager.K8sConfig,
								namespace, clientPodName,
								nginxServiceIP, "cafe.example.com",
								logoutPath, path,
							)
							if err != nil {
								return fmt.Errorf("OIDC logout failed: %w", err)
							}
							if statusCode != 200 {
								return fmt.Errorf("expected logout status 200, got %d, body: %s", statusCode, body)
							}
							if !strings.Contains(body, "You are logged out") {
								return fmt.Errorf(
									"expected response to contain %q, got: %s",
									"You are logged out", body,
								)
							}
							return nil
						}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(5 * time.Second).
							Should(Succeed())
					},
					Entry("coffee path with nginx-gateway-coffee client", "/coffee", "/logout-coffee", "URI: /coffee"),
					Entry("tea path with nginx-gateway-tea client", "/tea", "/logout-tea", "URI: /tea"),
				)

				Context("nginx directives", func() {
					var conf *framework.Payload
					coffeeProvider := fmt.Sprintf("%s_oidc-auth-coffee", namespace)
					teaProvider := fmt.Sprintf("%s_oidc-auth-tea", namespace)
					coffeeCallback := fmt.Sprintf("/oidc_callback_%s_oidc-auth-coffee", namespace)
					teaCallback := fmt.Sprintf("/oidc_callback_%s_oidc-auth-tea", namespace)
					issuer := "https://keycloak.authentication-filter.svc.cluster.local:8443/realms/nginx-gateway"

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
						Entry("coffee OIDC provider fields", []framework.ExpectedNginxField{
							{
								Directive:  "issuer",
								Value:      issuer,
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: coffeeProvider,
							},
							{
								Directive:  "client_id",
								Value:      "nginx-gateway-coffee",
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: coffeeProvider,
							},
							{
								Directive:             "client_secret",
								Value:                 "oidc-coffee-client-secret",
								File:                  "http.conf",
								Block:                 "oidc_provider",
								BlockValue:            coffeeProvider,
								ValueSubstringAllowed: true,
							},
							{
								Directive:  "redirect_uri",
								Value:      coffeeCallback,
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: coffeeProvider,
							},
							{
								Directive:             "ssl_trusted_certificate",
								Value:                 "keycloak-ca-secret",
								File:                  "http.conf",
								Block:                 "oidc_provider",
								BlockValue:            coffeeProvider,
								ValueSubstringAllowed: true,
							},
							{
								Directive:  "logout_uri",
								Value:      "/logout-coffee",
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: coffeeProvider,
							},
						}),
						Entry("tea OIDC provider fields", []framework.ExpectedNginxField{
							{
								Directive:  "issuer",
								Value:      issuer,
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: teaProvider,
							},
							{
								Directive:  "client_id",
								Value:      "nginx-gateway-tea",
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: teaProvider,
							},
							{
								Directive:             "client_secret",
								Value:                 "oidc-tea-client-secret",
								File:                  "http.conf",
								Block:                 "oidc_provider",
								BlockValue:            teaProvider,
								ValueSubstringAllowed: true,
							},
							{
								Directive:  "redirect_uri",
								Value:      teaCallback,
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: teaProvider,
							},
							{
								Directive:             "ssl_trusted_certificate",
								Value:                 "keycloak-ca-secret",
								File:                  "http.conf",
								Block:                 "oidc_provider",
								BlockValue:            teaProvider,
								ValueSubstringAllowed: true,
							},
							{
								Directive:  "logout_uri",
								Value:      "/logout-tea",
								File:       "http.conf",
								Block:      "oidc_provider",
								BlockValue: teaProvider,
							},
						}),
						Entry("OIDC auth directives in protected locations", []framework.ExpectedNginxField{
							{
								Directive: "auth_oidc",
								Value:     coffeeProvider,
								File:      "http.conf",
								Server:    "cafe.example.com",
								Location:  "/coffee",
							},
							{
								Directive: "auth_oidc",
								Value:     teaProvider,
								File:      "http.conf",
								Server:    "cafe.example.com",
								Location:  "/tea",
							},
						}),
						Entry("OIDC callback locations", []framework.ExpectedNginxField{
							{
								Directive: "auth_oidc",
								Value:     coffeeProvider,
								File:      "http.conf",
								Server:    "cafe.example.com",
								Location:  coffeeCallback,
							},
							{
								Directive: "auth_oidc",
								Value:     teaProvider,
								File:      "http.conf",
								Server:    "cafe.example.com",
								Location:  teaCallback,
							},
						}),
					)
				})
			})
		})
	})
})

func checkForAuthenticationFilterToBeAccepted(authenticationFilterNsNames types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Checking for AuthenticationFilter %q to have the condition Accepted/True/Accepted\n",
		authenticationFilterNsNames,
	)

	var af ngfAPI.AuthenticationFilter
	var err error

	if err = resourceManager.Get(ctx, authenticationFilterNsNames, &af); err != nil {
		return err
	}

	return framework.CheckFilterAccepted(
		af,
		framework.AuthenticationFilterControllers,
		(string)(ngfAPI.AuthenticationFilterConditionTypeAccepted),
		(string)(ngfAPI.AuthenticationFilterConditionReasonAccepted),
	)
}
