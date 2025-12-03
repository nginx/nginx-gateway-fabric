// Utility functions for managing resources in Kubernetes. Inspiration and methods used from
// https://github.com/kubernetes-sigs/gateway-api/tree/main/conformance/utils.

/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
)

// ResourceManager handles creating/updating/deleting Kubernetes resources.
type ResourceManager struct {
	K8sClient      client.Client
	ClientGoClient kubernetes.Interface // used when k8sClient is not enough
	K8sConfig      *rest.Config
	FS             embed.FS
	TimeoutConfig  TimeoutConfig
}

// ClusterInfo holds the cluster metadata.
type ClusterInfo struct {
	K8sVersion string
	// ID is the UID of kube-system namespace
	ID              string
	MemoryPerNode   string
	GkeInstanceType string
	GkeZone         string
	NodeCount       int
	CPUCountPerNode int64
	MaxPodsPerNode  int64
	IsGKE           bool
}

// Apply creates or updates Kubernetes resources defined as Go objects.
func (rm *ResourceManager) Apply(resources []client.Object, opts ...Option) error {
	options := LogOptions(opts...)
	if options.logEnabled {
		GinkgoWriter.Printf("Applying resources defined as Go objects\n")
	}
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.CreateTimeout)
	defer cancel()

	for _, resource := range resources {
		var obj client.Object

		unstructuredObj, ok := resource.(*unstructured.Unstructured)
		if ok {
			obj = unstructuredObj.DeepCopy()
		} else {
			t := reflect.TypeOf(resource).Elem()
			obj, ok = reflect.New(t).Interface().(client.Object)
			if !ok {
				panicMsg := "failed to cast object to client.Object"
				GinkgoWriter.Printf(
					"PANIC occurred during applying creates or updates Kubernetes resources defined as Go objects: %s\n",
					panicMsg,
				)

				panic(panicMsg)
			}
		}

		if err := rm.Get(ctx, client.ObjectKeyFromObject(resource), obj, opts...); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}

			if err := rm.Create(ctx, resource); err != nil {
				return fmt.Errorf("error creating resource: %w", err)
			}

			continue
		}

		// Some tests modify resources that are also modified by NGF (to update their status), so conflicts are possible
		// For example, a Gateway resource.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := rm.Get(ctx, client.ObjectKeyFromObject(resource), obj); err != nil {
				return err
			}
			resource.SetResourceVersion(obj.GetResourceVersion())

			return rm.Update(ctx, resource, nil)
		})
		if err != nil {
			retryErr := fmt.Errorf("error updating resource: %w", err)
			GinkgoWriter.Printf("%s\n", retryErr)

			return retryErr
		}
	}
	if options.logEnabled {
		GinkgoWriter.Printf("Resources defined as Go objects applied successfully\n")
	}
	return nil
}

// ApplyFromFiles creates or updates Kubernetes resources defined within the provided YAML files.
func (rm *ResourceManager) ApplyFromFiles(files []string, namespace string, opts ...Option) error {
	options := LogOptions(opts...)
	for _, file := range files {
		if options.logEnabled {
			GinkgoWriter.Printf("\nApplying resources from file: %q to namespace %q\n", file, namespace)
		}
		data, err := rm.GetFileContents(file)
		if err != nil {
			return err
		}

		if err = rm.ApplyFromBuffer(data, namespace); err != nil {
			return err
		}
	}
	if options.logEnabled {
		GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)
	}

	return nil
}

func (rm *ResourceManager) ApplyFromBuffer(buffer *bytes.Buffer, namespace string, opts ...Option) error {
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.CreateTimeout)
	defer cancel()

	options := LogOptions(opts...)
	if options.logEnabled {
		GinkgoWriter.Printf("Applying resources from buffer to namespace %q\n", namespace)
	}

	handlerFunc := func(obj unstructured.Unstructured) error {
		obj.SetNamespace(namespace)
		nsName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
		fetchedObj := obj.DeepCopy()
		if err := rm.Get(ctx, nsName, fetchedObj, opts...); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}

			if err := rm.Create(ctx, &obj); err != nil {
				return fmt.Errorf("error creating resource: %w", err)
			}

			return nil
		}

		// Some tests modify resources that are also modified by NGF (to update their status), so conflicts are possible
		// For example, a Gateway resource.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := rm.Get(ctx, nsName, fetchedObj); err != nil {
				return err
			}
			obj.SetResourceVersion(fetchedObj.GetResourceVersion())

			return rm.Update(ctx, &obj, nil)
		})
		if err != nil {
			retryErr := fmt.Errorf("error updating resource: %w", err)
			GinkgoWriter.Printf("%s\n", retryErr)

			return retryErr
		}

		return nil
	}

	return rm.readAndHandleObject(handlerFunc, buffer)
}

