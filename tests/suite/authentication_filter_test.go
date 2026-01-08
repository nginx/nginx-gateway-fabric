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

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
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

			DescribeTable("Authenticated/unauthenticated requests",
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
										framework.WithTestHeaders(test.headers))
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
										framework.WithTestHeaders(test.headers))
								}).
								WithTimeout(timeoutConfig.RequestTimeout).
								WithPolling(500 * time.Millisecond).
								Should(Succeed())
						}
					}
				},
				Entry("requests configurations", []test{
					// Requests expected to return 200
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
					// Requests expected to return 401
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

			DescribeTable("Authenticated/unauthenticated requests",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s, expected response code: %d\n", test.desc, test.responseCode)
						if test.responseCode == 200 {
							Eventually(
								func() error {
									return framework.ExpectGRPCRequestToSucceed(
										timeoutConfig.RequestTimeout,
										fmt.Sprintf("%s:%d", address, port),
										framework.WithTestHeaders(test.headers),
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
										framework.WithTestHeaders(test.headers),
									)
								}).
								WithTimeout(timeoutConfig.RequestTimeout).
								WithPolling(500 * time.Millisecond).
								Should(Succeed())
						}
					}
				},
				Entry("requests with valid authentication", []test{
					// Requests expected to return 200
					{
						desc: "Send gRPC request with basic-auth2",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
						},
						responseCode: 200,
					},
					// Requests expected to return Unauthenticated
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
										framework.WithTestHeaders(test.headers))
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
										framework.WithTestHeaders(test.headers))
								}).
								WithTimeout(timeoutConfig.RequestTimeout).
								WithPolling(500 * time.Millisecond).
								Should(Succeed())
						}
					}
				},
				Entry("requests configurations", []test{
					// Requests expected to return 200
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
					// Requests expected to return 500
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
						framework.WithTestHeaders(map[string]string{
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
