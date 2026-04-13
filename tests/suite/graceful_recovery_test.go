package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
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

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

const (
	nginxContainerName = "nginx"
	ngfContainerName   = "nginx-gateway"
	baseHTTPURL        = "http://cafe.example.com"
	baseHTTPSURL       = "https://cafe.example.com"
)

var gracefulRecoveryFiles = []string{
	"graceful-recovery/cafe.yaml",
	"graceful-recovery/cafe-secret.yaml",
	"graceful-recovery/gateway.yaml",
	"graceful-recovery/cafe-routes.yaml",
}

// Graceful Recovery tests verify that NGF recovers from various failure scenarios.
// Each scenario is an independent Describe block so it can be run in isolation during
// development and distributed across parallel processes in CI.
//
// Tests 1 and 2 (nginx container and NGF pod restarts) have no concurrency restriction:
// each GINKGO_PROCS process has its own NGF deployment and namespace, so they are isolated.
//
// Tests 3 and 4 (node restarts) are marked Serial because they restart the kind Docker
// container, which kills the entire Kubernetes node and would interfere with any other
// test running concurrently on the same cluster.
var _ = Describe("Graceful Recovery: nginx container restart", Label("graceful-recovery"), FlakeAttempts(2), func() {
	var (
		ns                 core.Namespace
		activeNGFPodName   string
		activeNginxPodName string
		teaURL             = baseHTTPSURL + "/tea"
		coffeeURL          = baseHTTPURL + "/coffee"
	)

	BeforeEach(func() {
		grSetup(&ns, &activeNGFPodName, &activeNginxPodName, &teaURL, &coffeeURL)
	})

	AfterEach(func() {
		grTeardown(&ns)
	})

	It("recovers when nginx container is restarted", func() {
		restartNginxContainer(activeNginxPodName, ns.Name, nginxContainerName)

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
			ns.Name,
			timeoutConfig.GetStatusTimeout,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		activeNginxPodName = nginxPodNames[0]

		setUpPortForward(activeNginxPodName, ns.Name)
		grRefreshURLs(&teaURL, &coffeeURL)

		checkNGFFunctionality(gracefulRecoveryFiles, &ns, &activeNginxPodName, &teaURL, &coffeeURL)

		if errorLogs := getNGFErrorLogs(activeNGFPodName); errorLogs != "" {
			GinkgoWriter.Printf("NGF has error logs: \n%s", errorLogs)
		}
		if errorLogs := getUnexpectedNginxErrorLogs(activeNginxPodName, ns.Name); errorLogs != "" {
			GinkgoWriter.Printf("NGINX has unexpected error logs: \n%s", errorLogs)
		}
	})
})