// Delete deletes Kubernetes resources defined as Go objects.
func (rm *ResourceManager) DeleteResources(resources []client.Object, opts ...client.DeleteOption) error {
	GinkgoWriter.Printf("Deleting resources\n")
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteTimeout)
	defer cancel()

	for _, resource := range resources {
		if err := rm.Delete(ctx, resource, opts); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting resource: %w", err)
		}
	}
	GinkgoWriter.Printf("Resources deleted successfully\n")

	return nil
}

func (rm *ResourceManager) DeleteNamespace(name string, opts ...Option) error {
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteNamespaceTimeout)
	GinkgoWriter.Printf("Deleting namespace %q\n", name)
	defer cancel()

	ns := &core.Namespace{}
	if err := rm.Get(ctx, types.NamespacedName{Name: name}, ns, opts...); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("error getting namespace: %w", err)
	}

	if err := rm.Delete(ctx, ns, nil, opts...); err != nil {
		return fmt.Errorf("error deleting namespace: %w", err)
	}

	GinkgoWriter.Printf("Waiting for namespace %q to be deleted\n", name)
	// Because the namespace deletion is asynchronous, we need to wait for the namespace to be deleted.
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			if err := rm.Get(ctx, types.NamespacedName{Name: name}, ns, opts...); err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}

				return false, fmt.Errorf("error getting namespace: %w", err)
			}

			return false, nil
		})
}

func (rm *ResourceManager) DeleteNamespaces(names []string, opts ...Option) error {
	GinkgoWriter.Printf("Deleting %d namespaces\n", len(names))
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteNamespaceTimeout*2)
	defer cancel()

	var combinedErrors error
	for _, name := range names {
		ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

		if err := rm.Delete(ctx, ns, nil, opts...); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("error deleting namespace: %w", err))
		}
	}

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			nsList := &core.NamespaceList{}
			if err := rm.List(ctx, nsList); err != nil {
				return false, nil //nolint:nilerr // retry on error
			}

			for _, namespace := range nsList.Items {
				if slices.Contains(names, namespace.Name) {
					return false, nil
				}
			}

			return true, nil
		})

	return errors.Join(combinedErrors, err)
}

// DeleteFromFiles deletes Kubernetes resources defined within the provided YAML files.
func (rm *ResourceManager) DeleteFromFiles(files []string, namespace string) error {
	GinkgoWriter.Printf("Deleting resources from files: %v in namespace %q\n", files, namespace)
	handlerFunc := func(obj unstructured.Unstructured) error {
		obj.SetNamespace(namespace)
		ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteTimeout)
		defer cancel()

		if err := rm.Delete(ctx, &obj, nil); err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		return nil
	}

	for _, file := range files {
		data, err := rm.GetFileContents(file)
		if err != nil {
			return err
		}

		if err = rm.readAndHandleObject(handlerFunc, data); err != nil {
			return err
		}
	}

	return nil
}

func (rm *ResourceManager) readAndHandleObject(
	handle func(unstructured.Unstructured) error,
	data *bytes.Buffer,
) error {
	decoder := yaml.NewYAMLOrJSONDecoder(data, 4096)

	for {
		obj := unstructured.Unstructured{}
		if err := decoder.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error decoding resource: %w", err)
		}

		if len(obj.Object) == 0 {
			continue
		}

		if err := handle(obj); err != nil {
			return err
		}
	}

	return nil
}

// GetFileContents takes a string that can either be a local file
// path or an https:// URL to YAML manifests and provides the contents.
func (rm *ResourceManager) GetFileContents(file string) (*bytes.Buffer, error) {
	if strings.HasPrefix(file, "http://") {
		err := fmt.Errorf("data can't be retrieved from %s: http is not supported, use https", file)
		GinkgoWriter.Printf("ERROR occurred during getting contents for file %q, error: %s\n", file, err)

		return nil, err
	} else if strings.HasPrefix(file, "https://") {
		ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.ManifestFetchTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, file, nil)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during getting contents for file %q, error: %s\n", file, err)

			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during getting contents for file %q, error: %s\n", file, err)

			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%d response when getting %s file contents", resp.StatusCode, file)
			GinkgoWriter.Printf("ERROR occurred during getting contents for file %q, error: %s\n", file, err)

			return nil, err
		}

		manifests := new(bytes.Buffer)
		count, err := manifests.ReadFrom(resp.Body)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during getting contents for file %q, error: %s\n", file, err)

			return nil, err
		}

		if resp.ContentLength != -1 && count != resp.ContentLength {
			err = fmt.Errorf("received %d bytes from %s, expected %d", count, file, resp.ContentLength)
			GinkgoWriter.Printf("ERROR occurred during getting contents for file %q, error: %s\n", file, err)

			return nil, err
		}
		return manifests, nil
	}

	if !strings.HasPrefix(file, "manifests/") {
		file = "manifests/" + file
	}

	b, err := rm.FS.ReadFile(file)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting file contents for file %q, error: %s\n", file, err)

		return nil, err
	}

	return bytes.NewBuffer(b), nil
}

