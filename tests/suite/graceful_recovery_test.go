package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/batch/v1"
	coordination "k8s.io/api/coordination/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

const (
	nginxContainerName = "nginx"
	ngfContainerName   = "nginx-gateway"
)

// Since checkNGFContainerLogsForErrors may experience interference from previous tests (as explained in the function
// documentation), this test is recommended to be run separate from other tests.
var _ = Describe("Graceful Recovery test", Ordered, Label("graceful-recovery"), func() {
	files := []string{
		"graceful-recovery/cafe.yaml",
		"graceful-recovery/cafe-secret.yaml",
		"graceful-recovery/gateway.yaml",
		"graceful-recovery/cafe-routes.yaml",
	}

	var ns core.Namespace

	baseHTTPURL := "http://cafe.example.com"
	baseHTTPSURL := "https://cafe.example.com"
	teaURL := baseHTTPSURL + "/tea"
	coffeeURL := baseHTTPURL + "/coffee"

	var ngfPodName string

	BeforeEach(func() {
		// this test is unique in that it will check the entire log of both ngf and nginx containers
		// for any errors, so in order to avoid errors generated in previous tests we will uninstall
		// NGF installed at the suite level, then re-deploy our own. We will also uninstall and re-install
		// NGF between each graceful-recovery test for the same reason.
		teardown(releaseName)

		setup(getDefaultSetupCfg())

		podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(podNames).To(HaveLen(1))

		ngfPodName = podNames[0]
		if portFwdPort != 0 {
			coffeeURL = fmt.Sprintf("%s:%d/coffee", baseHTTPURL, portFwdPort)
		}
		if portFwdHTTPSPort != 0 {
			teaURL = fmt.Sprintf("%s:%d/tea", baseHTTPSURL, portFwdHTTPSPort)
		}

		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "graceful-recovery",
			},
		}

		Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReadyWithPodCount(ns.Name, 2)).To(Succeed())

		Eventually(
			func() error {
				return checkForWorkingTraffic(teaURL, coffeeURL)
			}).
			WithTimeout(timeoutConfig.TestForTrafficTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())
	})

	AfterAll(func() {
		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())
	})

	It("recovers when NGF container is restarted", func() {
		runRecoveryTest(teaURL, coffeeURL, ngfPodName, ngfContainerName, files, &ns)
	})

	It("recovers when nginx container is restarted", func() {
		runRecoveryTest(teaURL, coffeeURL, ngfPodName, nginxContainerName, files, &ns)
	})

	It("recovers when drained node is restarted", func() {
		runRestartNodeWithDrainingTest(teaURL, coffeeURL, files, &ns)
	})

	It("recovers when node is restarted abruptly", func() {
		runRestartNodeAbruptlyTest(teaURL, coffeeURL, files, &ns)
	})
})

func runRestartNodeWithDrainingTest(teaURL, coffeeURL string, files []string, ns *core.Namespace) {
	runRestartNodeTest(teaURL, coffeeURL, files, ns, true)
}

func runRestartNodeAbruptlyTest(teaURL, coffeeURL string, files []string, ns *core.Namespace) {
	runRestartNodeTest(teaURL, coffeeURL, files, ns, false)
}

