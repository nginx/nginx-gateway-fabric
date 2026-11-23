package main

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("TLS Certificate Rotation", Ordered,
	Label("functional", "tls-cert-rotation"), func() {
		var (
			baseFiles = []string{
				"tls-cert-rotation/apps.yaml",
				"tls-cert-rotation/gateway.yaml",
				"tls-cert-rotation/routes.yaml",
			}
			namespace    string
			nginxPodName string
		)

		BeforeAll(func() {
			namespace = fmt.Sprintf("tls-cert-rot-%d", GinkgoParallelProcess())
			ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

			applySecret("app1-tls", namespace, "app1.example.com")
			applySecret("shared-tls", namespace, "shared1.example.com", "shared2.example.com")

			Expect(resourceManager.ApplyFromFiles(baseFiles, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

			names, err := resourceManager.GetReadyNginxPodNames(namespace, timeoutConfig.GetStatusTimeout)
			Expect(err).ToNot(HaveOccurred())
			Expect(names).To(HaveLen(1))
			nginxPodName = names[0]
			setUpPortForward(nginxPodName, namespace)
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			cleanUpPortForward()
			Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
		})

		It("serves the new cert after rotation", func() {
			p := helpers.BuildPortFwdPort(443, portFwdHTTPSPort)
			url := helpers.BuildPortFwdURL("https://app1.example.com/", p)
			expect200(url)

			currentCertSerial := getTLSCertSerial("app1.example.com", p)
			applySecret("app1-tls", namespace, "app1.example.com")
			expectCertChanges(p, currentCertSerial)
			expect200(url)
		})

		It("handles consecutive cert rotations", func() {
			p := helpers.BuildPortFwdPort(443, portFwdHTTPSPort)
			url := helpers.BuildPortFwdURL("https://app1.example.com/", p)
			for i := range 3 {
				GinkgoWriter.Printf("Consecutive rotation %d\n", i+1)
				currentCertSerial := getTLSCertSerial("app1.example.com", p)
				applySecret("app1-tls", namespace, "app1.example.com")
				expectCertChanges(p, currentCertSerial)
				expect200(url)
			}
		})

		It("rotates certs independently across listeners", func() {
			p := helpers.BuildPortFwdPort(443, portFwdHTTPSPort)
			s1 := getTLSCertSerial("app1.example.com", p)
			sShared := getTLSCertSerial("shared1.example.com", p)

			applySecret("app1-tls", namespace, "app1.example.com")
			expectCertChanges(p, s1)
			Expect(getTLSCertSerial("shared1.example.com", p).Cmp(sShared)).To(Equal(0),
				"shared cert should not change when app1-tls is rotated")

			expect200(helpers.BuildPortFwdURL("https://app1.example.com/", p))
			expect200(helpers.BuildPortFwdURL("https://shared1.example.com/", p))
		})

		It("serves new cert after delete and recreate", func() {
			p := helpers.BuildPortFwdPort(443, portFwdHTTPSPort)
			before := getTLSCertSerial("app1.example.com", p)

			Expect(resourceManager.DeleteFromFiles([]string{"tls-cert-rotation/routes.yaml"}, namespace)).To(Succeed())
			ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
			defer cancel()
			sec := &core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app1-tls", Namespace: namespace}}
			Expect(resourceManager.Delete(ctx, sec, nil)).To(Succeed())

			applySecret("app1-tls", namespace, "app1.example.com")
			Expect(resourceManager.ApplyFromFiles([]string{"tls-cert-rotation/routes.yaml"}, namespace)).To(Succeed())

			expectCertChanges(p, before)
			expect200(helpers.BuildPortFwdURL("https://app1.example.com/", p))
		})

		It("does not reload NGINX on cert-only rotation", func() {
			p := helpers.BuildPortFwdPort(443, portFwdHTTPSPort)
			url := helpers.BuildPortFwdURL("https://app1.example.com/", p)
			expect200(url)

			// Wait for the controller to finish processing any pending event batches
			// from previous tests before scoping the log check.
			time.Sleep(2 * time.Second)

			// Record the time just before the rotation so we can scope the log check.
			sinceTime := metav1.Now()

			currentCertSerial := getTLSCertSerial("app1.example.com", p)
			applySecret("app1-tls", namespace, "app1.example.com")
			expectCertChanges(p, currentCertSerial)
			expect200(url)

			// Verify the cert was applied, and a content-only update occurred.
			ngfPodNames, err := resourceManager.GetReadyNGFPodNames(
				ngfNamespace, releaseName, timeoutConfig.GetStatusTimeout,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(ngfPodNames).ToNot(BeEmpty())

			logs, err := resourceManager.GetPodLogs(
				ngfNamespace,
				ngfPodNames[0],
				&core.PodLogOptions{
					Container: "nginx-gateway",
					SinceTime: &sinceTime,
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to get NGF controller logs")
			Expect(logs).To(
				ContainSubstring("Sent file contents update to agent"),
				"expected a content-only update for cert rotation",
			)

			namespacedLogs := filterLogsByNamespaceBatches(logs, namespace)
			Expect(namespacedLogs).ToNot(
				ContainSubstring("Sent nginx configuration to agent"),
				"cert-only rotation should not trigger an NGINX reload",
			)
		})

		Context("when listeners share a certificate", func() {
			It("updates cert across all listeners referencing it", func() {
				p := helpers.BuildPortFwdPort(443, portFwdHTTPSPort)

				certSerial1 := getTLSCertSerial("shared1.example.com", p)
				certSerial2 := getTLSCertSerial("shared2.example.com", p)
				Expect(certSerial1.Cmp(certSerial2)).To(
					Equal(0), "both listeners should initially serve the same cert",
				)

				url1 := helpers.BuildPortFwdURL("https://shared1.example.com/", p)
				url2 := helpers.BuildPortFwdURL("https://shared2.example.com/", p)
				expect200(url1)
				expect200(url2)

				applySecret("shared-tls", namespace, "shared1.example.com", "shared2.example.com")

				// Capture the current serial before rotation.
				current := certSerial1
				Eventually(func(g Gomega) {
					// Fresh serials each poll iteration. These intentionally shadow the variables
					// outside the scope of the Eventually function to capture the pre-rotation serial.
					certSerial1 := getTLSCertSerial("shared1.example.com", p)
					certSerial2 := getTLSCertSerial("shared2.example.com", p)
					g.Expect(certSerial1.Cmp(current)).ToNot(Equal(0), "shared1 serial should change")
					g.Expect(certSerial2.Cmp(current)).ToNot(Equal(0), "shared2 serial should change")
					g.Expect(certSerial1.Cmp(certSerial2)).To(Equal(0), "both should serve same new cert")
				}).
					WithTimeout(timeoutConfig.UpdateTimeout).
					WithPolling(1 * time.Second).
					Should(Succeed())

				expect200(url1)
				expect200(url2)
			})
		})
	})

// applySecret generates a self-signed cert for the given hostnames and creates/updates the TLS Secret.
func applySecret(name, ns string, hostnames ...string) {
	ca, err := framework.GenerateSelfSignedCACert("test-ca")
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	cert, err := framework.GenerateSignedServerCert(ca, hostnames[0], hostnames)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	s := framework.CreateTLSSecret(name, ns, cert.CertPEM, cert.KeyPEM)
	ExpectWithOffset(1, resourceManager.Apply([]client.Object{s})).To(Succeed())
}

// getTLSCertSerial returns the leaf certificate serial number from a live TLS handshake.
func getTLSCertSerial(serverName string, port int) *big.Int {
	var serial *big.Int
	EventuallyWithOffset(1, func(g Gomega) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s, err := framework.GetTLSCertSerialNumber(ctx, address, port, serverName)
		g.Expect(err).ToNot(HaveOccurred())
		serial = s
	}).
		WithTimeout(timeoutConfig.RequestTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	return serial
}

// expectCertChanges polls until the served certificate serial differs from prev.
func expectCertChanges(port int, prev *big.Int) {
	Eventually(func(g Gomega) {
		serial := getTLSCertSerial("app1.example.com", port)
		g.Expect(serial.Cmp(prev)).ToNot(Equal(0), "cert serial should change")
	}).
		WithTimeout(timeoutConfig.UpdateTimeout).
		WithPolling(1 * time.Second).
		Should(Succeed())
}

// expect200 sends an HTTPS GET and expects a 200 OK.
func expect200(url string) {
	Eventually(func(g Gomega) {
		req := framework.Request{URL: url, Address: address, Timeout: timeoutConfig.RequestTimeout}
		resp, err := framework.Get(req)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
	}).
		WithTimeout(timeoutConfig.RequestTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}

// filterLogsByNamespaceBatches extracts log lines from event batches
// that reference the given namespace.
func filterLogsByNamespaceBatches(logs, namespace string) string {
	lines := strings.Split(logs, "\n")

	type batch struct {
		lines        []string
		hasNamespace bool
	}

	var batches []batch
	var current *batch

	for _, line := range lines {
		if strings.Contains(line, "Started processing event batch") {
			batches = append(batches, batch{})
			current = &batches[len(batches)-1]
		}

		if current != nil {
			current.lines = append(current.lines, line)
			if strings.Contains(line, namespace) {
				current.hasNamespace = true
			}
			if strings.Contains(line, "Finished processing event batch") {
				current = nil
			}
		}
	}

	var result strings.Builder
	for _, b := range batches {
		if b.hasNamespace {
			for _, line := range b.lines {
				result.WriteString(line)
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}