// WaitForAppsToBeReady waits for all apps in the specified namespace to be ready,
// or until the ctx timeout is reached.
func (rm *ResourceManager) WaitForAppsToBeReady(namespace string, opts ...Option) error {
	GinkgoWriter.Printf("Waiting for apps to be ready in namespace %q\n", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.CreateTimeout)
	defer cancel()

	return rm.WaitForAppsToBeReadyWithCtx(ctx, namespace, opts...)
}

// WaitForAppsToBeReadyWithCtx waits for all apps in the specified namespace to be ready or
// until the provided context is canceled.
func (rm *ResourceManager) WaitForAppsToBeReadyWithCtx(ctx context.Context, namespace string, opts ...Option) error {
	if err := rm.WaitForPodsToBeReady(ctx, namespace, opts...); err != nil {
		GinkgoWriter.Printf("ERROR occurred during waiting for pods to be ready, error: %s\n", err)

		return err
	}

	if err := rm.waitForHTTPRoutesToBeReady(ctx, namespace); err != nil {
		GinkgoWriter.Printf("ERROR occurred during waiting for HTTPRoutes to be ready, error: %s\n", err)

		return err
	}

	if err := rm.waitForGRPCRoutesToBeReady(ctx, namespace); err != nil {
		GinkgoWriter.Printf("ERROR occurred during waiting for GRPCRoutes to be ready, error: %s\n", err)

		return err
	}

	gatewayReadiness := rm.waitForGatewaysToBeReady(ctx, namespace)
	if gatewayReadiness != nil {
		GinkgoWriter.Printf("ERROR occurred during waiting for Gateways to be ready, error: %s\n", gatewayReadiness)
	}

	return gatewayReadiness
}

// WaitForPodsToBeReady waits for all Pods in the specified namespace to be ready or
// until the provided context is canceled.
func (rm *ResourceManager) WaitForPodsToBeReady(
	ctx context.Context,
	namespace string,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	waitingErr := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := rm.List(
				ctx,
				&podList,
				client.InNamespace(namespace),
			); err != nil {
				return false, err
			}

			var podsReady int
			for _, pod := range podList.Items {
				for _, cond := range pod.Status.Conditions {
					if cond.Type == core.PodReady && (cond.Status == core.ConditionTrue || cond.Reason == "PodCompleted") {
						podsReady++
					}
				}
			}
			if options.logEnabled {
				GinkgoWriter.Printf("Pods ready: %d out of %d in namespace %q\n", podsReady, len(podList.Items), namespace)
			}

			return podsReady == len(podList.Items), nil
		},
	)
	if waitingErr != nil {
		GinkgoWriter.Printf(
			"ERROR occurred during waiting for Pods to be ready in namespace %q, error: %s\n",
			namespace,
			waitingErr,
		)
	}

	return waitingErr
}

func (rm *ResourceManager) waitForGatewaysToBeReady(ctx context.Context, namespace string) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var gatewayList v1.GatewayList
			if err := rm.List(
				ctx,
				&gatewayList,
				client.InNamespace(namespace),
			); err != nil {
				return false, err
			}

			for _, gw := range gatewayList.Items {
				for _, cond := range gw.Status.Conditions {
					if cond.Type == string(v1.GatewayConditionProgrammed) && cond.Status == metav1.ConditionTrue {
						return true, nil
					}
				}
			}

			return false, nil
		},
	)
}

func (rm *ResourceManager) waitForHTTPRoutesToBeReady(ctx context.Context, namespace string) error {
	GinkgoWriter.Printf("Waiting for HTTPRoutes to be ready in namespace %q\n", namespace)
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var routeList v1.HTTPRouteList
			if err := rm.List(
				ctx,
				&routeList,
				client.InNamespace(namespace),
			); err != nil {
				return false, err
			}

			var numParents, readyCount int
			for _, route := range routeList.Items {
				numParents += len(route.Spec.ParentRefs)
				readyCount += countNumberOfReadyParents(route.Status.Parents)
			}

			return numParents == readyCount, nil
		},
	)
}

