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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

const (
	nginxContainerName = "nginx"
	ngfContainerName   = "nginx-gateway"
)

// Since this test involves restarting of the test node, it is recommended to be run separate from other tests
// such that any issues in this test do not interfere with other tests.
var _ = Describe("Graceful Recovery test", Ordered, Label("graceful-recovery"), func() {
	var (
		files = []string{
			"graceful-recovery/cafe.yaml",
			"graceful-recovery/cafe-secret.yaml",
			"graceful-recovery/gateway.yaml",
			"graceful-recovery/cafe-routes.yaml",
		}

		ns core.Namespace

		baseHTTPURL  = "http://cafe.example.com"
		baseHTTPSURL = "https://cafe.example.com"
		teaURL       = baseHTTPSURL + "/tea"
		coffeeURL    = baseHTTPURL + "/coffee"

		activeNGFPodName, activeNginxPodName string
	)

	checkForWorkingTraffic := func(teaURL, coffeeURL string) error {
		if err := expectRequestToSucceed(teaURL, address, "URI: /tea"); err != nil {
			return err
		}
		if err := expectRequestToSucceed(coffeeURL, address, "URI: /coffee"); err != nil {
			return err
		}
		return nil
	}

	checkForFailingTraffic := func(teaURL, coffeeURL string) error {
		if err := expectRequestToFail(teaURL, address); err != nil {
			return err
		}
		if err := expectRequestToFail(coffeeURL, address); err != nil {
			return err
		}
		return nil
	}

	getContainerRestartCount := func(podName, namespace, containerName string) (int, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
		defer cancel()

		var pod core.Pod
		if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
			return 0, fmt.Errorf("error retrieving Pod: %w", err)
		}

		var restartCount int
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name == containerName {
				restartCount = int(containerStatus.RestartCount)
			}
		}

		return restartCount, nil
	}

	checkContainerRestart := func(podName, containerName, namespace string, currentRestartCount int) error {
		restartCount, err := getContainerRestartCount(podName, namespace, containerName)
		if err != nil {
			return err
		}

		if restartCount != currentRestartCount+1 {
			return fmt.Errorf("expected current restart count: %d to match incremented restart count: %d",
				restartCount, currentRestartCount+1)
		}

		return nil
	}

	getNodeNames := func() ([]string, error) {
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

	runNodeDebuggerJob := func(nginxPodName, jobScript string) (*v1.Job, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
		defer cancel()

		var nginxPod core.Pod
		if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns.Name, Name: nginxPodName}, &nginxPod); err != nil {
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

		job.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"] = nginxPod.Spec.NodeName
		if len(job.Spec.Template.Spec.Containers) != 1 {
			return nil, fmt.Errorf(
				"expected node debugger job to contain one container, actual number: %d",
				len(job.Spec.Template.Spec.Containers),
			)
		}
		job.Spec.Template.Spec.Containers[0].Args = []string{jobScript}
		job.Namespace = ns.Name

		if err = resourceManager.Apply([]client.Object{job}); err != nil {
			return nil, fmt.Errorf("error in applying job: %w", err)
		}

		return job, nil
	}

	restartNginxContainer := func(nginxPodName, namespace, containerName string) {
		jobScript := "PID=$(pgrep -f \"nginx-agent\") && kill -9 $PID"

		restartCount, err := getContainerRestartCount(nginxPodName, namespace, containerName)
		Expect(err).ToNot(HaveOccurred())

		cleanUpPortForward()
		job, err := runNodeDebuggerJob(nginxPodName, jobScript)
		Expect(err).ToNot(HaveOccurred())

		Eventually(
			func() error {
				return checkContainerRestart(nginxPodName, containerName, namespace, restartCount)
			}).
			WithTimeout(timeoutConfig.CreateTimeout).
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

	checkNGFFunctionality := func(teaURL, coffeeURL string, files []string, ns *core.Namespace) {
		Eventually(
			func() error {
				return checkForWorkingTraffic(teaURL, coffeeURL)
			}).
			WithTimeout(timeoutConfig.TestForTrafficTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())

		cleanUpPortForward()
		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())

		Eventually(
			func() error {
				return checkForFailingTraffic(teaURL, coffeeURL)
			}).
			WithTimeout(timeoutConfig.TestForTrafficTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())

		var nginxPodNames []string
		var err error
		Eventually(
			func() bool {
				nginxPodNames, err = framework.GetReadyNginxPodNames(k8sClient, ns.Name, timeoutConfig.GetStatusTimeout)
				return len(nginxPodNames) == 1 && err == nil
			}).
			WithTimeout(timeoutConfig.CreateTimeout).
			WithPolling(500 * time.Millisecond).
			MustPassRepeatedly(10).
			Should(BeTrue())

		nginxPodName := nginxPodNames[0]
		Expect(nginxPodName).ToNot(BeEmpty())
		activeNginxPodName = nginxPodName

		setUpPortForward(activeNginxPodName, ns.Name)

		Eventually(
			func() error {
				return checkForWorkingTraffic(teaURL, coffeeURL)
			}).
			WithTimeout(timeoutConfig.TestForTrafficTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())
	}

	runRestartNodeTest := func(teaURL, coffeeURL string, files []string, ns *core.Namespace, drain bool) {
		nodeNames, err := getNodeNames()
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeNames).To(HaveLen(1))

		kindNodeName := nodeNames[0]

		Expect(clusterName).ToNot(BeNil(), "clusterName variable not set")
		Expect(*clusterName).ToNot(BeEmpty())
		containerName := *clusterName + "-control-plane"

		cleanUpPortForward()

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
				podNames, err = framework.GetReadyNGFPodNames(
					k8sClient,
					ngfNamespace,
					releaseName,
					timeoutConfig.GetStatusTimeout,
				)
				return len(podNames) == 1 && err == nil
			}).
			WithTimeout(timeoutConfig.CreateTimeout * 2).
			WithPolling(500 * time.Millisecond).
			MustPassRepeatedly(20).
			Should(BeTrue())
		newNGFPodName := podNames[0]

		// expected behavior is when node is drained, new pods will be created. when the node is
		// abruptly restarted, new pods are not created.
		if drain {
			Expect(newNGFPodName).ToNot(Equal(activeNGFPodName))
			activeNGFPodName = newNGFPodName
		} else {
			Expect(newNGFPodName).To(Equal(activeNGFPodName))
		}

		var nginxPodNames []string
		Eventually(
			func() bool {
				nginxPodNames, err = framework.GetReadyNginxPodNames(k8sClient, ns.Name, timeoutConfig.GetStatusTimeout)
				return len(nginxPodNames) == 1 && err == nil
			}).
			WithTimeout(timeoutConfig.CreateTimeout * 2).
			WithPolling(500 * time.Millisecond).
			MustPassRepeatedly(20).
			Should(BeTrue())
		newNginxPodName := nginxPodNames[0]

		if drain {
			Expect(newNginxPodName).ToNot(Equal(activeNginxPodName))
			activeNginxPodName = newNginxPodName
		} else {
			Expect(newNginxPodName).To(Equal(activeNginxPodName))
		}

		setUpPortForward(activeNginxPodName, ns.Name)

		// sets activeNginxPodName to new pod
		checkNGFFunctionality(teaURL, coffeeURL, files, ns)

		if errorLogs := getNGFErrorLogs(activeNGFPodName); errorLogs != "" {
			fmt.Printf("NGF has error logs: \n%s", errorLogs)
		}

		if errorLogs := getUnexpectedNginxErrorLogs(activeNginxPodName, ns.Name); errorLogs != "" {
			fmt.Printf("NGINX has unexpected error logs: \n%s", errorLogs)
		}
	}

	runRestartNodeWithDrainingTest := func(teaURL, coffeeURL string, files []string, ns *core.Namespace) {
		runRestartNodeTest(teaURL, coffeeURL, files, ns, true)
	}

	runRestartNodeAbruptlyTest := func(teaURL, coffeeURL string, files []string, ns *core.Namespace) {
		runRestartNodeTest(teaURL, coffeeURL, files, ns, false)
	}

	getLeaderElectionLeaseHolderName := func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
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

	checkLeaderLeaseChange := func(originalLeaseName string) error {
		leaseName, err := getLeaderElectionLeaseHolderName()
		if err != nil {
			return err
		}

		if originalLeaseName == leaseName {
			return fmt.Errorf(
				"expected originalLeaseName: %s, to not match current leaseName: %s",
				originalLeaseName,
				leaseName,
			)
		}

		return nil
	}

	BeforeAll(func() {
		podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(podNames).To(HaveLen(1))

		activeNGFPodName = podNames[0]

		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "graceful-recovery",
			},
		}

		Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())

		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, ns.Name, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))

		activeNginxPodName = nginxPodNames[0]

		setUpPortForward(activeNginxPodName, ns.Name)

		if portFwdPort != 0 {
			coffeeURL = fmt.Sprintf("%s:%d/coffee", baseHTTPURL, portFwdPort)
		}
		if portFwdHTTPSPort != 0 {
			teaURL = fmt.Sprintf("%s:%d/tea", baseHTTPSURL, portFwdHTTPSPort)
		}

		Eventually(
			func() error {
				return checkForWorkingTraffic(teaURL, coffeeURL)
			}).
			WithTimeout(timeoutConfig.TestForTrafficTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, ns.Name)
		cleanUpPortForward()
		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())
	})

	It("recovers when nginx container is restarted", func() {
		restartNginxContainer(activeNginxPodName, ns.Name, nginxContainerName)

		nginxPodNames, err := framework.GetReadyNginxPodNames(k8sClient, ns.Name, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		activeNginxPodName = nginxPodNames[0]

		setUpPortForward(activeNginxPodName, ns.Name)

		// sets activeNginxPodName to new pod
		checkNGFFunctionality(teaURL, coffeeURL, files, &ns)

		if errorLogs := getNGFErrorLogs(activeNGFPodName); errorLogs != "" {
			fmt.Printf("NGF has error logs: \n%s", errorLogs)
		}

		if errorLogs := getUnexpectedNginxErrorLogs(activeNginxPodName, ns.Name); errorLogs != "" {
			fmt.Printf("NGINX has unexpected error logs: \n%s", errorLogs)
		}
	})

	It("recovers when NGF Pod is restarted", func() {
		leaseName, err := getLeaderElectionLeaseHolderName()
		Expect(err).ToNot(HaveOccurred())

		ngfPod, err := resourceManager.GetPod(ngfNamespace, activeNGFPodName)
		Expect(err).ToNot(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
		defer cancel()

		Expect(k8sClient.Delete(ctx, ngfPod)).To(Succeed())

		var newNGFPodNames []string
		Eventually(
			func() bool {
				newNGFPodNames, err = framework.GetReadyNGFPodNames(
					k8sClient,
					ngfNamespace,
					releaseName,
					timeoutConfig.GetStatusTimeout,
				)
				return len(newNGFPodNames) == 1 && err == nil
			}).
			WithTimeout(timeoutConfig.CreateTimeout * 2).
			WithPolling(500 * time.Millisecond).
			MustPassRepeatedly(20).
			Should(BeTrue())

		newNGFPodName := newNGFPodNames[0]
		Expect(newNGFPodName).ToNot(BeEmpty())

		Expect(newNGFPodName).ToNot(Equal(activeNGFPodName))
		activeNGFPodName = newNGFPodName

		Eventually(
			func() error {
				return checkLeaderLeaseChange(leaseName)
			}).
			WithTimeout(timeoutConfig.GetLeaderLeaseTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())

		// sets activeNginxPodName to new pod
		checkNGFFunctionality(teaURL, coffeeURL, files, &ns)

		if errorLogs := getNGFErrorLogs(activeNGFPodName); errorLogs != "" {
			fmt.Printf("NGF has error logs: \n%s", errorLogs)
		}

		if errorLogs := getUnexpectedNginxErrorLogs(activeNginxPodName, ns.Name); errorLogs != "" {
			fmt.Printf("NGINX has unexpected error logs: \n%s", errorLogs)
		}
	})

	It("recovers when drained node is restarted", func() {
		runRestartNodeWithDrainingTest(teaURL, coffeeURL, files, &ns)
	})

	It("recovers when node is restarted abruptly", func() {
		if *plusEnabled {
			Skip(fmt.Sprintf("Skipping test when using NGINX Plus due to known issue:" +
				" https://github.com/nginx/nginx-gateway-fabric/issues/3248"))
		}
		runRestartNodeAbruptlyTest(teaURL, coffeeURL, files, &ns)
	})
})

