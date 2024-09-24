package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// ExpectedNginxField contains an nginx directive key and value,
// and the expected file, server, and location block that it should exist in.
type ExpectedNginxField struct {
	// Directive is the directive name.
	Directive string
	// Value is the value for the directive. Can be the full value or a substring. If it's a substring,
	// then ValueSubstringAllowed should be true.
	Value string
	// File is the file name that should contain the directive. Can be a full filename or a substring.
	File string
	// Location is the location name that the directive should exist in.
	Location string
	// Servers are the server names that the directive should exist in.
	Servers []string
	// ValueSubstringAllowed allows the expected value to be a substring of the real value.
	// This makes it easier for cases when real values are complex file names or contain things we
	// don't care about, and we just want to check if a substring exists.
	ValueSubstringAllowed bool
}

// ValidateNginxFieldExists accepts the nginx config and the configuration for the expected field,
// and returns whether or not that field exists where it should.
func ValidateNginxFieldExists(conf *Payload, expFieldCfg ExpectedNginxField) error {
	for _, config := range conf.Config {
		if !strings.Contains(config.File, expFieldCfg.File) {
			continue
		}

		for _, directive := range config.Parsed {
			if len(expFieldCfg.Servers) == 0 {
				if expFieldCfg.fieldFound(directive) {
					return nil
				}
				continue
			}

			for _, serverName := range expFieldCfg.Servers {
				if directive.Directive == "server" && getServerName(directive.Block) == serverName {
					for _, serverDirective := range directive.Block {
						if expFieldCfg.Location == "" && expFieldCfg.fieldFound(serverDirective) {
							return nil
						} else if serverDirective.Directive == "location" &&
							fieldExistsInLocation(serverDirective, expFieldCfg) {
							return nil
						}
					}
				}
			}
		}
	}

	b, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("error marshaling nginx config: %w", err)
	}

	return fmt.Errorf("field not found; expected: %+v\nNGINX conf: %s", expFieldCfg, string(b))
}

func getServerName(serverBlock Directives) string {
	for _, directive := range serverBlock {
		if directive.Directive == "server_name" {
			return directive.Args[0]
		}
	}

	return ""
}

func (e ExpectedNginxField) fieldFound(directive *Directive) bool {
	arg := strings.Join(directive.Args, " ")

	valueMatch := arg == e.Value
	if e.ValueSubstringAllowed {
		valueMatch = strings.Contains(arg, e.Value)
	}

	return directive.Directive == e.Directive && valueMatch
}

func fieldExistsInLocation(locationDirective *Directive, expFieldCfg ExpectedNginxField) bool {
	// location could start with '=', so get the last element which is the path
	loc := locationDirective.Args[len(locationDirective.Args)-1]
	if loc == expFieldCfg.Location {
		for _, directive := range locationDirective.Block {
			if expFieldCfg.fieldFound(directive) {
				return true
			}
		}
	}

	return false
}

// injectCrossplaneContainer adds an ephemeral container that contains crossplane for parsing
// nginx config. It attaches to the nginx container and shares volumes with it.
func injectCrossplaneContainer(
	k8sClient kubernetes.Interface,
	timeout time.Duration,
	ngfPodName,
	namespace string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pod := &core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ngfPodName,
			Namespace: namespace,
		},
		Spec: core.PodSpec{
			EphemeralContainers: []core.EphemeralContainer{
				{
					TargetContainerName: "nginx",
					EphemeralContainerCommon: core.EphemeralContainerCommon{
						Name:            "crossplane",
						Image:           "nginx-crossplane:latest",
						ImagePullPolicy: "Never",
						Stdin:           true,
						VolumeMounts: []core.VolumeMount{
							{
								MountPath: "/etc/nginx/conf.d",
								Name:      "nginx-conf",
							},
							{
								MountPath: "/etc/nginx/stream-conf.d",
								Name:      "nginx-stream-conf",
							},
							{
								MountPath: "/etc/nginx/module-includes",
								Name:      "module-includes",
							},
							{
								MountPath: "/etc/nginx/secrets",
								Name:      "nginx-secrets",
							},
							{
								MountPath: "/etc/nginx/includes",
								Name:      "nginx-includes",
							},
						},
					},
				},
			},
		},
	}

	podClient := k8sClient.CoreV1().Pods(namespace)
	if _, err := podClient.UpdateEphemeralContainers(ctx, ngfPodName, pod, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("error adding ephemeral container: %w", err)
	}

	return nil
}

// createCrossplaneExecutor creates the executor for the crossplane command.
func createCrossplaneExecutor(
	k8sClient kubernetes.Interface,
	k8sConfig *rest.Config,
	ngfPodName,
	namespace string,
) (remotecommand.Executor, error) {
	cmd := []string{"./crossplane", "/etc/nginx/nginx.conf"}
	opts := &core.PodExecOptions{
		Command:   cmd,
		Container: "crossplane",
		Stdout:    true,
		Stderr:    true,
	}

	req := k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		SubResource("exec").
		Name(ngfPodName).
		Namespace(namespace).
		VersionedParams(opts, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k8sConfig, http.MethodPost, req.URL())
	if err != nil {
		return nil, fmt.Errorf("error creating executor: %w", err)
	}

	return exec, nil
}

// The following types are copied from https://github.com/nginxinc/nginx-go-crossplane,
// with unnecessary fields stripped out.
type Payload struct {
	Config []Config `json:"config"`
}

type Config struct {
	File   string     `json:"file"`
	Parsed Directives `json:"parsed"`
}

type Directive struct {
	Comment   *string    `json:"comment,omitempty"`
	Directive string     `json:"directive"`
	File      string     `json:"file,omitempty"`
	Args      []string   `json:"args"`
	Includes  []int      `json:"includes,omitempty"`
	Block     Directives `json:"block,omitempty"`
	Line      int        `json:"line"`
}

type Directives []*Directive