func runRestartNodeTest(teaURL, coffeeURL string, files []string, ns *core.Namespace, drain bool) {
	nodeNames, err := getNodeNames()
	Expect(err).ToNot(HaveOccurred())
	Expect(nodeNames).To(HaveLen(1))

	kindNodeName := nodeNames[0]

	Expect(clusterName).ToNot(BeNil(), "clusterName variable not set")
	Expect(*clusterName).ToNot(BeEmpty())
	containerName := *clusterName + "-control-plane"

	if portFwdPort != 0 {
		close(portForwardStopCh)
	}

	if drain {
		output, err := exec.Command(
			"kubectl",
			"drain",
			kindNodeName,
			"--ignore-daemonsets",
			"--delete-emptydir-data",
		).CombinedOutput()

		Expect(err).ToNot(HaveOccurred(), string(output))

		output, err = exec.Command("kubectl", "delete", "node", kindNodeName).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
	}

	_, err = exec.Command("docker", "restart", containerName).CombinedOutput()
	Expect(err).ToNot(HaveOccurred())

	// need to wait for docker container to restart and be running before polling for ready NGF Pods or else we will error
	Eventually(
		func() bool {
			output, err := exec.Command(
				"docker",
				"inspect",
				"-f",
				"{{.State.Running}}",
				containerName,
			).CombinedOutput()
			return strings.TrimSpace(string(output)) == "true" && err == nil
		}).
		WithTimeout(timeoutConfig.CreateTimeout).
		WithPolling(500 * time.Millisecond).
		Should(BeTrue())

	// ngf can often oscillate between ready and error, so we wait for a stable readiness in ngf
	var podNames []string
	Eventually(
		func() bool {
			podNames, err = framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetStatusTimeout)
			return len(podNames) == 1 && err == nil
		}).
		WithTimeout(timeoutConfig.CreateTimeout * 2).
		WithPolling(500 * time.Millisecond).
		MustPassRepeatedly(20).
		Should(BeTrue())

	ngfPodName := podNames[0]
	Expect(ngfPodName).ToNot(BeEmpty())

	if portFwdPort != 0 {
		ports := []string{fmt.Sprintf("%d:80", ngfHTTPForwardedPort), fmt.Sprintf("%d:443", ngfHTTPSForwardedPort)}
		portForwardStopCh = make(chan struct{})
		err = framework.PortForward(ctlr.GetConfigOrDie(), ngfNamespace, ngfPodName, ports, portForwardStopCh)
		Expect(err).ToNot(HaveOccurred())
	}

	checkNGFFunctionality(teaURL, coffeeURL, ngfPodName, "", files, ns)
	if errorLogs := getUnexpectedNginxErrorLogs(ngfPodName); errorLogs != "" {
		Skip(fmt.Sprintf("NGINX has unexpected error logs: \n%s", errorLogs))
	}
}

func runRecoveryTest(teaURL, coffeeURL, ngfPodName, containerName string, files []string, ns *core.Namespace) {
	var (
		err       error
		leaseName string
	)

	if containerName != nginxContainerName {
		// Since we have already deployed resources and ran resourceManager.WaitForAppsToBeReadyWithPodCount earlier,
		// we know that the applications are ready at this point. This could only be the case if NGF has written
		// statuses, which could only be the case if NGF has the leader lease. Since there is only one instance
		// of NGF in this test, we can be certain that this is the correct leaseholder name.
		leaseName, err = getLeaderElectionLeaseHolderName()
		Expect(err).ToNot(HaveOccurred())
	}

	restartContainer(ngfPodName, containerName)

	if containerName != nginxContainerName {
		Eventually(
			func() error {
				return checkLeaderLeaseChange(leaseName)
			}).
			WithTimeout(timeoutConfig.GetLeaderLeaseTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())
	}

	checkNGFFunctionality(teaURL, coffeeURL, ngfPodName, containerName, files, ns)
	if errorLogs := getUnexpectedNginxErrorLogs(ngfPodName); errorLogs != "" {
		Skip(fmt.Sprintf("NGINX has unexpected error logs: \n%s", errorLogs))
	}
}

