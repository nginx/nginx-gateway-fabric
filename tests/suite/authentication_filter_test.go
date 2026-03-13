package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
				filePrefix := fmt.Sprintf("/etc/nginx/secrets/basic_auth_%s", namespace)
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
		When("valid JWT AuthenticationFilter is applied to the resources", func() {
			var (
				jwtHelper        *JWTTestHelper
				jwtSecret        *core.Secret
				jwtManifestFiles = []string{"authentication-filter/jwt-valid-auth.yaml"}
			)

			BeforeAll(func() {
				if !*plusEnabled {
					Skip("Skipping JWT AuthenticationFilter tests on NGINX OSS deployment (JWT requires NGINX Plus)")
				}

				var err error
				// Generate JWT keys, JWKS, and token
				jwtHelper, err = NewJWTTestHelper("test-key-id")
				Expect(err).ToNot(HaveOccurred())

				// Create Secret with JWKS
				jwtSecret = &core.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "jwks-secret",
						Namespace: namespace,
					},
					Type: core.SecretTypeOpaque,
					StringData: map[string]string{
						"auth": jwtHelper.JWKS,
					},
				}

				// Apply secret and manifest resources
				Expect(resourceManager.Apply([]client.Object{jwtSecret})).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(jwtManifestFiles, namespace)).To(Succeed())
			})

			AfterAll(func() {
				framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

				// Delete resources
				Expect(resourceManager.DeleteFromFiles(jwtManifestFiles, namespace)).To(Succeed())

				ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
				defer cancel()

				Expect(resourceManager.Delete(ctx, jwtSecret, nil)).To(Succeed())

				// Cleanup JWT helper (zero out sensitive data)
				if jwtHelper != nil {
					jwtHelper.Cleanup()
				}
			})

			Specify("JWT authenticationFilter is accepted", func() {
				nsname := types.NamespacedName{Name: "jwt-auth-file", Namespace: namespace}

				Eventually(checkForAuthenticationFilterToBeAccepted).
					WithArguments(nsname).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500*time.Millisecond).
					Should(Succeed(), "jwt-auth-file was not accepted")
			})

			Context("verify traffic with valid JWT AuthenticationFilter configuration for HTTPRoutes", func() {
				It("should successfully authenticate with valid JWT token", func() {
					Eventually(
						func() error {
							return framework.ExpectRequestToSucceed(
								timeoutConfig.RequestTimeout,
								fmt.Sprintf("http://cafe.example.com:%d/jwt-coffee", port),
								address,
								"URI: /jwt-coffee",
								framework.WithRequestHeaders(map[string]string{
									"Authorization": fmt.Sprintf("Bearer %s", jwtHelper.Token),
								}))
						}).
						WithTimeout(timeoutConfig.RequestTimeout).
						WithPolling(500 * time.Millisecond).
						Should(Succeed())
				})

				It("should return 401 for invalid JWT token", func() {
					Eventually(
						func() error {
							return framework.ExpectUnauthenticatedRequest(
								timeoutConfig.RequestTimeout,
								fmt.Sprintf("http://cafe.example.com:%d/jwt-coffee", port),
								address,
								framework.WithRequestHeaders(map[string]string{
									"Authorization": "Bearer invalid.jwt.token",
								}))
						}).
						WithTimeout(timeoutConfig.RequestTimeout).
						WithPolling(500 * time.Millisecond).
						Should(Succeed())
				})
			})

			Context("nginx directives", func() {
				var conf *framework.Payload
				// The JWT key file path follows the pattern: /etc/nginx/secrets/jwt_auth_{namespace}_{secret_name}
				jwtKeyFilePath := fmt.Sprintf("/etc/nginx/secrets/jwt_auth_%s_jwks-secret", namespace)

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
					Entry("JWT authentication", []framework.ExpectedNginxField{
						{
							Directive: "auth_jwt",
							Value:     "JWT Authentication",
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/jwt-coffee",
						},
						{
							Directive: "auth_jwt_key_file",
							Value:     jwtKeyFilePath,
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/jwt-coffee",
						},
						{
							Directive: "auth_jwt_key_cache",
							Value:     "10m",
							File:      "http.conf",
							Server:    "*.example.com",
							Location:  "/jwt-coffee",
						},
					}),
				)
			})
		})
	})

	When("JWT AuthenticationFilter with incorrect secretRef is applied", func() {
		invalidSecretManifestFiles := []string{"authentication-filter/jwt-invalid-secret.yaml"}

		BeforeAll(func() {
			if !*plusEnabled {
				Skip("Skipping JWT AuthenticationFilter tests on NGINX OSS deployment (JWT requires NGINX Plus)")
			}

			// Apply manifest resources
			Expect(resourceManager.ApplyFromFiles(invalidSecretManifestFiles, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

			// Delete resources
			Expect(resourceManager.DeleteFromFiles(invalidSecretManifestFiles, namespace)).To(Succeed())
		})

		Context("verify traffic returns 500 for misconfigured JWT filter", func() {
			It("should return 500 when JWT filter references non-existent secret", func() {
				Eventually(
					func() error {
						return framework.Expect500Response(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("http://cafe.example.com:%d/jwt-invalid-secret", port),
							address,
							framework.WithRequestHeaders(map[string]string{
								"Authorization": "Bearer some.jwt.token",
							}))
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})
	})

	When("JWT AuthenticationFilter with invalid secret value is applied", func() {
		invalidJWKSManifestFiles := []string{"authentication-filter/jwt-invalid-jwks.yaml"}

		BeforeAll(func() {
			if !*plusEnabled {
				Skip("Skipping JWT AuthenticationFilter tests on NGINX OSS deployment (JWT requires NGINX Plus)")
			}

			// Apply manifest resources
			Expect(resourceManager.ApplyFromFiles(invalidJWKSManifestFiles, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

			// Delete resources
			Expect(resourceManager.DeleteFromFiles(invalidJWKSManifestFiles, namespace)).To(Succeed())
		})

		Context("verify traffic returns 401 for JWT filter with invalid JWKS", func() {
			It("should return 401 when JWT filter uses secret with invalid JWKS data", func() {
				Eventually(
					func() error {
						return framework.ExpectUnauthenticatedRequest(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("http://cafe.example.com:%d/jwt-invalid-jwks", port),
							address,
							framework.WithRequestHeaders(map[string]string{
								"Authorization": "Bearer some.jwt.token",
							}))
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})
	})

	When("valid JWT AuthenticationFilter with Remote source (Keycloak) is applied", func() {
		var (
			keycloakToken             string
			keycloakPortForwardStopCh chan struct{}
			keycloakTLSSecret         *core.Secret
			jwksClientSecret          *core.Secret
			keycloakManifestFiles     = []string{
				"authentication-filter/keycloak.yaml",
				"authentication-filter/nginx-proxy.yaml",
			}
			jwtRemoteManifestFiles = []string{
				"authentication-filter/jwt-remote-auth.yaml",
			}
		)

		BeforeAll(func() {
			if !*plusEnabled {
				Skip("Skipping JWT Remote AuthenticationFilter tests on NGINX OSS deployment (JWT requires NGINX Plus)")
			}

			// Generate TLS certificates
			GinkgoWriter.Println("Generating TLS certificates for Keycloak...")
			caCertPEM, _, serverCert, serverKey, clientCert, clientKey, err := generateTLSCertificates(namespace)
			Expect(err).ToNot(HaveOccurred())

			// Create Keycloak TLS secret
			keycloakTLSSecret = &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keycloak-tls-cert",
					Namespace: namespace,
				},
				Type: core.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": serverCert,
					"tls.key": serverKey,
				},
			}

			// Create client certificate secret with CA
			jwksClientSecret = &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwks-client-cert",
					Namespace: namespace,
				},
				Type: core.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": clientCert,
					"tls.key": clientKey,
					"ca.crt":  caCertPEM,
				},
			}

			// Apply secrets
			Expect(resourceManager.Apply([]client.Object{keycloakTLSSecret, jwksClientSecret})).To(Succeed())

			// Deploy Keycloak and NginxProxy with DNS resolver
			Expect(resourceManager.ApplyFromFiles(keycloakManifestFiles, namespace)).To(Succeed())

			// Wait for Keycloak to be ready
			GinkgoWriter.Println("Waiting for Keycloak to be ready...")
			Eventually(func() error {
				pods, err := resourceManager.GetPods(namespace, client.MatchingLabels{
					"app": "keycloak",
				})
				if err != nil {
					return err
				}
				if len(pods) == 0 {
					return fmt.Errorf("no keycloak pods found")
				}
				for _, pod := range pods {
					for _, condition := range pod.Status.Conditions {
						if condition.Type == core.PodReady && condition.Status == core.ConditionTrue {
							return nil
						}
					}
				}
				return fmt.Errorf("keycloak pod not ready")
			}).
				WithTimeout(1*time.Minute).
				WithPolling(5*time.Second).
				Should(Succeed(), "Keycloak should be ready")

			// Port forward to Keycloak for configuration
			GinkgoWriter.Println("Setting up port-forward to Keycloak...")
			pods, err := resourceManager.GetPods(namespace, client.MatchingLabels{
				"app": "keycloak",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(pods).ToNot(BeEmpty())
			keycloakPodName := pods[0].Name

			// Set up port forwarding using framework
			keycloakPortForwardStopCh = make(chan struct{})
			ports := []string{"8443:8443"}
			err = framework.PortForward(resourceManager.K8sConfig, namespace, keycloakPodName, ports, keycloakPortForwardStopCh)
			Expect(err).ToNot(HaveOccurred())

			// Configure Keycloak via Admin API
			GinkgoWriter.Println("Configuring Keycloak...")

			// Wait for port-forward to establish and get admin token
			var adminToken string
			GinkgoWriter.Println("Waiting for port-forward to establish and obtaining admin token...")
			Eventually(func() error {
				var err error
				adminToken, err = getKeycloakAdminToken(caCertPEM)
				if err != nil {
					GinkgoWriter.Printf("Port-forward not ready yet or token request failed: %v\n", err)
					return err
				}
				return nil
			}).
				WithTimeout(30*time.Second).
				WithPolling(2*time.Second).
				Should(Succeed(), "Should establish port-forward and obtain admin token")

			// Create realm
			err = createKeycloakRealm(adminToken, caCertPEM)
			if err != nil {
				GinkgoWriter.Printf("Warning: Failed to create realm (may already exist): %v\n", err)
			}

			// Create client
			err = createKeycloakClient(adminToken, caCertPEM)
			if err != nil {
				GinkgoWriter.Printf("Warning: Failed to create client (may already exist): %v\n", err)
			}

			// Create user
			err = createKeycloakUser(adminToken, caCertPEM)
			if err != nil {
				GinkgoWriter.Printf("Warning: Failed to create user (may already exist): %v\n", err)
			}

			// Set user password using reset-password endpoint
			err = updateKeycloakUser(adminToken, caCertPEM)
			if err != nil {
				GinkgoWriter.Printf("Warning: Failed to set user password: %v\n", err)
			} else {
				GinkgoWriter.Println("✅ User password set successfully")
			}

			// Get JWT token for test user
			GinkgoWriter.Println("Obtaining JWT token from Keycloak...")
			Eventually(func() error {
				var err error
				keycloakToken, err = getKeycloakUserToken(caCertPEM)
				if err != nil {
					GinkgoWriter.Printf("Token retrieval attempt failed: %v\n", err)
					return err
				}
				if keycloakToken == "" {
					return fmt.Errorf("received empty token")
				}
				return nil
			}).
				WithTimeout(30*time.Second).
				WithPolling(2*time.Second).
				Should(Succeed(), "Should obtain JWT token from Keycloak")

			GinkgoWriter.Printf("Successfully obtained token (length: %d)\n", len(keycloakToken))

			// Apply JWT Remote AuthenticationFilter and HTTPRoute
			Expect(resourceManager.ApplyFromFiles(jwtRemoteManifestFiles, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

			// Clean up port forward
			if keycloakPortForwardStopCh != nil {
				GinkgoWriter.Println("Cleaning up Keycloak port-forward...")
				close(keycloakPortForwardStopCh)
			}

			// Delete resources
			Expect(resourceManager.DeleteFromFiles(jwtRemoteManifestFiles, namespace)).To(Succeed())
			Expect(resourceManager.DeleteFromFiles(keycloakManifestFiles, namespace)).To(Succeed())

			// Delete secrets
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
			defer cancel()

			if keycloakTLSSecret != nil {
				Expect(resourceManager.Delete(ctx, keycloakTLSSecret, nil)).To(Succeed())
			}
			if jwksClientSecret != nil {
				Expect(resourceManager.Delete(ctx, jwksClientSecret, nil)).To(Succeed())
			}
		})

		Specify("JWT remote authenticationFilter is accepted", func() {
			nsname := types.NamespacedName{Name: "jwt-remote-auth", Namespace: namespace}

			Eventually(checkForAuthenticationFilterToBeAccepted).
				WithArguments(nsname).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "jwt-remote-auth was not accepted")
		})

		Context("verify traffic with valid JWT Remote AuthenticationFilter configuration for HTTPRoutes", func() {
			It("should successfully authenticate with valid JWT token from Keycloak", func() {
				Eventually(
					func() error {
						return framework.ExpectRequestToSucceed(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("http://cafe.example.com:%d/jwt-remote-coffee", port),
							address,
							"URI: /jwt-remote-coffee",
							framework.WithRequestHeaders(map[string]string{
								"Authorization": fmt.Sprintf("Bearer %s", keycloakToken),
							}))
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("should return 401 for invalid JWT token", func() {
				Eventually(
					func() error {
						return framework.ExpectUnauthenticatedRequest(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("http://cafe.example.com:%d/jwt-remote-coffee", port),
							address,
							framework.WithRequestHeaders(map[string]string{
								"Authorization": "Bearer invalid.jwt.token",
							}))
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("should return 401 when no token is provided", func() {
				Eventually(
					func() error {
						return framework.ExpectUnauthenticatedRequest(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("http://cafe.example.com:%d/jwt-remote-coffee", port),
							address,
						)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("should allow access to unprotected endpoint without authentication", func() {
				Eventually(
					func() error {
						return framework.ExpectRequestToSucceed(
							timeoutConfig.RequestTimeout,
							fmt.Sprintf("http://cafe.example.com:%d/jwt-remote-tea", port),
							address,
							"URI: /jwt-remote-tea",
						)
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
				Entry("JWT remote authentication", []framework.ExpectedNginxField{
					{
						Directive: "auth_jwt",
						Value:     "JWT Remote Authentication",
						File:      "http.conf",
						Server:    "*.example.com",
						Location:  "/jwt-remote-coffee",
					},
					{
						Directive: "auth_jwt_key_request",
						Value:     fmt.Sprintf("/_ngf-internal-%s_jwt-remote-auth_jwks_uri", namespace),
						File:      "http.conf",
						Server:    "*.example.com",
						Location:  "/jwt-remote-coffee",
					},
					{
						Directive: "auth_jwt_key_cache",
						Value:     "10m",
						File:      "http.conf",
						Server:    "*.example.com",
						Location:  "/jwt-remote-coffee",
					},
				}),
			)

			It("should have internal location for JWKS retrieval with HTTPS", func() {
				internalLocation := fmt.Sprintf("/_ngf-internal-%s_jwt-remote-auth_jwks_uri", namespace)

				keycloakURL := fmt.Sprintf(
					"https://keycloak.%s.svc.cluster.local:8443/realms/myrealm/protocol/openid-connect/certs", namespace,
				)
				// Validate proxy_pass
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_pass",
					Value:     keycloakURL,
					File:      "http.conf",
					Server:    "*.example.com",
					Location:  internalLocation,
				})).To(Succeed())

				// Validate TLS client certificate settings (mTLS)
				clientCertPath := fmt.Sprintf(
					"/etc/nginx/secrets/jwt_remote_tls_%s_jwks-client-cert_jwt-remote-auth.pem", namespace,
				)
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_ssl_certificate",
					Value:     clientCertPath,
					File:      "http.conf",
					Server:    "*.example.com",
					Location:  internalLocation,
				})).To(Succeed())

				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_ssl_certificate_key",
					Value:     clientCertPath,
					File:      "http.conf",
					Server:    "*.example.com",
					Location:  internalLocation,
				})).To(Succeed())

				// Validate server certificate verification
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_ssl_verify",
					Value:     "on",
					File:      "http.conf",
					Server:    "*.example.com",
					Location:  internalLocation,
				})).To(Succeed())

				caCertPath := fmt.Sprintf("/etc/nginx/secrets/jwt_remote_tls_ca_%s_jwks-client-cert_jwt-remote-auth.crt", namespace)
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_ssl_trusted_certificate",
					Value:     caCertPath,
					File:      "http.conf",
					Server:    "*.example.com",
					Location:  internalLocation,
				})).To(Succeed())

				// Validate SNI settings
				Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
					Directive: "proxy_ssl_server_name",
					Value:     "on",
					File:      "http.conf",
					Server:    "*.example.com",
					Location:  internalLocation,
				})).To(Succeed())
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

// JWTTestHelper manages JWT authentication test resources.
type JWTTestHelper struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	JWKS       string
	Token      string
	KID        string
}

// NewJWTTestHelper creates a new JWT test helper with generated keys.
func NewJWTTestHelper(kid string) (*JWTTestHelper, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	helper := &JWTTestHelper{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		KID:        kid,
	}

	// Generate JWKS
	jwks, err := helper.generateJWKS()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWKS: %w", err)
	}
	helper.JWKS = jwks

	// Generate JWT token with far future expiration
	token, err := helper.generateToken(map[string]interface{}{
		"sub":  "test-user",
		"name": "Test User",
		"iat":  time.Now().Unix(),
		"exp":  9999999999,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}
	helper.Token = token

	return helper, nil
}

// generateJWKS creates a JWKS JSON string from the public key.
func (h *JWTTestHelper) generateJWKS() (string, error) {
	// Convert the modulus (n) to base64url
	n := h.PublicKey.N.Bytes()
	nBase64 := base64.RawURLEncoding.EncodeToString(n)

	// Convert the exponent (e) to base64url
	e := big.NewInt(int64(h.PublicKey.E))
	eBytes := e.Bytes()
	eBase64 := base64.RawURLEncoding.EncodeToString(eBytes)

	// Create JWKS structure
	jwks := map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": h.KID,
				"use": "sig",
				"alg": "RS256",
				"n":   nBase64,
				"e":   eBase64,
			},
		},
	}

	jwksBytes, err := json.Marshal(jwks)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWKS: %w", err)
	}

	return string(jwksBytes), nil
}

// generateToken creates a JWT token with the given claims.
func (h *JWTTestHelper) generateToken(claims map[string]interface{}) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = h.KID

	tokenString, err := token.SignedString(h.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// Cleanup removes any resources (currently no-op, but included for completeness).
func (h *JWTTestHelper) Cleanup() {
	// Zero out sensitive data
	if h.PrivateKey != nil {
		h.PrivateKey = nil
	}
	h.Token = ""
}

// Keycloak helper functions for JWT remote authentication testing

// getKeycloakAdminToken retrieves an admin access token from Keycloak.
func getKeycloakAdminToken(caCert []byte) (string, error) {
	// Using HTTPS via port-forward to localhost:8443
	url := "https://localhost:8443/realms/master/protocol/openid-connect/token"

	data := "client_id=admin-cli&username=admin&password=admin&grant_type=password"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create HTTP client with proper certificate verification
	httpClient, err := createSecureHTTPClient(caCert)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get admin token, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("access_token not found in response")
	}

	return token, nil
}

// createKeycloakRealm creates the 'myrealm' realm in Keycloak.
func createKeycloakRealm(adminToken string, caCert []byte) error {
	url := "https://localhost:8443/admin/realms"

	realmData := map[string]interface{}{
		"realm":                       "myrealm",
		"enabled":                     true,
		"verifyEmail":                 false,
		"loginWithEmailAllowed":       true,
		"registrationEmailAsUsername": false,
	}

	jsonData, err := json.Marshal(realmData)
	if err != nil {
		return fmt.Errorf("failed to marshal realm data: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpClient, err := createSecureHTTPClient(caCert)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create realm: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 201 = created, 409 = already exists
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed to create realm, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// createKeycloakClient creates a client in the 'myrealm' realm.
func createKeycloakClient(adminToken string, caCert []byte) error {
	url := "https://localhost:8443/admin/realms/myrealm/clients"

	clientData := map[string]interface{}{
		"clientId":                  "cafe-app",
		"enabled":                   true,
		"publicClient":              true,
		"directAccessGrantsEnabled": true,
		"standardFlowEnabled":       true,
	}

	jsonData, err := json.Marshal(clientData)
	if err != nil {
		return fmt.Errorf("failed to marshal client data: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpClient, err := createSecureHTTPClient(caCert)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 201 = created, 409 = already exists
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed to create client, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// createKeycloakUser creates a test user in the 'myrealm' realm.
func createKeycloakUser(adminToken string, caCert []byte) error {
	url := "https://localhost:8443/admin/realms/myrealm/users"

	userData := map[string]interface{}{
		"username":      "testuser",
		"enabled":       true,
		"email":         "testuser@example.com",
		"emailVerified": true,
		"firstName":     "Test",
		"lastName":      "User",
	}

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpClient, err := createSecureHTTPClient(caCert)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 201 = created, 409 = already exists
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed to create user, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// updateKeycloakUser updates the test user to ensure no required actions.
func updateKeycloakUser(adminToken string, caCert []byte) error {
	// First, get the user ID
	getUserURL := "https://localhost:8443/admin/realms/myrealm/users?username=testuser"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, getUserURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create get user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpClient, err := createSecureHTTPClient(caCert)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read get user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get user, status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response to get user ID
	var users []map[string]interface{}
	if err := json.Unmarshal(body, &users); err != nil {
		return fmt.Errorf("failed to parse users response: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("user not found")
	}

	userID, ok := users[0]["id"].(string)
	if !ok {
		return fmt.Errorf("failed to get user ID")
	}

	// Set the user password using reset-password endpoint
	resetPasswordURL := fmt.Sprintf("https://localhost:8443/admin/realms/myrealm/users/%s/reset-password", userID)

	passwordData := map[string]interface{}{
		"type":      "password",
		"value":     "testpassword",
		"temporary": false,
	}

	jsonData, err := json.Marshal(passwordData)
	if err != nil {
		return fmt.Errorf("failed to marshal password data: %w", err)
	}

	req, err = http.NewRequestWithContext(
		context.Background(), http.MethodPut, resetPasswordURL, bytes.NewReader(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to create reset password request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err = httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read reset password response: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to reset password, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getKeycloakUserToken obtains a JWT token for the test user.
func getKeycloakUserToken(caCert []byte) (string, error) {
	url := "https://localhost:8443/realms/myrealm/protocol/openid-connect/token"

	data := "client_id=cafe-app&username=testuser&password=testpassword&grant_type=password"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient, err := createSecureHTTPClient(caCert)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get user token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("access_token not found in response")
	}

	return token, nil
}

// createSecureHTTPClient creates an HTTP client that properly validates TLS certificates
// using the provided CA certificate. This ensures secure communication with Keycloak.
func createSecureHTTPClient(caCert []byte) (*http.Client, error) {
	// Create a certificate pool and add the CA certificate
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Create TLS config with proper certificate verification
	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

// generateTLSCertificates generates CA, server, and client certificates for TLS testing.
func generateTLSCertificates(namespace string) (
	caCertPEM, caKeyPEM,
	serverCertPEM, serverKeyPEM,
	clientCertPEM, clientKeyPEM []byte,
	err error,
) {
	// Generate CA certificate
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to generate CA private key: %w", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Keycloak-CA",
			Country:    []string{"US"},
			Locality:   []string{"San Francisco"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	caKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey)})

	// Parse CA cert for signing
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Generate server certificate
	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to generate server private key: %w", err)
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("keycloak.%s.svc.cluster.local", namespace),
			Country:    []string{"US"},
			Locality:   []string{"San Francisco"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			"keycloak",
			fmt.Sprintf("keycloak.%s", namespace),
			fmt.Sprintf("keycloak.%s.svc", namespace),
			fmt.Sprintf("keycloak.%s.svc.cluster.local", namespace),
			"localhost",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	serverCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	serverKeyPEM = pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey)},
	)

	// Generate client certificate
	clientPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to generate client private key: %w", err)
	}

	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "nginx-gateway-client",
			Country:    []string{"US"},
			Locality:   []string{"San Francisco"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, caCert, &clientPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	clientCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	clientKeyPEM = pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPrivKey)},
	)

	return caCertPEM, caKeyPEM, serverCertPEM, serverKeyPEM, clientCertPEM, clientKeyPEM, nil
}