var _ = Describe("Graceful Recovery: NGF pod restart", Label("graceful-recovery"), FlakeAttempts(2), func() {
	var (
		ns                 core.Namespace
		activeNGFPodName   string
		activeNginxPodName string
		teaURL             = baseHTTPSURL + "/tea"
		coffeeURL          = baseHTTPURL + "/coffee"
	)

	BeforeEach(func() {
		grSetup(&ns, &activeNGFPodName, &activeNginxPodName, &teaURL, &coffeeURL)
	})

	AfterEach(func() {
		grTeardown(&ns)
	})

	It("recovers when NGF pod is restarted", func() {
		leaseName, err := getLeaderElectionLeaseHolderName()
		Expect(err).ToNot(HaveOccurred())

		ngfPod, err := resourceManager.GetPod(ngfNamespace, activeNGFPodName)
		Expect(err).ToNot(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
		defer cancel()

		Expect(resourceManager.Delete(ctx, ngfPod, nil)).To(Succeed())

		var newNGFPodNames []string
		Eventually(
			func() bool {
				newNGFPodNames, err = resourceManager.GetReadyNGFPodNames(
					ngfNamespace,
					releaseName,
					timeoutConfig.GetStatusTimeout,
				)
				return len(newNGFPodNames) == 1 && err == nil
			}).
			WithTimeout(timeoutConfig.CreateTimeout * 2).
			WithPolling(500 * time.Millisecond).
			MustPassRepeatedly(3).
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

		cleanUpPortForward()

		// The nginx pod shouldn't necessarily change when the NGF pod is restarted,
		// but in case there were any errors during this test which causes a re-run,
		// we'll need to get the nginx pod name again since it may have changed and
		// the loadbalancer IP address may have changed as well.
		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
			ns.Name,
			timeoutConfig.GetStatusTimeout,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		activeNginxPodName = nginxPodNames[0]

		setUpPortForward(activeNginxPodName, ns.Name)
		grRefreshURLs(&teaURL, &coffeeURL)

		checkNGFFunctionality(gracefulRecoveryFiles, &ns, &activeNginxPodName, &teaURL, &coffeeURL)

		if errorLogs := getNGFErrorLogs(activeNGFPodName); errorLogs != "" {
			GinkgoWriter.Printf("NGF has error logs: \n%s", errorLogs)
		}
		if errorLogs := getUnexpectedNginxErrorLogs(activeNginxPodName, ns.Name); errorLogs != "" {
			GinkgoWriter.Printf("NGINX has unexpected error logs: \n%s", errorLogs)
		}
	})
})

// Serial: this test restarts the kind Docker container (the entire Kubernetes node).
// Running it concurrently with other tests on the same cluster would kill their pods.
var _ = Describe("Graceful Recovery: drained node restart",
	Serial, Label("graceful-recovery"), FlakeAttempts(2), func() {
		var (
			ns                 core.Namespace
			activeNGFPodName   string
			activeNginxPodName string
			teaURL             = baseHTTPSURL + "/tea"
			coffeeURL          = baseHTTPURL + "/coffee"
		)

		BeforeEach(func() {
			grSetup(&ns, &activeNGFPodName, &activeNginxPodName, &teaURL, &coffeeURL)
		})

		AfterEach(func() {
			grTeardown(&ns)
		})

		It("recovers when drained node is restarted", func() {
			runRestartNodeTest(
				gracefulRecoveryFiles, &ns,
				&activeNGFPodName, &activeNginxPodName,
				&teaURL, &coffeeURL,
				true, /* drain */
			)
		})
	})

// Serial: this test restarts the kind Docker container (the entire Kubernetes node).
// Running it concurrently with other tests on the same cluster would kill their pods.
var _ = Describe("Graceful Recovery: abrupt node restart",
	Serial, Label("graceful-recovery"), FlakeAttempts(2), func() {
		var (
			ns                 core.Namespace
			activeNGFPodName   string
			activeNginxPodName string
			teaURL             = baseHTTPSURL + "/tea"
			coffeeURL          = baseHTTPURL + "/coffee"
		)

		BeforeEach(func() {
			grSetup(&ns, &activeNGFPodName, &activeNginxPodName, &teaURL, &coffeeURL)
		})

		AfterEach(func() {
			grTeardown(&ns)
		})

		It("recovers when node is restarted abruptly", func() {
			if *plusEnabled {
				Skip("Skipping test when using NGINX Plus due to known issue:" +
					" https://github.com/nginx/nginx-gateway-fabric/issues/3248")
			}
			runRestartNodeTest(
				gracefulRecoveryFiles, &ns,
				&activeNGFPodName, &activeNginxPodName,
				&teaURL, &coffeeURL,
				false, /* drain */
			)
		})
	})

// grSetup initializes the test namespace, applies manifests, sets up port forwarding,
// and verifies that initial traffic is working before the test scenario runs.
func grSetup(ns *core.Namespace, activeNGFPodName, activeNginxPodName *string, teaURL, coffeeURL *string) {
	podNames, err := resourceManager.GetReadyNGFPodNames(
		ngfNamespace,
		releaseName,
		timeoutConfig.GetStatusTimeout,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(podNames).To(HaveLen(1))
	*activeNGFPodName = podNames[0]

	// Use a namespace name that is unique per Ginkgo process to avoid collisions
	// when multiple processes run simultaneously (e.g. GINKGO_PROCS=2).
	*ns = core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("graceful-recovery-%d", GinkgoParallelProcess()),
		},
	}

	Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
	Expect(resourceManager.ApplyFromFiles(gracefulRecoveryFiles, ns.Name)).To(Succeed())
	Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())

	nginxPodNames, err := resourceManager.GetReadyNginxPodNames(ns.Name, timeoutConfig.GetStatusTimeout)
	Expect(err).ToNot(HaveOccurred())
	Expect(nginxPodNames).To(HaveLen(1))
	*activeNginxPodName = nginxPodNames[0]

	setUpPortForward(*activeNginxPodName, ns.Name)
	grRefreshURLs(teaURL, coffeeURL)

	Eventually(
		func() error {
			return grCheckForWorkingTraffic(*teaURL, *coffeeURL)
		}).
		WithTimeout(timeoutConfig.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}

// grTeardown adds NGINX logs and events to the report, tears down the port forward,
// and deletes all test resources and the namespace.
func grTeardown(ns *core.Namespace) {
	framework.AddNginxLogsAndEventsToReport(resourceManager, ns.Name)
	cleanUpPortForward()
	Expect(resourceManager.DeleteFromFiles(gracefulRecoveryFiles, ns.Name)).To(Succeed())
	Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())
}