func restartContainer(ngfPodName, containerName string) {
	var jobScript string
	if containerName == "nginx" {
		jobScript = "PID=$(pgrep -f \"nginx: master process\") && kill -9 $PID"
	} else {
		jobScript = "PID=$(pgrep -f \"/usr/bin/gateway\") && kill -9 $PID"
	}

	restartCount, err := getContainerRestartCount(ngfPodName, containerName)
	Expect(err).ToNot(HaveOccurred())

	job, err := runNodeDebuggerJob(ngfPodName, jobScript)
	Expect(err).ToNot(HaveOccurred())

	Eventually(
		func() error {
			return checkContainerRestart(ngfPodName, containerName, restartCount)
		}).
		WithTimeout(timeoutConfig.ContainerRestartTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	// default propagation policy is metav1.DeletePropagationOrphan which does not delete the underlying
	// pod created through the job after the job is deleted. Setting it to metav1.DeletePropagationBackground
	// deletes the underlying pod after the job is deleted.
	Expect(resourceManager.Delete(
		[]client.Object{job},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)).To(Succeed())
}

func checkContainerRestart(ngfPodName, containerName string, currentRestartCount int) error {
	restartCount, err := getContainerRestartCount(ngfPodName, containerName)
	if err != nil {
		return err
	}

	if restartCount != currentRestartCount+1 {
		return fmt.Errorf("expected current restart count: %d to match incremented restart count: %d",
			restartCount, currentRestartCount+1)
	}

	return nil
}

func checkForWorkingTraffic(teaURL, coffeeURL string) error {
	if err := expectRequestToSucceed(teaURL, address, "URI: /tea"); err != nil {
		return err
	}
	if err := expectRequestToSucceed(coffeeURL, address, "URI: /coffee"); err != nil {
		return err
	}
	return nil
}

func checkForFailingTraffic(teaURL, coffeeURL string) error {
	if err := expectRequestToFail(teaURL, address); err != nil {
		return err
	}
	if err := expectRequestToFail(coffeeURL, address); err != nil {
		return err
	}
	return nil
}

func expectRequestToSucceed(appURL, address string, responseBodyMessage string) error {
	status, body, err := framework.Get(appURL, address, timeoutConfig.RequestTimeout)

	if status != http.StatusOK {
		return errors.New("http status was not 200")
	}

	if !strings.Contains(body, responseBodyMessage) {
		return errors.New("expected response body to contain correct body message")
	}

	return err
}

func expectRequestToFail(appURL, address string) error {
	status, body, err := framework.Get(appURL, address, timeoutConfig.RequestTimeout)
	if status != 0 {
		return errors.New("expected http status to be 0")
	}

	if body != "" {
		return fmt.Errorf("expected response body to be empty, instead received: %s", body)
	}

	if err == nil {
		return errors.New("expected request to error")
	}

	return nil
}

func checkNGFFunctionality(teaURL, coffeeURL, ngfPodName, containerName string, files []string, ns *core.Namespace) {
	Eventually(
		func() error {
			return checkForWorkingTraffic(teaURL, coffeeURL)
		}).
		WithTimeout(timeoutConfig.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())

	Eventually(
		func() error {
			return checkForFailingTraffic(teaURL, coffeeURL)
		}).
		WithTimeout(timeoutConfig.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
	Expect(resourceManager.WaitForAppsToBeReadyWithPodCount(ns.Name, 2)).To(Succeed())

	Eventually(
		func() error {
			return checkForWorkingTraffic(teaURL, coffeeURL)
		}).
		WithTimeout(timeoutConfig.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	// When the NGINX process is killed, some errors are expected in the NGF logs while we wait for the
	// NGINX container to be restarted. Therefore, we don't want to check the NGF logs for errors when the container
	// we restarted was NGINX.
	if containerName != nginxContainerName {
		checkNGFContainerLogsForErrors(ngfPodName)
	}
}

func getNginxErrorLogs(ngfPodName string) string {
	nginxLogs, err := resourceManager.GetPodLogs(
		ngfNamespace,
		ngfPodName,
		&core.PodLogOptions{Container: nginxContainerName},
	)
	Expect(err).ToNot(HaveOccurred())

	errPrefixes := []string{
		framework.CritNGINXLog,
		framework.ErrorNGINXLog,
		framework.WarnNGINXLog,
		framework.AlertNGINXLog,
		framework.EmergNGINXLog,
	}
	errorLogs := ""

	for _, line := range strings.Split(nginxLogs, "\n") {
		for _, prefix := range errPrefixes {
			if strings.Contains(line, prefix) {
				errorLogs += line + "\n"
				break
			}
		}
	}

	return errorLogs
}

func getUnexpectedNginxErrorLogs(ngfPodName string) string {
	expectedErrStrings := []string{
		"connect() failed (111: Connection refused)",
		"could not be resolved (host not found) during usage report",
		"server returned 429",
		// FIXME(salonichf5) remove this error message check
		// when https://github.com/nginx/nginx-gateway-fabric/issues/2090 is completed.
		"no live upstreams while connecting to upstream",
	}

	unexpectedErrors := ""

	errorLogs := getNginxErrorLogs(ngfPodName)

	for _, line := range strings.Split(errorLogs, "\n") {
		if !slices.ContainsFunc(expectedErrStrings, func(s string) bool {
			return strings.Contains(line, s)
		}) {
			unexpectedErrors += line
		}
	}

	return unexpectedErrors
}

// checkNGFContainerLogsForErrors checks NGF container's logs for any possible errors.
func checkNGFContainerLogsForErrors(ngfPodName string) {
	ngfLogs, err := resourceManager.GetPodLogs(
		ngfNamespace,
		ngfPodName,
		&core.PodLogOptions{Container: ngfContainerName},
	)
	Expect(err).ToNot(HaveOccurred())

	for _, line := range strings.Split(ngfLogs, "\n") {
		Expect(line).ToNot(ContainSubstring("\"level\":\"error\""), line)
	}
}

func checkLeaderLeaseChange(originalLeaseName string) error {
	leaseName, err := getLeaderElectionLeaseHolderName()
	if err != nil {
		return err
	}

	if originalLeaseName == leaseName {
		return fmt.Errorf("expected originalLeaseName: %s, to not match current leaseName: %s", originalLeaseName, leaseName)
	}

	return nil
}

func getLeaderElectionLeaseHolderName() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var lease coordination.Lease
	key := types.NamespacedName{Name: "ngf-test-nginx-gateway-fabric-leader-election", Namespace: ngfNamespace}

	if err := k8sClient.Get(ctx, key, &lease); err != nil {
		return "", errors.New("could not retrieve leader election lease")
	}

	if *lease.Spec.HolderIdentity == "" {
		return "", errors.New("leader election lease holder identity is empty")
	}

	return *lease.Spec.HolderIdentity, nil
}

func getContainerRestartCount(ngfPodName, containerName string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var ngfPod core.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ngfNamespace, Name: ngfPodName}, &ngfPod); err != nil {
		return 0, fmt.Errorf("error retrieving NGF Pod: %w", err)
	}

	var restartCount int
	for _, containerStatus := range ngfPod.Status.ContainerStatuses {
		if containerStatus.Name == containerName {
			restartCount = int(containerStatus.RestartCount)
		}
	}

	return restartCount, nil
}

