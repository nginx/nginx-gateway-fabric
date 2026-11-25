package framework

import (
	"fmt"
	"runtime/debug"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// notReadyPodLogTailLines caps how many log lines are fetched per not-ready container
// so that failure diagnostics stay readable and avoid excessive API load.
const notReadyPodLogTailLines = 50

// GetLogs returns the logs for all containers in all pods for a release.
func GetLogs(rm ResourceManager, namespace string, releaseName string) string {
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
	commitHash = "unknown"
	commitTime = "unknown"
	dirtyBuild = "unknown"

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return commitHash, commitTime, dirtyBuild
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

	return commitHash, commitTime, dirtyBuild
}

// AddNginxLogsAndEventsToReport adds nginx logs and events from the namespace to the report if the spec failed.
func AddNginxLogsAndEventsToReport(rm ResourceManager, namespace string, opts ...Option) {
	if CurrentSpecReport().Failed() {
		var returnLogs string

		nginxPodNames, _ := rm.GetReadyNginxPodNames(namespace, rm.TimeoutConfig.GetStatusTimeout, opts...)

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

// podReady reports whether the given Pod has a Ready condition that is True (or has completed).
func podReady(pod core.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == core.PodReady && (cond.Status == core.ConditionTrue || cond.Reason == "PodCompleted") {
			return true
		}
	}

	return false
}

// describeContainerStatuses appends describe-style failure info for a set of container statuses.
func describeContainerStatuses(sb *strings.Builder, label string, statuses []core.ContainerStatus) {
	for _, cs := range statuses {
		fmt.Fprintf(sb, "    %s %q: ready=%t restartCount=%d\n", label, cs.Name, cs.Ready, cs.RestartCount)

		switch {
		case cs.State.Waiting != nil:
			fmt.Fprintf(sb, "      waiting: reason=%q message=%q\n",
				cs.State.Waiting.Reason, cs.State.Waiting.Message)
		case cs.State.Terminated != nil:
			fmt.Fprintf(sb, "      terminated: reason=%q exitCode=%d message=%q\n",
				cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
		}

		if cs.LastTerminationState.Terminated != nil {
			lt := cs.LastTerminationState.Terminated
			fmt.Fprintf(sb, "      lastTermination: reason=%q exitCode=%d message=%q\n",
				lt.Reason, lt.ExitCode, lt.Message)
		}
	}
}

// DescribeNotReadyPods returns describe-style diagnostics for every not-ready Pod in the namespace,
// including Pod phase/conditions, container states (waiting/terminated reasons, restart counts,
// last-termination info), tailed logs for started-but-not-ready containers, and namespace Events.
func DescribeNotReadyPods(rm ResourceManager, namespace string) string {
	pods, err := rm.GetPods(namespace, client.MatchingLabels{})
	if err != nil {
		return fmt.Sprintf("failed to list pods in namespace %q for diagnostics: %v\n", namespace, err)
	}

	var sb strings.Builder

	var notReady int
	for _, pod := range pods {
		if podReady(pod) {
			continue
		}
		notReady++

		fmt.Fprintf(&sb, "Not-ready Pod %q (phase=%s):\n", pod.Name, pod.Status.Phase)

		for _, cond := range pod.Status.Conditions {
			fmt.Fprintf(&sb, "    condition %s=%s reason=%q message=%q\n",
				cond.Type, cond.Status, cond.Reason, cond.Message)
		}

		describeContainerStatuses(&sb, "initContainer", pod.Status.InitContainerStatuses)
		describeContainerStatuses(&sb, "container", pod.Status.ContainerStatuses)

		// Fetch tailed logs for containers that have started but are not ready (e.g. crash-looping).
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready || (cs.State.Waiting != nil && cs.LastTerminationState.Terminated == nil) {
				// Skip containers that are ready, or waiting without ever having started (no logs yet).
				continue
			}
			tail := int64(notReadyPodLogTailLines)
			logs, logErr := rm.GetPodLogs(pod.Namespace, pod.Name, &core.PodLogOptions{
				Container: cs.Name,
				TailLines: &tail,
			})
			if logErr != nil {
				fmt.Fprintf(&sb, "      logs for container %q: failed to get logs: %v\n", cs.Name, logErr)
				continue
			}
			fmt.Fprintf(&sb, "      logs for container %q (last %d lines):\n%s\n",
				cs.Name, notReadyPodLogTailLines, logs)
		}
	}

	if notReady == 0 {
		fmt.Fprintf(&sb, "no not-ready pods found in namespace %q\n", namespace)
	}

	sb.WriteString(GetEvents(rm, namespace))

	return sb.String()
}