func expectRequestToSucceed(appURL, address string, responseBodyMessage string) error {
	status, body, err := framework.Get(appURL, address, timeoutConfig.RequestTimeout, nil, nil)

	if status != http.StatusOK {
		return errors.New("http status was not 200")
	}

	if !strings.Contains(body, responseBodyMessage) {
		return errors.New("expected response body to contain correct body message")
	}

	return err
}

func expectRequestToFail(appURL, address string) error {
	status, body, err := framework.Get(appURL, address, timeoutConfig.RequestTimeout, nil, nil)
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

func getNginxErrorLogs(nginxPodName, namespace string) string {
	nginxLogs, err := resourceManager.GetPodLogs(
		namespace,
		nginxPodName,
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

func getUnexpectedNginxErrorLogs(nginxPodName, namespace string) string {
	expectedErrStrings := []string{
		"connect() failed (111: Connection refused)",
		"could not be resolved (host not found) during usage report",
		"server returned 429",
		"no live upstreams while connecting to upstream",
	}

	unexpectedErrors := ""

	errorLogs := getNginxErrorLogs(nginxPodName, namespace)

	for _, line := range strings.Split(errorLogs, "\n") {
		if !slices.ContainsFunc(expectedErrStrings, func(s string) bool {
			return strings.Contains(line, s)
		}) {
			unexpectedErrors += line
		}
	}

	return unexpectedErrors
}

// getNGFErrorLogs gets NGF container error logs.
func getNGFErrorLogs(ngfPodName string) string {
	ngfLogs, err := resourceManager.GetPodLogs(
		ngfNamespace,
		ngfPodName,
		&core.PodLogOptions{Container: ngfContainerName},
	)
	Expect(err).ToNot(HaveOccurred())

	errorLogs := ""

	for _, line := range strings.Split(ngfLogs, "\n") {
		if strings.Contains(line, "\"level\":\"error\"") {
			errorLogs += line + "\n"
			break
		}
	}

	return errorLogs
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