// grCheckForWorkingTraffic sends concurrent requests to the tea and coffee endpoints
// and expects both to succeed.
func grCheckForWorkingTraffic(teaURL, coffeeURL string) error {
	var wg sync.WaitGroup
	var teaErr, coffeeErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		teaErr = framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, teaURL, address, "URI: /tea")
	}()
	go func() {
		defer wg.Done()
		coffeeErr = framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, coffeeURL, address, "URI: /coffee")
	}()
	wg.Wait()

	return errors.Join(teaErr, coffeeErr)
}

// grCheckForFailingTraffic sends concurrent requests to the tea and coffee endpoints
// and expects both to fail (e.g. after manifests are deleted or NGINX is restarting).
func grCheckForFailingTraffic(teaURL, coffeeURL string) error {
	var wg sync.WaitGroup
	var teaErr, coffeeErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		teaErr = framework.ExpectRequestToFail(timeoutConfig.RequestTimeout, teaURL, address)
	}()
	go func() {
		defer wg.Done()
		coffeeErr = framework.ExpectRequestToFail(timeoutConfig.RequestTimeout, coffeeURL, address)
	}()
	wg.Wait()

	return errors.Join(teaErr, coffeeErr)
}

// grRefreshURLs updates teaURL and coffeeURL to include the current port forward ports.
// If no port forward is active (LoadBalancer mode), the base URLs are used unchanged.
func grRefreshURLs(teaURL, coffeeURL *string) {
	*coffeeURL = baseHTTPURL + "/coffee"
	*teaURL = baseHTTPSURL + "/tea"
	if portFwdPort != 0 {
		*coffeeURL = fmt.Sprintf("%s:%d/coffee", baseHTTPURL, portFwdPort)
	}
	if portFwdHTTPSPort != 0 {
		*teaURL = fmt.Sprintf("%s:%d/tea", baseHTTPSURL, portFwdHTTPSPort)
	}
}

// checkNGFFunctionality verifies that NGF is functioning correctly by:
// 1. Checking that traffic is working
// 2. Deleting manifests and verifying traffic fails
// 3. Re-applying manifests and verifying traffic recovers.
func checkNGFFunctionality(files []string, ns *core.Namespace, activeNginxPodName *string, teaURL, coffeeURL *string) {
	Eventually(
		func() error {
			return grCheckForWorkingTraffic(*teaURL, *coffeeURL)
		}).
		WithTimeout(timeoutConfig.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())

	cleanUpPortForward()
	Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())

	Eventually(
		func() error {
			return grCheckForFailingTraffic(*teaURL, *coffeeURL)
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
			nginxPodNames, err = resourceManager.GetReadyNginxPodNames(
				ns.Name,
				timeoutConfig.GetStatusTimeout,
			)
			return len(nginxPodNames) == 1 && err == nil
		}).
		WithTimeout(timeoutConfig.CreateTimeout).
		WithPolling(500 * time.Millisecond).
		MustPassRepeatedly(3).
		Should(BeTrue())

	*activeNginxPodName = nginxPodNames[0]
	Expect(*activeNginxPodName).ToNot(BeEmpty())

	setUpPortForward(*activeNginxPodName, ns.Name)
	grRefreshURLs(teaURL, coffeeURL)

	Eventually(
		func() error {
			return grCheckForWorkingTraffic(*teaURL, *coffeeURL)
		}).
		WithTimeout(timeoutConfig.TestForTrafficTimeout).
		WithPolling(500 * time.Millisecond).
		Should(Succeed())
}

// getContainerRestartCount returns the restart count for the named container in the given pod.
func getContainerRestartCount(podName, namespace, containerName string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var pod core.Pod
	if err := resourceManager.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: podName},
		&pod,
	); err != nil {
		return 0, fmt.Errorf("error retrieving Pod: %w", err)
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == containerName {
			return int(containerStatus.RestartCount), nil
		}
	}

	return 0, fmt.Errorf("container %q not found in pod %q", containerName, podName)
}

// checkContainerRestart verifies the container restart count has incremented by exactly 1
// from the expected current count.
func checkContainerRestart(podName, containerName, namespace string, currentRestartCount int) error {
	restartCount, err := getContainerRestartCount(podName, namespace, containerName)
	if err != nil {
		return err
	}

	if restartCount != currentRestartCount+1 {
		restartErr := fmt.Errorf("expected current restart count: %d to match incremented restart count: %d",
			restartCount, currentRestartCount+1)
		GinkgoWriter.Printf("%s\n", restartErr)
		return restartErr
	}

	return nil
}

