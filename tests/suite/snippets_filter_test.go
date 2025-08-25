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
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("SnippetsFilter", Ordered, Label("functional", "snippets-filter"), func() {
	var (
		files = []string{
			"snippets-filter/cafe.yaml",
			"snippets-filter/gateway.yaml",
			"snippets-filter/grpc-backend.yaml",
		}

		namespace = "snippets-filter"

		nginxPodName string
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		GinkgoWriter.Printf("Creating namespace %q\n", ns)
		applyErr := resourceManager.Apply([]client.Object{ns})
		if applyErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource to namespace %q, error: %v\n",
				ns,
				applyErr,
			)
		}
		Expect(applyErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q applied successfully,\n", ns)

		GinkgoWriter.Printf("Applying resources from files %v to namespace %q\n", files, namespace)
		applyFromFilesErr := resourceManager.ApplyFromFiles(files, namespace)
		if applyFromFilesErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during applying resource from files %v, in namespace %q, error: %s\n",
				files,
				namespace,
				applyFromFilesErr,
			)
		}
		Expect(applyFromFilesErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)

		GinkgoWriter.Printf("Waiting for apps to be ready in namespace %q\n", namespace)
		waitingErr := resourceManager.WaitForAppsToBeReady(namespace)
		if waitingErr != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for apps to be ready in namespace %q, error: %s\n",
				namespace,
				waitingErr,
			)
		}
		Expect(waitingErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Apps are ready in namespace %q,\n", namespace)

		GinkgoWriter.Printf("Retrieving ready NGINX pods in namespace %q\n", namespace)
		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, namespace, timeoutConfig.GetStatusTimeout)
		if err != nil {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for NginxPods to be ready in namespace %q, error: %s\n",
				namespace,
				err,
			)
		}
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Nginx pods in namespace %q: %v\n", namespace, nginxPodNames)
		Expect(nginxPodNames).To(HaveLen(1))

		nginxPodName = nginxPodNames[0]

		GinkgoWriter.Printf("Setting up port-forward to nginx pod %s in namespace %q\n", nginxPodNames, namespace)
		setUpPortForward(nginxPodName, namespace)
	})

	AfterAll(func() {
		GinkgoWriter.Printf("Cleaning up portForward")
		cleanUpPortForward()

		GinkgoWriter.Printf("Deleting namespace %q\n", namespace)
		deleteNSErr := resourceManager.DeleteNamespace(namespace)
		if deleteNSErr != nil {
			GinkgoWriter.Printf("ERROR occurred during deleting namespace %q, error: %s\n", namespace, deleteNSErr)
		}
		Expect(deleteNSErr).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Namespace %q deleted successfully\n", namespace)
	})

	When("SnippetsFilters are applied to the resources", func() {
		snippetsFilter := []string{
			"snippets-filter/valid-sf.yaml",
		}

		BeforeAll(func() {
			GinkgoWriter.Printf(
				"Applying resources from files with snippetsFilter: %v to namespace %q\n",
				snippetsFilter,
				namespace,
			)
			applyFromFilesErr := resourceManager.ApplyFromFiles(snippetsFilter, namespace)
			if applyFromFilesErr != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during applying from files with snippetsFilter: %v, error: %v\n",
					snippetsFilter,
					applyFromFilesErr,
				)
			}
			Expect(applyFromFilesErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf("SnippetsFilter: %v applied successfully to namespace %q,\n", snippetsFilter, namespace)
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			GinkgoWriter.Printf("Adding NGINX logs and Events to Report in namespace %q\n", namespace)
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			deleteFromFilesErr := resourceManager.DeleteFromFiles(snippetsFilter, namespace)
			if deleteFromFilesErr != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during deleting from files with snippetsFilter: %v, error: %v\n",
					snippetsFilter,
					deleteFromFilesErr,
				)
			}
			Expect(deleteFromFilesErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf(
				"Resources from files with snippetsFilter: %v deleted successfully from namespace %q,\n",
				snippetsFilter,
				namespace,
			)
		})

		Specify("snippetsFilters are accepted", func() {
			snippetsFilterNames := []string{
				"all-contexts",
				"grpc-all-contexts",
			}

			for _, name := range snippetsFilterNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				Eventually(checkForSnippetsFilterToBeAccepted).
					WithArguments(nsname).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500*time.Millisecond).
					Should(Succeed(), fmt.Sprintf("%q was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoute", func() {
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")
				GinkgoWriter.Printf("Setting up base URL for tests: %s\n", baseURL)

				Eventually(
					func() error {
						return expectRequestToSucceed(baseURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload
			snippetsFilterFilePrefix := "/etc/nginx/includes/SnippetsFilter_"

			mainContext := fmt.Sprintf("%smain_", snippetsFilterFilePrefix)
			httpContext := fmt.Sprintf("%shttp_", snippetsFilterFilePrefix)
			httpServerContext := fmt.Sprintf("%shttp.server_", snippetsFilterFilePrefix)
			httpServerLocationContext := fmt.Sprintf("%shttp.server.location_", snippetsFilterFilePrefix)

			httpRouteSuffix := fmt.Sprintf("%s_all-contexts.conf", namespace)
			grpcRouteSuffix := fmt.Sprintf("%s_grpc-all-contexts.conf", namespace)

			BeforeAll(func() {
				var err error
				GinkgoWriter.Printf(
					"Retrieving NGINX configuration for Pod %s in Namespace %q\n",
					nginxPodName,
					namespace,
				)
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
				if err != nil {
					GinkgoWriter.Printf(
						"ERROR occurred during retrieving NGINX configuration: %v\n",
						err,
					)
				}
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf(
					"NGINX configuration: %v retrieved successfully for Pod %q in Namespace %q\n",
					conf,
					nginxPodName,
					namespace,
				)
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						GinkgoWriter.Printf(
							"Validating NGINX field %q with value %q in file %q, server %q, location %q\n",
							expCfg.Directive,
							expCfg.Value,
							expCfg.File,
							expCfg.Server,
							expCfg.Location,
						)
						validationErr := framework.ValidateNginxFieldExists(conf, expCfg)
						if validationErr != nil {
							GinkgoWriter.Printf(
								"ERROR occurred during validation of NGINX field to exist with expected field: %q, nerror: %s\n",
								expCfg.Directive,
								validationErr,
							)
						}
						Expect(validationErr).ToNot(HaveOccurred())
						GinkgoWriter.Printf("NGINX field %q exists as expected\n", expCfg.Directive)
					}
				},
				Entry("HTTPRoute", []framework.ExpectedNginxField{
					{
						Directive: "worker_priority",
						Value:     "0",
						File:      fmt.Sprintf("%s%s", mainContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", mainContext, httpRouteSuffix),
						File:      "main.conf",
					},
					{
						Directive: "aio",
						Value:     "off",
						File:      fmt.Sprintf("%s%s", httpContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpContext, httpRouteSuffix),
						File:      "http.conf",
					},
					{
						Directive: "auth_delay",
						Value:     "0s",
						File:      fmt.Sprintf("%s%s", httpServerContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerContext, httpRouteSuffix),
						Server:    "cafe.example.com",
						File:      "http.conf",
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerLocationContext, httpRouteSuffix),
						File:      "http.conf",
						Location:  "/coffee",
						Server:    "cafe.example.com",
					},
					{
						Directive: "keepalive_time",
						Value:     "1h",
						File:      fmt.Sprintf("%s%s", httpServerLocationContext, httpRouteSuffix),
					},
				}),
				Entry("GRPCRoute", []framework.ExpectedNginxField{
					{
						Directive: "worker_shutdown_timeout",
						Value:     "120s",
						File:      fmt.Sprintf("%s%s", mainContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", mainContext, grpcRouteSuffix),
						File:      "main.conf",
					},
					{
						Directive: "types_hash_bucket_size",
						Value:     "64",
						File:      fmt.Sprintf("%s%s", httpContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpContext, grpcRouteSuffix),
						File:      "http.conf",
					},
					{
						Directive: "server_tokens",
						Value:     "on",
						File:      fmt.Sprintf("%s%s", httpServerContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerContext, grpcRouteSuffix),
						Server:    "*.example.com",
						File:      "http.conf",
					},
					{
						Directive: "tcp_nodelay",
						Value:     "on",
						File:      fmt.Sprintf("%s%s", httpServerLocationContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerLocationContext, grpcRouteSuffix),
						File:      "http.conf",
						Location:  "/helloworld.Greeter/SayHello",
						Server:    "*.example.com",
					},
				}),
			)
		})
	})

	When("SnippetsFilter is invalid", func() {
		Specify("if directives already present in the config are used", func() {
			files := []string{"snippets-filter/invalid-duplicate-sf.yaml"}

			GinkgoWriter.Printf(
				"Applying resources from files with invalid snippetsFilter: %v to namespace %q\n",
				files,
				namespace,
			)
			applyFromFilesErr := resourceManager.ApplyFromFiles(files, namespace)
			if applyFromFilesErr != nil {
				GinkgoWriter.Printf(
					"Error occurred during applying from files %v in namespace %q, error: %s\n",
					files,
					namespace,
					applyFromFilesErr,
				)
			}
			Expect(applyFromFilesErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf("SnippetsFilter applied successfully\n")

			nsname := types.NamespacedName{Name: "tea", Namespace: namespace}
			Eventually(checkHTTPRouteToHaveGatewayNotProgrammedCond).
				WithArguments(nsname).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})

		Specify("if directives are provided in the wrong context", func() {
			files := []string{"snippets-filter/invalid-context-sf.yaml"}

			applyFromFilesErr := resourceManager.ApplyFromFiles(files, namespace)
			if applyFromFilesErr != nil {
				GinkgoWriter.Printf(
					"Error occurred during applying from files %v in namespace %q, error: %s\n",
					files,
					namespace,
					applyFromFilesErr,
				)
			}
			Expect(applyFromFilesErr).ToNot(HaveOccurred())
			GinkgoWriter.Printf("SnippetsFilter applied successfully\n")

			nsname := types.NamespacedName{Name: "soda", Namespace: namespace}
			Eventually(checkHTTPRouteToHaveGatewayNotProgrammedCond).
				WithArguments(nsname).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			GinkgoWriter.Printf("Deleting resources from files: %v in namespace %q\n", files, namespace)
			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
			GinkgoWriter.Printf("Resources from files deleted successfully\n")
		})
	})
})

func checkHTTPRouteToHaveGatewayNotProgrammedCond(httpRouteNsName types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Checking for HTTPRoute %q to have the condition Accepted/True/GatewayNotProgrammed\n",
		httpRouteNsName,
	)

	var hr v1.HTTPRoute
	var err error

	if err = k8sClient.Get(ctx, httpRouteNsName, &hr); err != nil {
		return err
	}

	if len(hr.Status.Parents) != 1 {
		return fmt.Errorf("httproute has %d parent statuses, expected 1", len(hr.Status.Parents))
	}

	parent := hr.Status.Parents[0]
	if parent.Conditions == nil {
		return fmt.Errorf("expected parent conditions to not be nil")
	}

	cond := parent.Conditions[1]
	if cond.Type != string(v1.GatewayConditionAccepted) {
		return fmt.Errorf("expected condition type to be Accepted, got %s", cond.Type)
	}

	if cond.Status != metav1.ConditionFalse {
		return fmt.Errorf("expected condition status to be False, got %s", cond.Status)
	}

	if cond.Reason != string(conditions.RouteReasonGatewayNotProgrammed) {
		return fmt.Errorf("expected condition reason to be GatewayNotProgrammed, got %s", cond.Reason)
	}

	return nil
}

func checkForSnippetsFilterToBeAccepted(snippetsFilterNsNames types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Checking for SnippetsFilter %q to have the condition Accepted/True/Accepted\n",
		snippetsFilterNsNames,
	)

	var sf ngfAPI.SnippetsFilter
	var err error

	if err = k8sClient.Get(ctx, snippetsFilterNsNames, &sf); err != nil {
		return err
	}

	if len(sf.Status.Controllers) != 1 {
		return fmt.Errorf("snippetsFilter has %d controller statuses, expected 1", len(sf.Status.Controllers))
	}

	status := sf.Status.Controllers[0]
	if status.ControllerName != ngfControllerName {
		return fmt.Errorf("expected controller name to be %s, got %s", ngfControllerName, status.ControllerName)
	}

	condition := status.Conditions[0]
	if condition.Type != string(ngfAPI.SnippetsFilterConditionTypeAccepted) {
		return fmt.Errorf("expected condition type to be Accepted, got %s", condition.Type)
	}

	if status.Conditions[0].Status != metav1.ConditionTrue {
		return fmt.Errorf("expected condition status to be %s, got %s", metav1.ConditionTrue, condition.Status)
	}

	if status.Conditions[0].Reason != string(ngfAPI.SnippetsFilterConditionReasonAccepted) {
		return fmt.Errorf(
			"expected condition reason to be %s, got %s",
			ngfAPI.SnippetsFilterConditionReasonAccepted,
			condition.Reason,
		)
	}

	return nil
}
