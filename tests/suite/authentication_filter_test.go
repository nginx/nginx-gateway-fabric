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

		Context("verify working traffic with valid response returned for HTTPRoutes requests", func() {
			type test struct {
				desc     string
				url      string // since port is not available at this point, we build full URL in the test
				path     string
				headers  map[string]string
				expected string
			}

			DescribeTable("200 response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
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
					}
				},
				Entry("requests with valid authentication", []test{
					{
						desc: "Send https /coffee1 traffic with basic-auth1",
						url:  "http://cafe.example.com:",
						path: "/coffee1",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
						},
						expected: "URI: /coffee1",
					},
					{
						desc: "Send https /coffee2 traffic with basic-auth1",
						url:  "http://cafe.example.com:",
						path: "/coffee2",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
						},
						expected: "URI: /coffee2",
					},
					{
						desc: "Send https /tea traffic with basic-auth2",
						url:  "http://cafe.example.com:",
						path: "/tea",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
						},
						expected: "URI: /tea",
					},
					{
						desc:     "Send https /latte traffic without authentication",
						url:      "http://cafe.example.com:",
						path:     "/latte",
						headers:  nil,
						expected: "URI: /latte",
					},
				}),
			)

			DescribeTable("401 response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
						Eventually(
							func() error {
								return framework.ExpectUnauthorizedRequest(
									timeoutConfig.RequestTimeout,
									fmt.Sprintf("%s%d%s", test.url, port, test.path),
									address,
									framework.WithTestHeaders(test.headers))
							}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(500 * time.Millisecond).
							Should(Succeed())
					}
				},
				Entry("requests with invalid authentication", []test{
					{
						desc: "Send https /coffee1 traffic with wrong authentication",
						url:  "http://cafe.example.com:",
						path: "/coffee1",
						headers: map[string]string{
							"Authorization": "Basic 0000",
						},
					},
					{
						desc: "Send https /coffee1 traffic without authentication",
						url:  "http://cafe.example.com:",
						path: "/coffee1",
					},
					{
						desc: "Send https /tea traffic with wrong authentication",
						url:  "http://cafe.example.com:",
						path: "/tea",
						headers: map[string]string{
							"Authorization": "Basic 0000",
						},
					},
					{
						desc: "Send https /tea traffic without authentication",
						url:  "http://cafe.example.com:",
						path: "/tea",
					},
				}),
			)
		})

		Context("verify working traffic with valid response returned for GRPCRoutes requests", func() {
			type test struct {
				headers map[string]string
				desc    string
			}

			DescribeTable("Successful response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
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
					}
				},
				Entry("requests with valid authentication", []test{
					{
						desc: "Send gRPC request with basic-auth2",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
						},
					},
				}),
			)

			DescribeTable("Failed response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
						Eventually(
							func() error {
								return framework.ExpectUnauthorizedGRPCRequest(
									timeoutConfig.RequestTimeout,
									fmt.Sprintf("%s:%d", address, port),
									framework.WithTestHeaders(test.headers),
								)
							}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(500 * time.Millisecond).
							Should(Succeed())
					}
				},
				Entry("requests with invalid authentication", []test{
					{
						desc: "Send gRPC request with invalid auth",
						headers: map[string]string{
							"Authorization": "Basic 00000",
						},
					},
					{
						desc: "Send gRPC request without authentication",
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
			validAuthenticationFilter = []string{
				"authentication-filter/basic-auth1.yaml",
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
			Expect(resourceManager.ApplyFromFiles(validAuthenticationFilter, wrongNamespace)).To(Succeed())
			Expect(resourceManager.ApplyFromFiles(invalidAuthenticationFilters, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			Expect(resourceManager.DeleteFromFiles(invalidAuthenticationFilters, namespace)).To(Succeed())
			Expect(resourceManager.DeleteFromFiles(validAuthenticationFilter, wrongNamespace)).To(Succeed())
			Expect(resourceManager.DeleteNamespace(wrongNamespace)).To(Succeed())
		})

		Specify("authenticationFilters are accepted", func() {
			invalidAuthenticationFilterNames := []string{
				"basic-auth-wrong-key",
				"basic-auth-opaque",
			}
			validAuthenticationFilter := "basic-auth2"
			invalidNamespaceAuthenticationFilterNames := "basic-auth1"

			// Check that valid AuthenticationFilter is accepted
			Eventually(checkForAuthenticationFilterToBeAccepted).
				WithArguments(
					types.NamespacedName{Name: validAuthenticationFilter, Namespace: namespace},
				).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), fmt.Sprintf("%s was not accepted", validAuthenticationFilter))

			// Check that invalid AuthenticationFilters are not accepted
			for _, name := range invalidAuthenticationFilterNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				Eventually(checkForAuthenticationFilterToBeAccepted).
					WithArguments(nsname).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500*time.Millisecond).
					ShouldNot(Succeed(), fmt.Sprintf("%s was accepted", name))
			}

			// Check that valid AuthenticationFilter in wrong namespace is not accepted
			Eventually(checkForAuthenticationFilterToBeAccepted).
				WithArguments(
					types.NamespacedName{Name: invalidNamespaceAuthenticationFilterNames, Namespace: wrongNamespace},
				).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), fmt.Sprintf("%s was not accepted", invalidNamespaceAuthenticationFilterNames))
		})

		Context("verify working traffic with valid response returned for HTTPRoutes requests", func() {
			type test struct {
				desc     string
				url      string // since port is not available at this point, we build full URL in the test
				path     string
				headers  map[string]string
				expected string
			}

			DescribeTable("200 response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
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
					}
				},
				Entry("requests with valid authentication", []test{
					{
						desc: "Send https /tea traffic with valid basic-auth2",
						url:  "http://cafe.example.com:",
						path: "/tea",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
						},
						expected: "URI: /tea",
					},
					{
						desc:     "Send https /latte traffic without authentication",
						url:      "http://cafe.example.com:",
						path:     "/latte",
						headers:  nil,
						expected: "URI: /latte",
					},
				}),
			)

			DescribeTable("500 response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
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
				},
				Entry("requests with invalid authentication configuration", []test{
					{
						desc: "Send https /coffee1 traffic with invalid Auth type",
						url:  "http://cafe.example.com:",
						path: "/coffee1",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
						},
					},
					{
						desc: "Send https /coffee2 traffic with invalid Auth type",
						url:  "http://cafe.example.com:",
						path: "/coffee2",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
						},
					},
					{
						desc: "Send https /soda traffic with basic-auth1 in different namespace",
						url:  "http://cafe.example.com:",
						path: "/soda",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjE6cGFzc3dvcmQx",
						},
					},
					{
						desc: "Send https /matcha traffic with not existing AuthenticationFilter",
						url:  "http://cafe.example.com:",
						path: "/matcha",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
						},
					},
					{
						desc: "Send https /chocolate traffic with invalid key",
						url:  "http://cafe.example.com:",
						path: "/chocolate",
						headers: map[string]string{
							"Authorization": "Basic 0000",
						},
					},
				}),
			)
		})

		Context("verify working traffic for GRPCRoutes requests", func() {
			type test struct {
				headers map[string]string
				desc    string
			}

			DescribeTable("Failed response",
				func(tests []test) {
					for _, test := range tests {
						GinkgoWriter.Printf("Test case: %s\n", test.desc)
						Eventually(
							func() error {
								return framework.Expect500GRPCResponse(
									timeoutConfig.RequestTimeout,
									fmt.Sprintf("%s:%d", address, port),
									framework.WithTestHeaders(test.headers),
								)
							}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(500 * time.Millisecond).
							Should(Succeed())
					}
				},
				Entry("requests with invalid authentication", []test{
					{
						desc: "Send gRPC request with invalid key AuthFilter",
						headers: map[string]string{
							"Authorization": "Basic dXNlcjI6cGFzc3dvcmQy",
						},
					},
				}),
			)
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
		ngfControllerName,
		framework.AuthenticationFilterControllers,
		(string)(ngfAPI.AuthenticationFilterConditionTypeAccepted),
		(string)(ngfAPI.AuthenticationFilterConditionReasonAccepted),
	)
}