// getNodeNames returns the names of all nodes in the cluster.
func getNodeNames() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var nodes core.NodeList
	if err := resourceManager.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("error listing nodes: %w", err)
	}

	names := make([]string, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		names = append(names, node.Name)
	}

	return names, nil
}

// runNodeDebuggerJob creates a node debugger job that targets the node where the given
// nginx pod is scheduled, triggering an nginx container restart.
func runNodeDebuggerJob(nginxPodName, namespace string) (*v1.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	var nginxPod core.Pod
	if err := resourceManager.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: nginxPodName},
		&nginxPod,
	); err != nil {
		return nil, fmt.Errorf("error retrieving nginx Pod: %w", err)
	}

	b, err := resourceManager.GetFileContents("graceful-recovery/node-debugger-job.yaml")
	if err != nil {
		debugErr := fmt.Errorf("error processing node debugger job file: %w", err)
		GinkgoWriter.Printf("%s\n", debugErr)
		return nil, debugErr
	}

	job := &v1.Job{}
	if err = yaml.Unmarshal(b.Bytes(), job); err != nil {
		yamlErr := fmt.Errorf("error with yaml unmarshal: %w", err)
		GinkgoWriter.Printf("%s\n", yamlErr)
		return nil, yamlErr
	}

	job.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"] = nginxPod.Spec.NodeName
	if len(job.Spec.Template.Spec.Containers) != 1 {
		containerErr := fmt.Errorf(
			"expected node debugger job to contain one container, actual number: %d",
			len(job.Spec.Template.Spec.Containers),
		)
		GinkgoWriter.Printf("ERROR: %s\n", containerErr)
		return nil, containerErr
	}
	job.Namespace = namespace

	if err = resourceManager.Apply([]client.Object{job}); err != nil {
		return nil, fmt.Errorf("error in applying job: %w", err)
	}

	return job, nil
}

// restartNginxContainer triggers an nginx container restart via a node debugger job
// and waits for the restart count to increment before returning.
func restartNginxContainer(nginxPodName, namespace, containerName string) {
	restartCount, err := getContainerRestartCount(nginxPodName, namespace, containerName)
	Expect(err).ToNot(HaveOccurred())

	cleanUpPortForward()
	job, err := runNodeDebuggerJob(nginxPodName, namespace)
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
	Expect(resourceManager.DeleteResources(
		[]client.Object{job},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)).To(Succeed())
}

// getLeaderElectionLeaseHolderName returns the current NGF leader election lease holder name.
func getLeaderElectionLeaseHolderName() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	var lease coordination.Lease
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-nginx-gateway-fabric-leader-election", releaseName),
		Namespace: ngfNamespace,
	}

	if err := resourceManager.Get(ctx, key, &lease); err != nil {
		return "", errors.New("could not retrieve leader election lease")
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == "" {
		leaderErr := errors.New("leader election lease holder identity is empty")
		GinkgoWriter.Printf("ERROR: %s\n", leaderErr)
		return "", leaderErr
	}

	return *lease.Spec.HolderIdentity, nil
}

