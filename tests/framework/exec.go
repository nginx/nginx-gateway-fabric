package framework

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// execResult holds the output of an exec command.
type execResult struct {
	Stdout string
	Stderr string
}

// execInPod executes a command inside a container of a Kubernetes pod.
// If container is empty, the first container in the pod is used.
func execInPod(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	k8sConfig *rest.Config,
	namespace, podName, container string,
	command []string,
) (execResult, error) {
	opts := &core.PodExecOptions{
		Command:   command,
		Container: container,
		Stdout:    true,
		Stderr:    true,
	}

	req := k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		SubResource("exec").
		Name(podName).
		Namespace(namespace).
		VersionedParams(opts, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k8sConfig, http.MethodPost, req.URL())
	if err != nil {
		return execResult{}, fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer

	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return execResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}, fmt.Errorf("exec failed: %w (stderr: %s)", err, stderr.String())
	}

	return execResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, nil
}
