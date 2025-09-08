package framework

import (
	"fmt"
	"runtime/debug"

	. "github.com/onsi/ginkgo/v2"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetLogs returns the logs for all containers in all pods for a release.
func GetLogs(rm ResourceManager, namespace string, releaseName string) string {
	GinkgoWriter.Printf(
		"Getting logs for all containers in all pods for release %q in namespace %q\n",
		releaseName,
		namespace,
	)
	var returnLogs string
	pods, err := rm.GetPods(namespace, client.MatchingLabels{
		"app.kubernetes.io/instance": releaseName,
	})
	if err != nil {
		return fmt.Sprintf("failed to get pods: %v", err)
	}

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			returnLogs += fmt.Sprintf("Logs for container %s:\n", container.Name)
			logs, err := rm.GetPodLogs(pod.Namespace, pod.Name, &core.PodLogOptions{
				Container: container.Name,
			})
			if err != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during getting logs for container %q in pod %q in namespace %q: %v\n",
					container.Name,
					pod.Name,
					pod.Namespace,
					err,
				)
				returnLogs += fmt.Sprintf("  failed to get logs: %v\n", err)
				continue
			}
			returnLogs += fmt.Sprintf("  %s\n", logs)
		}
	}
	return returnLogs
}

// GetEvents returns the events for a namespace.
func GetEvents(rm ResourceManager, namespace string) string {
	var returnEvents string
	events, err := rm.GetEvents(namespace)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting events in namespace %q: %v\n", namespace, err)

		return fmt.Sprintf("failed to get events: %v", err)
	}

	eventGroups := make(map[string][]core.Event)
	for _, event := range events.Items {
		eventGroups[event.InvolvedObject.Name] = append(eventGroups[event.InvolvedObject.Name], event)
	}

	for name, events := range eventGroups {
		returnEvents += fmt.Sprintf("Events for %s:\n", name)
		for _, event := range events {
			returnEvents += fmt.Sprintf("  %s\n", event.Message)
		}
		returnEvents += "\n"
	}
	return returnEvents
}

// GetBuildInfo returns the build information.
func GetBuildInfo() (commitHash string, commitTime string, dirtyBuild string) {
	GinkgoWriter.Printf("Getting build info\n")
	commitHash = "unknown"
	commitTime = "unknown"
	dirtyBuild = "unknown"

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			commitHash = kv.Value
		case "vcs.time":
			commitTime = kv.Value
		case "vcs.modified":
			dirtyBuild = kv.Value
		}
	}

	return
}

// AddNginxLogsAndEventsToReport adds nginx logs and events from the namespace to the report if the spec failed.
func AddNginxLogsAndEventsToReport(rm ResourceManager, namespace string, opts ...Option) {
	if CurrentSpecReport().Failed() {
		GinkgoWriter.Printf("Current spec failed. Adding Nginx logs and events to report for namespace %q\n", namespace)
		var returnLogs string

		nginxPodNames, _ := GetReadyNginxPodNames(rm.K8sClient, namespace, rm.TimeoutConfig.GetStatusTimeout, opts...)

		for _, nginxPodName := range nginxPodNames {
			returnLogs += fmt.Sprintf("Logs for Nginx Pod %s:\n", nginxPodName)
			nginxLogs, _ := rm.GetPodLogs(
				namespace,
				nginxPodName,
				&core.PodLogOptions{Container: "nginx"},
			)

			returnLogs += fmt.Sprintf("  %s\n", nginxLogs)
		}
		AddReportEntry("Nginx Logs", returnLogs, ReportEntryVisibilityNever)

		events := GetEvents(rm, namespace)
		AddReportEntry("Test Events", events, ReportEntryVisibilityNever)
	}
}