// checkLeaderLeaseChange verifies the leader election lease holder has changed from the original.
func checkLeaderLeaseChange(originalLeaseName string) error {
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

// runRestartNodeTest restarts the kind Docker container (entire Kubernetes node) and verifies
// that NGF and NGINX recover. If drain is true, the node is gracefully drained before restarting;
// otherwise it is restarted abruptly.
//
// NGF and NGINX pod readiness are polled in parallel after the node restarts to reduce wait time.
func runRestartNodeTest(
	files []string,
	ns *core.Namespace,
	activeNGFPodName, activeNginxPodName *string,
	teaURL, coffeeURL *string,
	drain bool,
) {
	nodeNames, err := getNodeNames()
	Expect(err).ToNot(HaveOccurred())
	Expect(nodeNames).To(HaveLen(1))

	kindNodeName := nodeNames[0]

	Expect(clusterName).ToNot(BeNil(), "clusterName variable not set")
	Expect(*clusterName).ToNot(BeEmpty())
	containerName := *clusterName + "-control-plane"

	cleanUpPortForward()

	if drain {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
		defer drainCancel()
		output, err := exec.CommandContext(drainCtx,
			"kubectl",
			"drain",
			kindNodeName,
			"--ignore-daemonsets",
			"--delete-emptydir-data",
		).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))

		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), timeoutConfig.DeleteTimeout)
		defer deleteCancel()
		output, err = exec.CommandContext(deleteCtx, "kubectl", "delete", "node", kindNodeName).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
	}

	restartCtx, restartCancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout)
	defer restartCancel()
	_, err = exec.CommandContext(restartCtx, "docker", "restart", containerName).CombinedOutput()
	Expect(err).ToNot(HaveOccurred())

	// Wait for the Docker container to be running before polling for ready pods, otherwise the pod
	// API calls will return errors while the API server is still starting up.
	Eventually(
		func() bool {
			inspectCtx, inspectCancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
			defer inspectCancel()
			output, err := exec.CommandContext(inspectCtx,
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

	// Poll NGF and NGINX pod readiness in parallel — both pods recover simultaneously after
	// the node restarts, so polling them concurrently cuts the wait from ~240s max to ~120s max.
	newNGFPodName, newNginxPodName := waitForPodsReadyAfterNodeRestart(ns)

	// After a graceful drain, new pods are created on the rescheduled node.
	// After an abrupt restart, the same pods resume on the same node.
	if drain {
		Expect(newNGFPodName).ToNot(Equal(*activeNGFPodName))
		*activeNGFPodName = newNGFPodName
		Expect(newNginxPodName).ToNot(Equal(*activeNginxPodName))
		*activeNginxPodName = newNginxPodName
	} else {
		Expect(newNGFPodName).To(Equal(*activeNGFPodName))
		Expect(newNginxPodName).To(Equal(*activeNginxPodName))
	}

	// Wait for NGF to finish programming the Gateway and HTTPRoutes before checking traffic.
	// Pod readiness only means the containers are up — NGF may still be reconciling config
	// and pushing it to NGINX, especially after an abrupt restart where reconciliation
	// replays from scratch.
	Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())

	setUpPortForward(*activeNginxPodName, ns.Name)
	grRefreshURLs(teaURL, coffeeURL)

	checkNGFFunctionality(files, ns, activeNginxPodName, teaURL, coffeeURL)

	if errorLogs := getNGFErrorLogs(*activeNGFPodName); errorLogs != "" {
		GinkgoWriter.Printf("NGF has error logs: \n%s", errorLogs)
	}
	if errorLogs := getUnexpectedNginxErrorLogs(*activeNginxPodName, ns.Name); errorLogs != "" {
		GinkgoWriter.Printf("NGINX has unexpected error logs: \n%s", errorLogs)
	}
}

// waitForPodsReadyAfterNodeRestart polls NGF and NGINX pod readiness concurrently.
// Each pod must report exactly 1 ready pod on 3 consecutive checks before being considered stable.
// Returns the new NGF and NGINX pod names once both are stable.
func waitForPodsReadyAfterNodeRestart(ns *core.Namespace) (ngfPodName, nginxPodName string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout*2)
	defer cancel()

	// pollUntilStable blocks until getPodNames returns exactly 1 name on 3 consecutive polls,
	// or until ctx is canceled. It returns the stable pod name or an error.
	pollUntilStable := func(label string, getPodNames func() ([]string, error)) (string, error) {
		var consecutiveSuccesses int
		var stableName string
		for {
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("%s pods did not become stable after node restart: %w", label, ctx.Err())
			default:
			}
			names, err := getPodNames()
			if len(names) == 1 && err == nil {
				consecutiveSuccesses++
				stableName = names[0]
				if consecutiveSuccesses >= 3 {
					return stableName, nil
				}
			} else {
				consecutiveSuccesses = 0
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Run NGF and NGINX readiness polls concurrently — both pods recover simultaneously after
	// a node restart, so polling them in parallel cuts the wait from ~240s max to ~120s max.
	type result struct {
		err  error
		name string
	}

	ngfCh := make(chan result, 1)
	nginxCh := make(chan result, 1)

	// ngf can often oscillate between ready and error, so we wait for stable readiness.
	go func() {
		name, err := pollUntilStable("NGF", func() ([]string, error) {
			return resourceManager.GetReadyNGFPodNames(ngfNamespace, releaseName, timeoutConfig.GetStatusTimeout)
		})
		ngfCh <- result{name: name, err: err}
	}()

	go func() {
		name, err := pollUntilStable("NGINX", func() ([]string, error) {
			return resourceManager.GetReadyNginxPodNames(ns.Name, timeoutConfig.GetStatusTimeout)
		})
		nginxCh <- result{name: name, err: err}
	}()

	ngfResult := <-ngfCh
	nginxResult := <-nginxCh

	Expect(ngfResult.err).ToNot(HaveOccurred())
	Expect(nginxResult.err).ToNot(HaveOccurred())

	return ngfResult.name, nginxResult.name
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
