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

			// Each parallel proc runs its own isolated NGF controller (release
			// "ngf-test-<proc>" in namespace "nginx-gateway-<proc>"), and this test reads
			// only that controller's logs. The flake is therefore NOT cross-proc: it comes
			// from other operations on the SAME proc that share this one controller and the
			// single per-proc namespace ("tls-cert-rot-<proc>"). Earlier TLS specs in this
			// Ordered container (and background re-syncs) trigger legitimate reloads whose
			// log lines can overlap this check's window. A namespace-substring filter cannot
			// separate those, since all same-proc activity shares the namespace; likewise the
			// high-level "Sent nginx configuration to agent" line carries no correlation data
			// (no correlationId, namespace, or filename) and is un-attributable.
			//
			// Instead, correlate on the specific secret's TLS keypair filename: find the
			// correlationIds that referenced our keypair file, then assert none of them
			// triggered a reload (ConfigApplyRequest). A cert-only rotation must only produce
			// a content update (ContentUpdateRequest) for that correlationId.
			keypairFilename := fmt.Sprintf("ssl_keypair_%s_app1-tls.pem", namespace)
			Expect(reloadCorrelatedWithSecret(logs, keypairFilename)).To(
				BeFalse(),
				"cert-only rotation should not trigger an NGINX reload for this secret",
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

// logFieldValue extracts the value for a logr/zap-style structured field key from a
// single log line. It supports both JSON-encoded values ("key": "value" or "key":value)
// and plain key=value / "key" "value" encodings so the helper is resilient to the
// controller's log format. It returns the value with surrounding quotes trimmed and
// whether the key was found.
func logFieldValue(line, key string) (string, bool) {
	// Try JSON-style: "key":"value" or "key":value
	for _, sep := range []string{fmt.Sprintf("%q:", key), fmt.Sprintf("%q :", key)} {
		if idx := strings.Index(line, sep); idx != -1 {
			rest := strings.TrimSpace(line[idx+len(sep):])
			return trimLogValue(rest), true
		}
	}
	// Try logfmt-style: key=value
	if v, ok := scanKeyValue(line, key+"="); ok {
		return v, true
	}
	// Try spaced style: "key" "value" or key "value"
	for _, k := range []string{fmt.Sprintf("%q ", key), key + " "} {
		if v, ok := scanKeyValue(line, k); ok {
			return v, true
		}
	}
	return "", false
}

// scanKeyValue finds the token following the given prefix and returns its trimmed value.
func scanKeyValue(line, prefix string) (string, bool) {
	idx := strings.Index(line, prefix)
	if idx == -1 {
		return "", false
	}
	rest := strings.TrimSpace(line[idx+len(prefix):])
	return trimLogValue(rest), true
}

// trimLogValue extracts the first whitespace- or comma-delimited token from s and trims
// any surrounding quotes.
func trimLogValue(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '"' {
		if end := strings.IndexByte(s[1:], '"'); end != -1 {
			return s[1 : 1+end]
		}
	}
	end := strings.IndexAny(s, " ,}\t")
	if end == -1 {
		return strings.Trim(s, `"`)
	}
	return strings.Trim(s[:end], `"`)
}

// reloadCorrelatedWithSecret reports whether an NGINX reload (ConfigApplyRequest) was
// correlated with the given TLS keypair filename.
//
// Correlation is established via the controller's structured logs:
//   - "Getting file for agent" lines carry both a "file" field and a "correlation_id";
//     these identify the requests that referenced our specific secret.
//   - "Sending configuration to agent" lines (the reload path) carry a "correlationId";
//     if any of them match a correlationId that referenced our keypair file, a reload
//     was triggered for this secret.
//   - "Sending file content update to agent" lines (the content-only path) are expected
//     for a cert rotation and are intentionally ignored.
//
// This avoids relying on the high-level "Sent nginx configuration to agent" line, which
// carries no correlation data and is therefore un-attributable. It also avoids filtering
// by namespace, which cannot isolate this secret's reload from other reloads on the same
// proc (every spec on a proc shares one controller and one "tls-cert-rot-<proc>"
// namespace).
func reloadCorrelatedWithSecret(logs, keypairFilename string) bool {
	lines := strings.Split(logs, "\n")

	// Step A: collect correlationIds that referenced our keypair file.
	secretCorrelationIDs := make(map[string]struct{})
	for _, line := range lines {
		if !strings.Contains(line, "Getting file for agent") {
			continue
		}
		file, ok := logFieldValue(line, "file")
		if !ok || !strings.Contains(file, keypairFilename) {
			continue
		}
		if id, ok := logFieldValue(line, "correlation_id"); ok && id != "" {
			secretCorrelationIDs[id] = struct{}{}
		}
	}

	if len(secretCorrelationIDs) == 0 {
		return false
	}

	// Step B: check whether any reload (ConfigApplyRequest) used one of those IDs.
	for _, line := range lines {
		if !strings.Contains(line, "Sending configuration to agent") {
			continue
		}
		id, ok := logFieldValue(line, "correlationId")
		if !ok {
			continue
		}
		if _, matched := secretCorrelationIDs[id]; matched {
			return true
		}
	}

	return false
}