func (rm *ResourceManager) waitForGRPCRoutesToBeReady(ctx context.Context, namespace string) error {
	GinkgoWriter.Printf("Waiting for GRPCRoutes to be ready in namespace %q\n", namespace)
	// First, check if grpcroute even exists for v1. If not, ignore.
	var routeList v1.GRPCRouteList
	err := rm.List(ctx, &routeList, client.InNamespace(namespace))
	if err != nil && strings.Contains(err.Error(), "no matches for kind") {
		GinkgoWriter.Printf("No GRPCRoute resources found in namespace %q, skipping wait\n", namespace)

		return nil
	}

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var routeList v1.GRPCRouteList
			if err := rm.List(
				ctx,
				&routeList,
				client.InNamespace(namespace),
			); err != nil {
				return false, err
			}

			var numParents, readyCount int
			for _, route := range routeList.Items {
				numParents += len(route.Spec.ParentRefs)
				readyCount += countNumberOfReadyParents(route.Status.Parents)
			}

			return numParents == readyCount, nil
		},
	)
}

// GetLBIPAddress gets the IP or Hostname from the Loadbalancer service.
func (rm *ResourceManager) GetLBIPAddress(namespace string) (string, error) {
	GinkgoWriter.Printf("Getting LoadBalancer IP/Hostname in namespace %q\n", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.CreateTimeout)
	defer cancel()

	var serviceList core.ServiceList
	var address string
	if err := rm.List(
		ctx, &serviceList,
		client.InNamespace(namespace),
	); err != nil {
		return "", err
	}
	var nsName types.NamespacedName

	for _, svc := range serviceList.Items {
		if svc.Spec.Type == core.ServiceTypeLoadBalancer {
			nsName = types.NamespacedName{Namespace: svc.GetNamespace(), Name: svc.GetName()}
			if err := rm.waitForLBStatusToBeReady(ctx, nsName); err != nil {
				return "", fmt.Errorf("error getting status from LoadBalancer service: %w", err)
			}
		}
	}

	if nsName.Name != "" {
		var lbService core.Service

		if err := rm.Get(ctx, nsName, &lbService); err != nil {
			return "", fmt.Errorf("error getting LoadBalancer service: %w", err)
		}
		if lbService.Status.LoadBalancer.Ingress[0].IP != "" {
			address = lbService.Status.LoadBalancer.Ingress[0].IP
		} else if lbService.Status.LoadBalancer.Ingress[0].Hostname != "" {
			address = lbService.Status.LoadBalancer.Ingress[0].Hostname
		}
		return address, nil
	}
	return "", nil
}

func (rm *ResourceManager) waitForLBStatusToBeReady(ctx context.Context, svcNsName types.NamespacedName) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var svc core.Service
			if err := rm.Get(ctx, svcNsName, &svc); err != nil {
				return false, err
			}
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				return true, nil
			}

			return false, nil
		},
	)
}

// GetClusterInfo retrieves node info and Kubernetes version from the cluster.
func (rm *ResourceManager) GetClusterInfo() (ClusterInfo, error) {
	GinkgoWriter.Printf("Getting cluster info|nodes\n")
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var nodes core.NodeList
	ci := &ClusterInfo{}
	if err := rm.List(ctx, &nodes); err != nil {
		return *ci, fmt.Errorf("error getting nodes: %w", err)
	}

	ci.NodeCount = len(nodes.Items)

	node := nodes.Items[0]
	ci.K8sVersion = node.Status.NodeInfo.KubeletVersion
	ci.CPUCountPerNode, _ = node.Status.Capacity.Cpu().AsInt64()
	ci.MemoryPerNode = node.Status.Capacity.Memory().String()
	ci.MaxPodsPerNode, _ = node.Status.Capacity.Pods().AsInt64()
	providerID := node.Spec.ProviderID

	if strings.Split(providerID, "://")[0] == "gce" {
		ci.IsGKE = true
		ci.GkeInstanceType = node.Labels["beta.kubernetes.io/instance-type"]
		ci.GkeZone = node.Labels["topology.kubernetes.io/zone"]
	}

	var ns core.Namespace
	key := types.NamespacedName{Name: "kube-system"}

	if err := rm.Get(ctx, key, &ns); err != nil {
		return *ci, fmt.Errorf("error getting kube-system namespace: %w", err)
	}

	ci.ID = string(ns.UID)

	return *ci, nil
}

