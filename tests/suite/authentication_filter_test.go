package main

import (
	"context"
	"fmt"
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

var _ = Describe("AuthenticationFilter", Ordered, Label("functional", "authentication-filter"), func() {
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

	When("valid AuthenticationFilters are applied to the resources", func() {
		AuthenticationFilters := []string{
			"authentication-filter/basic-valid-auth.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(AuthenticationFilters, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			Expect(resourceManager.DeleteFromFiles(AuthenticationFilters, namespace)).To(Succeed())
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

		Context("verify traffic with valid AuthenticationFilter configurations for HTTPRoutes", func() {
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

		Context("verify traffic with valid AuthenticationFilter configurations for GRPCRoutes", func() {
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

	When("invalid AuthenticationFilters are applied to the resources", func() {
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

		Context("verify traffic for HTTPRoutes configured with valid and invalid AuthenticationFilters", func() {
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

	When("valid JWT AuthenticationFilter is applied to the resources", func() {
		var (
			jwtHelper *JWTTestHelper
			jwtSecret *core.Secret
			jwtRoute  *gatewayv1.HTTPRoute
			jwtFilter *ngfAPI.AuthenticationFilter
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

			// Create AuthenticationFilter
			jwtFilter = &ngfAPI.AuthenticationFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwt-auth-file",
					Namespace: namespace,
				},
				Spec: ngfAPI.AuthenticationFilterSpec{
					Type: ngfAPI.AuthTypeJWT,
					JWT: &ngfAPI.JWTAuth{
						Realm:  "JWT Authentication",
						Source: ngfAPI.JWTKeySourceFile,
						File: &ngfAPI.JWTFileKeySource{
							SecretRef: ngfAPI.LocalObjectReference{
								Name: "jwks-secret",
							},
						},
						KeyCache: helpers.GetPointer[ngfAPI.Duration]("10m"),
					},
				},
			}

			// Create HTTPRoute
			jwtRoute = &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwt-cafe-routes",
					Namespace: namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{
							{
								Name: "auth-gateway",
							},
						},
					},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
										Value: helpers.GetPointer("/jwt-coffee"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "coffee",
											Port: helpers.GetPointer[gatewayv1.PortNumber](80),
										},
									},
								},
							},
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterExtensionRef,
									ExtensionRef: &gatewayv1.LocalObjectReference{
										Group: "gateway.nginx.org",
										Kind:  "AuthenticationFilter",
										Name:  "jwt-auth-file",
									},
								},
							},
						},
					},
				},
			}

			// Apply resources
			Expect(resourceManager.Apply([]client.Object{jwtSecret, jwtFilter, jwtRoute})).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

			// Delete resources
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
			defer cancel()

			Expect(resourceManager.Delete(ctx, jwtRoute, nil)).To(Succeed())
			Expect(resourceManager.Delete(ctx, jwtFilter, nil)).To(Succeed())
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

	When("JWT AuthenticationFilter with incorrect secretRef is applied", func() {
		var (
			invalidJWTFilter *ngfAPI.AuthenticationFilter
			invalidJWTRoute  *gatewayv1.HTTPRoute
		)

		BeforeAll(func() {
			if !*plusEnabled {
				Skip("Skipping JWT AuthenticationFilter tests on NGINX OSS deployment (JWT requires NGINX Plus)")
			}

			// Create AuthenticationFilter with non-existent secret reference
			invalidJWTFilter = &ngfAPI.AuthenticationFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwt-auth-invalid-secret",
					Namespace: namespace,
				},
				Spec: ngfAPI.AuthenticationFilterSpec{
					Type: ngfAPI.AuthTypeJWT,
					JWT: &ngfAPI.JWTAuth{
						Realm:  "JWT Authentication",
						Source: ngfAPI.JWTKeySourceFile,
						File: &ngfAPI.JWTFileKeySource{
							SecretRef: ngfAPI.LocalObjectReference{
								Name: "non-existent-secret",
							},
						},
						KeyCache: helpers.GetPointer[ngfAPI.Duration]("10m"),
					},
				},
			}

			// Create HTTPRoute
			invalidJWTRoute = &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwt-invalid-secret-route",
					Namespace: namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{
							{
								Name: "auth-gateway",
							},
						},
					},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
										Value: helpers.GetPointer("/jwt-invalid-secret"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "coffee",
											Port: helpers.GetPointer[gatewayv1.PortNumber](80),
										},
									},
								},
							},
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterExtensionRef,
									ExtensionRef: &gatewayv1.LocalObjectReference{
										Group: "gateway.nginx.org",
										Kind:  "AuthenticationFilter",
										Name:  "jwt-auth-invalid-secret",
									},
								},
							},
						},
					},
				},
			}

			// Apply resources
			Expect(resourceManager.Apply([]client.Object{invalidJWTFilter, invalidJWTRoute})).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

			// Delete resources
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
			defer cancel()

			Expect(resourceManager.Delete(ctx, invalidJWTRoute, nil)).To(Succeed())
			Expect(resourceManager.Delete(ctx, invalidJWTFilter, nil)).To(Succeed())
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
		var (
			invalidJWKSSecret *core.Secret
			invalidJWKSFilter *ngfAPI.AuthenticationFilter
			invalidJWKSRoute  *gatewayv1.HTTPRoute
		)

		BeforeAll(func() {
			if !*plusEnabled {
				Skip("Skipping JWT AuthenticationFilter tests on NGINX OSS deployment (JWT requires NGINX Plus)")
			}

			// Create Secret with invalid JWKS data
			invalidJWKSSecret = &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-jwks-secret",
					Namespace: namespace,
				},
				Type: core.SecretTypeOpaque,
				StringData: map[string]string{
					"auth": "invalid-jwks-data",
				},
			}

			// Create AuthenticationFilter
			invalidJWKSFilter = &ngfAPI.AuthenticationFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwt-auth-invalid-jwks",
					Namespace: namespace,
				},
				Spec: ngfAPI.AuthenticationFilterSpec{
					Type: ngfAPI.AuthTypeJWT,
					JWT: &ngfAPI.JWTAuth{
						Realm:  "JWT Authentication",
						Source: ngfAPI.JWTKeySourceFile,
						File: &ngfAPI.JWTFileKeySource{
							SecretRef: ngfAPI.LocalObjectReference{
								Name: "invalid-jwks-secret",
							},
						},
						KeyCache: helpers.GetPointer[ngfAPI.Duration]("10m"),
					},
				},
			}

			// Create HTTPRoute
			invalidJWKSRoute = &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jwt-invalid-jwks-route",
					Namespace: namespace,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{
							{
								Name: "auth-gateway",
							},
						},
					},
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
										Value: helpers.GetPointer("/jwt-invalid-jwks"),
									},
								},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "coffee",
											Port: helpers.GetPointer[gatewayv1.PortNumber](80),
										},
									},
								},
							},
							Filters: []gatewayv1.HTTPRouteFilter{
								{
									Type: gatewayv1.HTTPRouteFilterExtensionRef,
									ExtensionRef: &gatewayv1.LocalObjectReference{
										Group: "gateway.nginx.org",
										Kind:  "AuthenticationFilter",
										Name:  "jwt-auth-invalid-jwks",
									},
								},
							},
						},
					},
				},
			}

			// Apply resources
			Expect(resourceManager.Apply([]client.Object{invalidJWKSSecret, invalidJWKSFilter, invalidJWKSRoute})).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)

			// Delete resources
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
			defer cancel()

			Expect(resourceManager.Delete(ctx, invalidJWKSRoute, nil)).To(Succeed())
			Expect(resourceManager.Delete(ctx, invalidJWKSFilter, nil)).To(Succeed())
			Expect(resourceManager.Delete(ctx, invalidJWKSSecret, nil)).To(Succeed())
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
