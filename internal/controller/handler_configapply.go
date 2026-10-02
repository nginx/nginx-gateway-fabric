package controller

import (
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

// maxConfigApplyRetryAttempts bounds how many times applyConfigWithRetry will regenerate and
// re-apply config in response to a recoverable, remediated failure, independent of any
// remediator-specific limit (e.g. a max zone size).
const maxConfigApplyRetryAttempts = 8

// configApplyIssueSource identifies which path surfaced a configApplyIssue.
type configApplyIssueSource int

const (
	// configApplyIssueSourceReload indicates a full config apply/reload failed at
	// config-parse time.
	configApplyIssueSourceReload configApplyIssueSource = iota
	// configApplyIssueSourcePlusAPI indicates a dynamic NGINX Plus API call failed.
	configApplyIssueSourcePlusAPI
)

func (s configApplyIssueSource) String() string {
	switch s {
	case configApplyIssueSourceReload:
		return "reload"
	case configApplyIssueSourcePlusAPI:
		return "plus-api"
	default:
		return "unknown"
	}
}

// configApplyIssue describes a config-apply failure.
type configApplyIssue struct {
	// err is the underlying error the issue was detected from.
	err error
	// matches holds any identifiers (e.g. upstream/zone names) the remediator extracted from
	// the error text, describing what specifically needs remediation.
	matches []string
	// source identifies which path surfaced the failure.
	source configApplyIssueSource
}

// configApplyRemediator recognizes one specific class of recoverable NGINX config-apply failure
// and knows how to mutate state so that a subsequent attempt may succeed.
type configApplyRemediator interface {
	// name identifies the remediator.
	name() string
	// detect inspects the most recent attempt's errors and returns the issue it recognizes as its own, if any.
	detect(configErr, upstreamErr error) (issue configApplyIssue, ok bool)
	// remediate attempts to make progress against a previously detected issue, mutating
	// whatever state (e.g. deployment overrides) is needed for the next attempt to have a
	// chance of succeeding. Returns false if no further progress is possible which the caller treats as terminal.
	remediate(deployment *agent.Deployment, issue configApplyIssue) bool
}

// detectConfigApplyIssue returns the first remediator (in order) that recognizes the given
// errors as an issue it can act on, along with the issue it detected.
func detectConfigApplyIssue(
	remediators []configApplyRemediator,
	configErr, upstreamErr error,
) (configApplyRemediator, configApplyIssue, bool) {
	for _, r := range remediators {
		if issue, ok := r.detect(configErr, upstreamErr); ok {
			return r, issue, true
		}
	}

	return nil, configApplyIssue{}, false
}

// applyConfigWithRetry generates and applies nginx configuration.
// If applying fails, each remediator is given a chance to recognize the failure and
// remediate it; if one does, config is regenerated and the whole cycle is retried.
// This continues until the apply succeeds, a remediator reports it can make no
// further progress, or no remediator recognizes the failure -- in which case the failure is
// surfaced as-is via the deployment's latest error fields.
func (h *eventHandlerImpl) applyConfigWithRetry(
	deployment *agent.Deployment,
	conf dataplane.Configuration,
	volumeMounts []v1.VolumeMount,
	generate func(logr.Logger) []agent.File,
	remediators []configApplyRemediator,
) {
	logger := h.cfg.runtimeLogger.Logger.WithName("applyConfigWithRetry")

	for attempt := range maxConfigApplyRetryAttempts {
		files := generate(logger)
		h.cfg.nginxUpdater.UpdateConfig(deployment, files, volumeMounts)
		configErr := deployment.GetLatestConfigError()

		var upstreamErr error
		if h.cfg.plus {
			h.cfg.nginxUpdater.UpdateUpstreamServers(deployment, conf)
			upstreamErr = deployment.GetLatestUpstreamError()
		}

		if configErr == nil && upstreamErr == nil {
			return
		}

		logger.V(1).Info(
			"Config apply attempt reported an error; checking if any remediator recognizes it",
			"attempt", attempt+1,
			"configError", configErr,
			"upstreamError", upstreamErr,
		)

		remediator, issue, ok := detectConfigApplyIssue(remediators, configErr, upstreamErr)
		if !ok {
			return
		}

		if !remediator.remediate(deployment, issue) {
			logger.Error(issue.err, "recoverable config apply issue reached its remediation limit",
				"remediator", remediator.name(), "matches", issue.matches, "source", issue.source)
			return
		}

		logger.Info("Retrying NGINX config apply after remediating a recoverable issue",
			"remediator", remediator.name(), "matches", issue.matches, "source", issue.source, "attempt", attempt+1)
	}

	logger.Error(
		fmt.Errorf("exhausted %d config apply retry attempts", maxConfigApplyRetryAttempts),
		"giving up remediating recoverable config apply issue(s)",
	)
}