// GetPodNames returns the names of all Pods in the specified namespace that match the given labels.
func (rm *ResourceManager) GetPodNames(namespace string, labels client.MatchingLabels) ([]string, error) {
	GinkgoWriter.Printf("Getting pod names in namespace %q with labels %v\n", namespace, labels)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var podList core.PodList
	if err := rm.List(
		ctx,
		&podList,
		client.InNamespace(namespace),
		labels,
	); err != nil {
		return nil, fmt.Errorf("error getting list of Pods: %w", err)
	}

	names := make([]string, 0, len(podList.Items))

	for _, pod := range podList.Items {
		names = append(names, pod.Name)
	}
	GinkgoWriter.Printf("Found pod names in namespace %q: %v\n", namespace, names)

	return names, nil
}

// GetPods returns all Pods in the specified namespace that match the given labels.
func (rm *ResourceManager) GetPods(namespace string, labels client.MatchingLabels) ([]core.Pod, error) {
	GinkgoWriter.Printf("Getting pods in namespace %q with labels %v\n", namespace, labels)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var podList core.PodList
	if err := rm.List(
		ctx,
		&podList,
		client.InNamespace(namespace),
		labels,
	); err != nil {
		return nil, fmt.Errorf("error getting list of Pods: %w", err)
	}
	GinkgoWriter.Printf("Found %d pods in namespace %q\n", len(podList.Items), namespace)

	return podList.Items, nil
}

// GetPod returns the Pod in the specified namespace with the given name.
func (rm *ResourceManager) GetPod(namespace, name string) (*core.Pod, error) {
	GinkgoWriter.Printf("Getting pod %q in namespace %q\n", name, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var pod core.Pod
	if err := rm.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		return nil, fmt.Errorf("error getting Pod: %w", err)
	}
	GinkgoWriter.Printf("Found pod %q in namespace %q\n", name, namespace)

	return &pod, nil
}

// GetPodLogs returns the logs from the specified Pod.
func (rm *ResourceManager) GetPodLogs(namespace, name string, opts *core.PodLogOptions) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	req := rm.ClientGoClient.CoreV1().Pods(namespace).GetLogs(name, opts)

	logs, err := req.Stream(ctx)
	if err != nil {
		getLogsErr := fmt.Errorf("error getting logs from Pod: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting logs from pod %q in namespace %q, error: %s\n",
			name,
			namespace,
			getLogsErr,
		)

		return "", getLogsErr
	}
	defer logs.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(logs); err != nil {
		readLogsErr := fmt.Errorf("error reading logs from Pod: %w", err)
		GinkgoWriter.Printf("ERROR occurred during reading logs from pod %q in namespace %q, error: %s\n",
			name,
			namespace,
			readLogsErr,
		)

		return "", readLogsErr
	}

	return buf.String(), nil
}

// GetNGFDeployment returns the NGF Deployment in the specified namespace with the given release name.
func (rm *ResourceManager) GetNGFDeployment(namespace, releaseName string) (*apps.Deployment, error) {
	GinkgoWriter.Printf("Getting NGF Deployment in namespace %q with release name %q\n", namespace, releaseName)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var deployments apps.DeploymentList

	if err := rm.List(
		ctx,
		&deployments,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance": releaseName,
		},
	); err != nil {
		return nil, fmt.Errorf("error getting list of Deployments: %w", err)
	}

	if len(deployments.Items) != 1 {
		deploymentsAmountErr := fmt.Errorf("expected 1 NGF Deployment, got %d", len(deployments.Items))
		GinkgoWriter.Printf("ERROR occurred during getting NGF Deployment in namespace %q with release name %q, error: %s\n",
			namespace,
			releaseName,
			deploymentsAmountErr,
		)

		return nil, deploymentsAmountErr
	}

	GinkgoWriter.Printf(
		"Found NGF Deployment %q in namespace %q with release name %q\n",
		deployments.Items[0].Name,
		namespace,
		releaseName,
	)
	deployment := deployments.Items[0]
	return &deployment, nil
}

func (rm *ResourceManager) getGatewayClassNginxProxy(
	namespace,
	releaseName string,
) (*ngfAPIv1alpha2.NginxProxy, error) {
	GinkgoWriter.Printf("Getting NginxProxy in namespace %q with release name %q\n", namespace, releaseName)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var proxy ngfAPIv1alpha2.NginxProxy
	proxyName := releaseName + "-proxy-config"

	if err := rm.Get(ctx, types.NamespacedName{Namespace: namespace, Name: proxyName}, &proxy); err != nil {
		return nil, err
	}
	GinkgoWriter.Printf("Successfully found NginxProxy %q in namespace %q\n", proxyName, namespace)

	return &proxy, nil
}

