package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("ClientSettingsPolicy Listener-level attachment", Ordered, Label("functional", "cspolicy-listener"),
	func() {
		var (
			files = []string{
				"clientsettings-listener/backends.yaml",
				"clientsettings-listener/gateway.yaml",
				"clientsettings-listener/routes.yaml",
			}

			namespace    string
			nginxPodName string
		)

		BeforeAll(func() {
			namespace = fmt.Sprintf("cs-listener-%d", GinkgoParallelProcess())

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

		When("a gateway-level and a listener-level CSP are created", func() {
			var policies = []string{
				"clientsettings-listener/policies.yaml",
			}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(policies, namespace)).To(Succeed())
			})

			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(policies, namespace)).To(Succeed())
			})

			Specify("both policies are accepted", func() {
				for _, name := range []string{"gw-csp", "listener-a-csp"} {
					nsname := types.NamespacedName{Name: name, Namespace: namespace}
					err := waitForCSPolicyStatus(nsname, framework.PolicyAccepted)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
				}
			})

			Context("nginx config", func() {
				var (
					conf       *framework.Payload
					filePrefix string
				)

				BeforeAll(func() {
					filePrefix = fmt.Sprintf("/etc/nginx/includes/ClientSettingsPolicy_%s", namespace)
					conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
				})

				It("includes the gateway policy only on listener-b's server block (not overridden)", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     fmt.Sprintf("%s_gw-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "b.example.com",
					})).To(Succeed())
				})

				It("includes the listener-a policy on listener-a's server block (overrides gateway policy)", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     fmt.Sprintf("%s_listener-a-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "a.example.com",
					})).To(Succeed())
				})

				It("sets the correct client_max_body_size for listener-a (overridden to 5000)", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "client_max_body_size",
						Value:     "5000",
						File:      fmt.Sprintf("%s_listener-a-csp.conf", filePrefix),
					})).To(Succeed())
				})

				It("sets the gateway-level client_max_body_size to 1000", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "client_max_body_size",
						Value:     "1000",
						File:      fmt.Sprintf("%s_gw-csp.conf", filePrefix),
					})).To(Succeed())
				})
			})

			Context("traffic", func() {
				DescribeTable("client_max_body_size is enforced per listener",
					func(host, uri string, bodySize, expStatus int) {
						port := helpers.BuildPortFwdPort(80, portFwdPort)
						url := helpers.BuildPortFwdURL(host, port) + uri

						payload := make([]byte, bodySize)
						_, err := rand.Read(payload)
						Expect(err).ToNot(HaveOccurred())

						request := framework.Request{
							URL:     url,
							Address: address,
							Body:    bytes.NewReader(payload),
							Timeout: timeoutConfig.RequestTimeout,
						}
						Eventually(func() int {
							resp, err := framework.Post(request)
							if err != nil {
								return 0
							}
							return resp.StatusCode
						}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(500 * time.Millisecond).
							Should(Equal(expStatus))
					},
					func(host, uri string, bodySize, expStatus int) string {
						return fmt.Sprintf(
							"%s%s with %d bytes should return %d",
							host, uri, bodySize, expStatus,
						)
					},
					// listener-a has listener-level override: maxSize=5000
					Entry(nil, "a.example.com", "/", 4000, http.StatusOK),
					Entry(nil, "a.example.com", "/", 6000, http.StatusRequestEntityTooLarge),
					// listener-b inherits gateway-level: maxSize=1000
					Entry(nil, "b.example.com", "/", 800, http.StatusOK),
					Entry(nil, "b.example.com", "/", 1200, http.StatusRequestEntityTooLarge),
				)
			})
		})

		When("a CSP targets a ListenerSet listener via SectionName", func() {
			var lsFiles = []string{
				"clientsettings-listener/listenerset.yaml",
				"clientsettings-listener/ls-route.yaml",
				"clientsettings-listener/ls-policies.yaml",
			}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(lsFiles, namespace)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
			})

			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(lsFiles, namespace)).To(Succeed())
			})

			Specify("the policy is accepted", func() {
				nsname := types.NamespacedName{Name: "ls-listener-csp", Namespace: namespace}
				err := waitForCSPolicyStatus(nsname, framework.PolicyAccepted)
				Expect(err).ToNot(HaveOccurred(), "ls-listener-csp was not accepted")
			})

			Context("nginx config", func() {
				var (
					conf       *framework.Payload
					filePrefix string
				)

				BeforeAll(func() {
					filePrefix = fmt.Sprintf("/etc/nginx/includes/ClientSettingsPolicy_%s", namespace)
					conf = framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)
				})

				It("includes the ListenerSet listener policy on the ls.example.com server block", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "include",
						Value:     fmt.Sprintf("%s_ls-listener-csp.conf", filePrefix),
						File:      "http.conf",
						Server:    "ls.example.com",
					})).To(Succeed())
				})

				It("sets client_max_body_size to 8000 for the ListenerSet listener", func() {
					Expect(framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "client_max_body_size",
						Value:     "8000",
						File:      fmt.Sprintf("%s_ls-listener-csp.conf", filePrefix),
					})).To(Succeed())
				})
			})

			Context("traffic", func() {
				DescribeTable("client_max_body_size is enforced on the ListenerSet listener",
					func(bodySize, expStatus int) {
						port := helpers.BuildPortFwdPort(80, portFwdPort)
						url := helpers.BuildPortFwdURL("ls.example.com", port) + "/"

						payload := make([]byte, bodySize)
						_, err := rand.Read(payload)
						Expect(err).ToNot(HaveOccurred())

						request := framework.Request{
							URL:     url,
							Address: address,
							Body:    bytes.NewReader(payload),
							Timeout: timeoutConfig.RequestTimeout,
						}
						Eventually(func() int {
							resp, err := framework.Post(request)
							if err != nil {
								return 0
							}
							return resp.StatusCode
						}).
							WithTimeout(timeoutConfig.RequestTimeout).
							WithPolling(500 * time.Millisecond).
							Should(Equal(expStatus))
					},
					func(bodySize, expStatus int) string {
						return fmt.Sprintf(
							"ls.example.com with %d bytes should return %d",
							bodySize, expStatus,
						)
					},
					Entry(nil, 7000, http.StatusOK),
					Entry(nil, 9000, http.StatusRequestEntityTooLarge),
				)
			})
		})

		When("a CSP targets a nonexistent listener via SectionName", func() {
			var invalidFiles = []string{
				"clientsettings-listener/invalid-section-csp.yaml",
			}

			BeforeAll(func() {
				Expect(resourceManager.ApplyFromFiles(invalidFiles, namespace)).To(Succeed())
			})

			AfterAll(func() {
				Expect(resourceManager.DeleteFromFiles(invalidFiles, namespace)).To(Succeed())
			})

			Specify("the policy's accepted condition is set to TargetNotFound", func() {
				nsname := types.NamespacedName{Name: "invalid-section-csp", Namespace: namespace}
				Expect(waitForCSPolicyStatus(nsname, framework.PolicyCondition{
					Status: metav1.ConditionFalse,
					Reason: v1.PolicyReasonTargetNotFound,
				})).To(Succeed())
			})
		})
	},
)