func getNodeNames() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()
	var nodes core.NodeList

	if err := k8sClient.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("error listing nodes: %w", err)
	}

	names := make([]string, 0, len(nodes.Items))

	for _, node := range nodes.Items {
		names = append(names, node.Name)
	}

	return names, nil
}

func runNodeDebuggerJob(ngfPodName, jobScript string) (*v1.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var ngfPod core.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ngfNamespace, Name: ngfPodName}, &ngfPod); err != nil {
		return nil, fmt.Errorf("error retrieving NGF Pod: %w", err)
	}

	b, err := resourceManager.GetFileContents("graceful-recovery/node-debugger-job.yaml")
	if err != nil {
		return nil, fmt.Errorf("error processing node debugger job file: %w", err)
	}

	job := &v1.Job{}
	if err = yaml.Unmarshal(b.Bytes(), job); err != nil {
		return nil, fmt.Errorf("error with yaml unmarshal: %w", err)
	}

	job.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"] = ngfPod.Spec.NodeName
	if len(job.Spec.Template.Spec.Containers) != 1 {
		return nil, fmt.Errorf(
			"expected node debugger job to contain one container, actual number: %d",
			len(job.Spec.Template.Spec.Containers),
		)
	}
	job.Spec.Template.Spec.Containers[0].Args = []string{jobScript}
	job.Namespace = ngfNamespace

	if err = resourceManager.Apply([]client.Object{job}); err != nil {
		return nil, fmt.Errorf("error in applying job: %w", err)
	}

	return job, nil
}