// ScaleNginxDeployment scales the Nginx Deployment to the specified number of replicas.
func (rm *ResourceManager) ScaleNginxDeployment(namespace, releaseName string, replicas int32) error {
	GinkgoWriter.Printf("Scaling Nginx Deployment in namespace %q with release name %q to %d replicas\n",
		namespace,
		releaseName,
		replicas,
	)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.UpdateTimeout)
	defer cancel()

	// If there is another NginxProxy which "overrides" the gateway class  one, then this won't work and
	// may need refactoring.
	proxy, err := rm.getGatewayClassNginxProxy(namespace, releaseName)
	if err != nil {
		getNginxProxyErr := fmt.Errorf("error getting NginxProxy: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting NginxProxy in namespace %q with release name %q, error: %s\n",
			namespace,
			releaseName,
			getNginxProxyErr,
		)

		return getNginxProxyErr
	}

	proxy.Spec.Kubernetes.Deployment.Replicas = &replicas

	if err = rm.Update(ctx, proxy, nil); err != nil {
		return fmt.Errorf("error updating NginxProxy: %w", err)
	}

	GinkgoWriter.Printf("Successfully scaled Nginx Deployment in namespace %q with release name %q to %d replicas\n",
		namespace,
		releaseName,
		replicas,
	)

	return nil
}

// GetEvents returns all Events in the specified namespace.
func (rm *ResourceManager) GetEvents(namespace string) (*core.EventList, error) {
	GinkgoWriter.Printf("Getting Events in namespace %q\n", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var eventList core.EventList
	if err := rm.List(
		ctx,
		&eventList,
		client.InNamespace(namespace),
	); err != nil {
		return &core.EventList{}, fmt.Errorf("error getting list of Events: %w", err)
	}
	GinkgoWriter.Printf("Successfully found %d Events in namespace %q\n", len(eventList.Items), namespace)

	return &eventList, nil
}

// ScaleDeployment scales the Deployment to the specified number of replicas.
func (rm *ResourceManager) ScaleDeployment(namespace, name string, replicas int32) error {
	GinkgoWriter.Printf("Scaling Deployment %q in namespace %q to %d replicas\n", name, namespace, replicas)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.UpdateTimeout)
	defer cancel()

	var deployment apps.Deployment
	if err := rm.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		return fmt.Errorf("error getting Deployment: %w", err)
	}

	deployment.Spec.Replicas = &replicas
	if err := rm.Update(ctx, &deployment, nil); err != nil {
		return fmt.Errorf("error updating Deployment: %w", err)
	}
	GinkgoWriter.Printf("Successfully scaled Deployment %q in namespace %q to %d replicas\n", name, namespace, replicas)

	return nil
}

// GetReadyNGFPodNames returns the name(s) of the NGF Pod(s).
func (rm *ResourceManager) GetReadyNGFPodNames(
	namespace,
	releaseName string,
	timeout time.Duration,
	opts ...Option,
) ([]string, error) {
	options := LogOptions(opts...)
	if options.logEnabled {
		GinkgoWriter.Printf("Getting ready NGF Pod names in namespace %q with release name %q\n", namespace, releaseName)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var ngfPodNames []string

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, // poll immediately
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := rm.List(
				ctx,
				&podList,
				client.InNamespace(namespace),
				client.MatchingLabels{
					"app.kubernetes.io/instance": releaseName,
				},
			); err != nil {
				return false, fmt.Errorf("error getting list of NGF Pods: %w", err)
			}

			ngfPodNames = getReadyPodNames(podList, opts...)
			return len(ngfPodNames) > 0, nil
		},
	)
	if err != nil {
		waitingPodsErr := fmt.Errorf("timed out waiting for NGF Pods to be ready: %w", err)
		if options.logEnabled {
			GinkgoWriter.Printf(
				"ERROR occurred during waiting for NGF Pods to be ready in namespace %q with release name %q, error: %s\n",
				namespace,
				releaseName,
				waitingPodsErr,
			)
		}

		return nil, waitingPodsErr
	}
	if options.logEnabled {
		GinkgoWriter.Printf(
			"Successfully found ready NGF Pod names in namespace %q with release name %q: %v\n",
			namespace,
			releaseName,
			ngfPodNames,
		)
	}

	return ngfPodNames, nil
}

// GetReadyNginxPodNames returns the name(s) of the NGINX Pod(s).
func (rm *ResourceManager) GetReadyNginxPodNames(
	namespace string,
	timeout time.Duration,
	opts ...Option,
) ([]string, error) {
	options := LogOptions(opts...)
	if options.logEnabled {
		GinkgoWriter.Printf("Getting ready NGINX Pod names in namespace %q\n", namespace)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var nginxPodNames []string

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, // poll immediately
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := rm.List(
				ctx,
				&podList,
				client.InNamespace(namespace),
			); err != nil {
				return false, fmt.Errorf("error getting list of NGINX Pods: %w", err)
			}

			var filteredPodList core.PodList
			for _, pod := range podList.Items {
				if _, ok := pod.Annotations["gateway.networking.k8s.io/gateway-name"]; ok {
					filteredPodList.Items = append(filteredPodList.Items, pod)
				}
			}

			nginxPodNames = getReadyPodNames(filteredPodList, opts...)
			return len(nginxPodNames) > 0, nil
		},
	)
	if err != nil {
		waitingPodsErr := fmt.Errorf("timed out waiting for NGINX Pods to be ready: %w", err)
		if options.logEnabled {
			GinkgoWriter.Printf("ERROR occurred during waiting for NGINX Pods to be ready in namespace %q, error: %s\n",
				namespace,
				waitingPodsErr,
			)
		}

		return nil, waitingPodsErr
	}
	if options.logEnabled {
		GinkgoWriter.Printf(
			"Successfully found ready NGINX Pod name(s) in namespace %q: %v\n",
			namespace,
			nginxPodNames,
		)
	}

	return nginxPodNames, nil
}

func getReadyPodNames(podList core.PodList, opts ...Option) []string {
	var names []string
	for _, pod := range podList.Items {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == core.PodReady && cond.Status == core.ConditionTrue {
				names = append(names, pod.Name)
			}
		}
	}
	options := LogOptions(opts...)
	if options.logEnabled {
		GinkgoWriter.Printf("Found %d ready pod name(s): %v\n", len(names), names)
	}

	return names
}

func countNumberOfReadyParents(parents []v1.RouteParentStatus) int {
	readyCount := 0

	for _, parent := range parents {
		for _, cond := range parent.Conditions {
			if cond.Type == string(v1.RouteConditionAccepted) && cond.Status == metav1.ConditionTrue {
				readyCount++
			}
		}
	}
	GinkgoWriter.Printf("Found %d ready parent(s)\n", readyCount)

	return readyCount
}

// WaitForPodsToBeReadyWithCount waits for all Pods in the specified namespace to be ready or
// until the provided context is canceled.
func (rm *ResourceManager) WaitForPodsToBeReadyWithCount(
	ctx context.Context,
	namespace string,
	count int,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	GinkgoWriter.Printf("Waiting for %d pods to be ready in namespace %q\n", count, namespace)

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := rm.List(
				ctx,
				&podList,
				client.InNamespace(namespace),
			); err != nil {
				return false, err
			}

			var podsReady int
			for _, pod := range podList.Items {
				for _, cond := range pod.Status.Conditions {
					if cond.Type == core.PodReady && cond.Status == core.ConditionTrue {
						podsReady++
					}
				}
			}
			if options.logEnabled {
				GinkgoWriter.Printf("Found %d/%d ready pods in namespace %q\n", podsReady, count, namespace)
			}

			return podsReady == count, nil
		},
	)
}

// WaitForGatewayObservedGeneration waits for the provided Gateway's ObservedGeneration to equal the expected value.
func (rm *ResourceManager) WaitForGatewayObservedGeneration(
	ctx context.Context,
	namespace,
	name string,
	generation int,
) error {
	GinkgoWriter.Printf("Waiting for Gateway %q in namespace %q to have ObservedGeneration %d\n",
		name,
		namespace,
		generation,
	)
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var gw v1.Gateway
			key := types.NamespacedName{Namespace: namespace, Name: name}
			if err := rm.Get(ctx, key, &gw); err != nil {
				return false, err
			}

			for _, cond := range gw.Status.Conditions {
				if cond.ObservedGeneration == int64(generation) {
					return true, nil
				}
			}

			return false, nil
		},
	)
}

// GetNginxConfig uses crossplane to get the nginx configuration and convert it to JSON.
// If the crossplane image is loaded locally on the node, crossplaneImageRepo can be empty.
func (rm *ResourceManager) GetNginxConfig(
	nginxPodName,
	namespace,
	crossplaneImageRepo string,
	opts ...Option,
) (*Payload, error) {
	GinkgoWriter.Printf("Getting NGINX config from pod %q in namespace %q\n", nginxPodName, namespace)
	options := LogOptions(opts...)

	if err := injectCrossplaneContainer(
		rm.ClientGoClient,
		rm.TimeoutConfig.UpdateTimeout,
		nginxPodName,
		namespace,
		crossplaneImageRepo,
	); err != nil {
		GinkgoWriter.Printf("ERROR occurred during injecting crossplane container, error: %s\n", err)

		return nil, err
	}

	exec, err := createCrossplaneExecutor(rm.ClientGoClient, rm.K8sConfig, nginxPodName, namespace)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during creating crossplane executor, error: %s\n", err)

		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.RequestTimeout)
	defer cancel()

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	if err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
				Stdout: buf,
				Stderr: errBuf,
			}); err != nil {
				return false, nil //nolint:nilerr // we want to retry if there's an error
			}

			if errBuf.String() != "" {
				return false, nil
			}

			return true, nil
		},
	); err != nil {
		containerErr := fmt.Errorf("could not connect to ephemeral container: %w", err)
		if options.logEnabled {
			GinkgoWriter.Printf("ERROR occurred during waiting for NGINX Pods to be ready in namespace %q, error: %s\n",
				namespace,
				containerErr,
			)
		}

		return nil, containerErr
	}

	conf := &Payload{}
	if err := json.Unmarshal(buf.Bytes(), conf); err != nil {
		unmarshalErr := fmt.Errorf("error unmarshaling nginx config: %w", err)
		GinkgoWriter.Printf("ERROR occurred during unmarshaling nginx config from pod %q in namespace %q, error: %s\n",
			nginxPodName,
			namespace,
			unmarshalErr,
		)

		return nil, unmarshalErr
	}
	GinkgoWriter.Printf("Successfully got NGINX config from pod %q in namespace %q\n", nginxPodName, namespace)

	return conf, nil
}

// Get retrieves a resource by key, logging errors if enabled.
func (rm *ResourceManager) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	if err := rm.K8sClient.Get(ctx, key, obj); err != nil {
		// Don't log NotFound errors - they're often expected (e.g., when checking if resource was deleted)
		if options.logEnabled && !apierrors.IsNotFound(err) {
			GinkgoWriter.Printf("Could not get k8s resource %q error: %v\n", obj.GetName(), err)
		}

		return err
	}

	return nil
}

// Create adds a new resource, returning an error on failure.
func (rm *ResourceManager) Create(
	ctx context.Context,
	obj client.Object,
) error {
	if err := rm.K8sClient.Create(ctx, obj); err != nil {
		createErr := fmt.Errorf("error creating k8s resource %q: %w", obj.GetName(), err)
		GinkgoWriter.Printf("%v\n", createErr)

		return createErr
	}
	return nil
}

// Delete removes a resource, returning an error on failure.
func (rm *ResourceManager) Delete(
	ctx context.Context,
	obj client.Object,
	deleteOpts []client.DeleteOption,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	if err := rm.K8sClient.Delete(ctx, obj, deleteOpts...); err != nil {
		if options.logEnabled {
			GinkgoWriter.Printf("Could not delete k8s resource %q: %w\n", obj.GetName(), err)
		}

		return err
	}
	return nil
}

// Update modifies a resource.
func (rm *ResourceManager) Update(
	ctx context.Context,
	obj client.Object,
	updateOpts []client.UpdateOption,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	if err := rm.K8sClient.Update(ctx, obj, updateOpts...); err != nil {
		updateResourceErr := fmt.Errorf("error updating k8s resource: %w", err)
		if options.logEnabled {
			GinkgoWriter.Printf(
				"ERROR occurred during updating k8s resource in namespace %q with name %q, error: %s\n",
				obj.GetNamespace(),
				obj.GetName(),
				updateResourceErr,
			)
		}

		return updateResourceErr
	}

	return nil
}

// List retrieves a list of resources, returning an error on failure.
func (rm *ResourceManager) List(
	ctx context.Context,
	list client.ObjectList,
	listOpts ...client.ListOption,
) error {
	if err := rm.K8sClient.List(ctx, list, listOpts...); err != nil {
		listErr := fmt.Errorf("error listing k8s resources: %w", err)
		GinkgoWriter.Printf("%v\n", listErr)

		return listErr
	}
	return nil
}
