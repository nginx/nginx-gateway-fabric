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
func (rm *ResourceManager) Apply(resources []client.Object) error {
	GinkgoWriter.Printf("Applying resources defined as Go objects\n")
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

		if err := rm.K8sClient.Get(ctx, client.ObjectKeyFromObject(resource), obj); err != nil {
			if !apierrors.IsNotFound(err) {
				notFoundErr := fmt.Errorf("error getting resource: %w", err)
				GinkgoWriter.Printf(
					"ERROR occurred during getting Kubernetes resources: %s\n",
					notFoundErr,
				)

				return notFoundErr
			}

			if err := rm.K8sClient.Create(ctx, resource); err != nil {
				creatingResourceErr := fmt.Errorf("error creating resource: %w", err)
				GinkgoWriter.Printf(
					"ERROR occurred during applying creates Kubernetes resources: %s\n",
					creatingResourceErr,
				)

				return creatingResourceErr
			}

			continue
		}

		// Some tests modify resources that are also modified by NGF (to update their status), so conflicts are possible
		// For example, a Gateway resource.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := rm.K8sClient.Get(ctx, client.ObjectKeyFromObject(resource), obj); err != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during getting Kubernetes resources on retries: %s\n",
					err,
				)

				return err
			}
			resource.SetResourceVersion(obj.GetResourceVersion())
			updateErr := rm.K8sClient.Update(ctx, resource)
			if updateErr != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during updating Kubernetes resources on retries: %s\n",
					updateErr,
				)
			}

			return updateErr
		})
		if err != nil {
			retryErr := fmt.Errorf("error updating resource: %w", err)
			GinkgoWriter.Printf(
				"ERROR occurred during retries: %s\n",
				retryErr,
			)

			return retryErr
		}
	}
	GinkgoWriter.Printf("Resources defined as Go objects applied successfully\n")

	return nil
}

// ApplyFromFiles creates or updates Kubernetes resources defined within the provided YAML files.
func (rm *ResourceManager) ApplyFromFiles(files []string, namespace string) error {
	for _, file := range files {
		GinkgoWriter.Printf("Applying resources from file: %q to namespace %q\n", file, namespace)
		data, err := rm.GetFileContents(file)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during getting file contents for file %q, error: %s\n", file, err)

			return err
		}

		if err = rm.ApplyFromBuffer(data, namespace); err != nil {
			GinkgoWriter.Printf("ERROR occurred during applying resources from file %q, error: %s\n", file, err)

			return err
		}
	}
	GinkgoWriter.Printf("Resources from files applied successfully to namespace %q,\n", namespace)

	return nil
}

func (rm *ResourceManager) ApplyFromBuffer(buffer *bytes.Buffer, namespace string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.CreateTimeout)
	defer cancel()

	handlerFunc := func(obj unstructured.Unstructured) error {
		obj.SetNamespace(namespace)
		nsName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
		fetchedObj := obj.DeepCopy()
		if err := rm.K8sClient.Get(ctx, nsName, fetchedObj); err != nil {
			if !apierrors.IsNotFound(err) {
				getResourceErr := fmt.Errorf("error getting resource: %w", err)
				GinkgoWriter.Printf("ERROR occurred during getting resource from buffer, error: %s\n", getResourceErr)

				return getResourceErr
			}

			if err := rm.K8sClient.Create(ctx, &obj); err != nil {
				createResourceErr := fmt.Errorf("error creating resource: %w", err)
				GinkgoWriter.Printf("ERROR occurred during creating resource from buffer, error: %s\n", createResourceErr)

				return createResourceErr
			}

			return nil
		}

		// Some tests modify resources that are also modified by NGF (to update their status), so conflicts are possible
		// For example, a Gateway resource.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := rm.K8sClient.Get(ctx, nsName, fetchedObj); err != nil {
				GinkgoWriter.Printf(
					"ERROR occurred during getting resource from buffer on retries, error: %s\n",
					err,
				)

				return err
			}
			obj.SetResourceVersion(fetchedObj.GetResourceVersion())
			updateErr := rm.K8sClient.Update(ctx, &obj)
			if updateErr != nil {
				GinkgoWriter.Printf("ERROR occurred during updating resource from buffer, error: %s\n", updateErr)
			}

			return updateErr
		})
		if err != nil {
			retryErr := fmt.Errorf("error updating resource: %w", err)
			GinkgoWriter.Printf(
				"ERROR occurred during retries, while update from buffer error: %s\n",
				retryErr,
			)

			return retryErr
		}

		return nil
	}

	return rm.readAndHandleObject(handlerFunc, buffer)
}

// Delete deletes Kubernetes resources defined as Go objects.
func (rm *ResourceManager) Delete(resources []client.Object, opts ...client.DeleteOption) error {
	GinkgoWriter.Printf("Deleting resources\n")
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteTimeout)
	defer cancel()

	for _, resource := range resources {
		if err := rm.K8sClient.Delete(ctx, resource, opts...); err != nil && !apierrors.IsNotFound(err) {
			delErr := fmt.Errorf("error deleting resource: %w", err)
			GinkgoWriter.Printf("ERROR occurred during deleting resource, error: %s\n", delErr)

			return delErr
		}
	}
	GinkgoWriter.Printf("Resources deleted successfully\n")

	return nil
}

func (rm *ResourceManager) DeleteNamespace(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteNamespaceTimeout)
	GinkgoWriter.Printf("Deleting namespace %q\n", name)
	defer cancel()

	ns := &core.Namespace{}
	if err := rm.K8sClient.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			GinkgoWriter.Printf("Namespace %q not found, nothing to delete\n", name)

			return nil
		}
		getNsErr := fmt.Errorf("error getting namespace: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting namespace, error: %s\n", getNsErr)

		return getNsErr
	}

	if err := rm.K8sClient.Delete(ctx, ns); err != nil {
		delErr := fmt.Errorf("error deleting namespace: %w", err)
		GinkgoWriter.Printf("ERROR occurred during deleting namespace, error: %s\n", delErr)

		return delErr
	}

	GinkgoWriter.Printf("Waiting for namespace %q to be deleted\n", name)
	// Because the namespace deletion is asynchronous, we need to wait for the namespace to be deleted.
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			if err := rm.K8sClient.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
				if apierrors.IsNotFound(err) {
					GinkgoWriter.Printf("Namespace %q not found (deleted)\n", name)

					return true, nil
				}
				getNsErr := fmt.Errorf("error getting namespace: %w", err)
				GinkgoWriter.Printf("ERROR occurred during getting namespace, error: %s\n", getNsErr)

				return false, getNsErr
			}

			return false, nil
		})
}

func (rm *ResourceManager) DeleteNamespaces(names []string) error {
	GinkgoWriter.Printf("Deleting namespaces: %v\n", names)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.DeleteNamespaceTimeout*2)
	defer cancel()

	var combinedErrors error
	for _, name := range names {
		ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

		if err := rm.K8sClient.Delete(ctx, ns); err != nil {
			if apierrors.IsNotFound(err) {
				GinkgoWriter.Printf("Namespace %q not found, nothing to delete\n", name)
				continue
			}
			delNsErr := fmt.Errorf("error deleting namespace: %w", err)
			GinkgoWriter.Printf("ERROR occurred during deleting namespace %q, error: %s\n", name, delNsErr)

			combinedErrors = errors.Join(combinedErrors, delNsErr)
		}
	}

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			nsList := &core.NamespaceList{}
			if err := rm.K8sClient.List(ctx, nsList); err != nil {
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

		if err := rm.K8sClient.Delete(ctx, &obj); err != nil && !apierrors.IsNotFound(err) {
			GinkgoWriter.Printf("ERROR occurred during deleting resource from file, error: %s\n", err)

			return err
		}

		return nil
	}

	for _, file := range files {
		data, err := rm.GetFileContents(file)
		if err != nil {
			GinkgoWriter.Printf("ERROR occurred during getting file contents for file %q, error: %s\n", file, err)

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
		return nil, fmt.Errorf("data can't be retrieved from %s: http is not supported, use https", file)
	} else if strings.HasPrefix(file, "https://") {
		ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.ManifestFetchTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, file, nil)
		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%d response when getting %s file contents", resp.StatusCode, file)
		}

		manifests := new(bytes.Buffer)
		count, err := manifests.ReadFrom(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.ContentLength != -1 && count != resp.ContentLength {
			return nil, fmt.Errorf("received %d bytes from %s, expected %d", count, file, resp.ContentLength)
		}
		return manifests, nil
	}

	if !strings.HasPrefix(file, "manifests/") {
		file = "manifests/" + file
	}

	b, err := rm.FS.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(b), nil
}

// WaitForAppsToBeReady waits for all apps in the specified namespace to be ready,
// or until the ctx timeout is reached.
func (rm *ResourceManager) WaitForAppsToBeReady(namespace string) error {
	GinkgoWriter.Printf("Waiting for apps to be ready in namespace %q\n", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.CreateTimeout)
	defer cancel()

	return rm.WaitForAppsToBeReadyWithCtx(ctx, namespace)
}

// WaitForAppsToBeReadyWithCtx waits for all apps in the specified namespace to be ready or
// until the provided context is canceled.
func (rm *ResourceManager) WaitForAppsToBeReadyWithCtx(ctx context.Context, namespace string) error {
	if err := rm.WaitForPodsToBeReady(ctx, namespace); err != nil {
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
func (rm *ResourceManager) WaitForPodsToBeReady(ctx context.Context, namespace string) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := rm.K8sClient.List(ctx, &podList, client.InNamespace(namespace)); err != nil {
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

			return podsReady == len(podList.Items), nil
		},
	)
}

func (rm *ResourceManager) waitForGatewaysToBeReady(ctx context.Context, namespace string) error {
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var gatewayList v1.GatewayList
			if err := rm.K8sClient.List(ctx, &gatewayList, client.InNamespace(namespace)); err != nil {
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
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var routeList v1.HTTPRouteList
			if err := rm.K8sClient.List(ctx, &routeList, client.InNamespace(namespace)); err != nil {
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
	// First, check if grpcroute even exists for v1. If not, ignore.
	var routeList v1.GRPCRouteList
	err := rm.K8sClient.List(ctx, &routeList, client.InNamespace(namespace))
	if err != nil && strings.Contains(err.Error(), "no matches for kind") {
		return nil
	}

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var routeList v1.GRPCRouteList
			if err := rm.K8sClient.List(ctx, &routeList, client.InNamespace(namespace)); err != nil {
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
	if err := rm.K8sClient.List(ctx, &serviceList, client.InNamespace(namespace)); err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting list of services in namespace %q, error: %s\n",
			namespace, err)

		return "", err
	}
	var nsName types.NamespacedName

	for _, svc := range serviceList.Items {
		if svc.Spec.Type == core.ServiceTypeLoadBalancer {
			nsName = types.NamespacedName{Namespace: svc.GetNamespace(), Name: svc.GetName()}
			if err := rm.waitForLBStatusToBeReady(ctx, nsName); err != nil {
				lbStatusErr := fmt.Errorf("error getting status from LoadBalancer service: %w", err)
				GinkgoWriter.Printf(
					"ERROR occurred during waiting for LoadBalancer service in namespace %q to be ready, error: %s\n",
					nsName,
					err,
				)

				return "", lbStatusErr
			}
		}
	}

	if nsName.Name != "" {
		var lbService core.Service

		if err := rm.K8sClient.Get(ctx, nsName, &lbService); err != nil {
			getLBStatusErr := fmt.Errorf("error getting LoadBalancer service: %w", err)
			GinkgoWriter.Printf("ERROR occurred during getting LoadBalancer service in namespace %q, error: %s\n",
				nsName,
				err,
			)

			return "", getLBStatusErr
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
			if err := rm.K8sClient.Get(ctx, svcNsName, &svc); err != nil {
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
	GinkgoWriter.Printf("Getting cluster info\n")
	ctx, cancel := context.WithTimeout(context.Background(), rm.TimeoutConfig.GetTimeout)
	defer cancel()

	var nodes core.NodeList
	ci := &ClusterInfo{}
	if err := rm.K8sClient.List(ctx, &nodes); err != nil {
		getNodesErr := fmt.Errorf("error getting nodes: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting nodes in cluster, error: %s\n",
			getNodesErr,
		)

		return *ci, getNodesErr
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

	if err := rm.K8sClient.Get(ctx, key, &ns); err != nil {
		getK8sNamespaceErr := fmt.Errorf("error getting kube-system namespace: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting kube-system namespace, error: %s\n",
			getK8sNamespaceErr,
		)

		return *ci, getK8sNamespaceErr
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
	if err := rm.K8sClient.List(
		ctx,
		&podList,
		client.InNamespace(namespace),
		labels,
	); err != nil {
		getPodsErr := fmt.Errorf("error getting list of Pods: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting list of Pods in namespace %q, error: %s\n",
			namespace,
			getPodsErr,
		)

		return nil, getPodsErr
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
	if err := rm.K8sClient.List(
		ctx,
		&podList,
		client.InNamespace(namespace),
		labels,
	); err != nil {
		getPodsErr := fmt.Errorf("error getting list of Pods: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting list of Pods in namespace %q, error: %s\n",
			namespace,
			getPodsErr,
		)

		return nil, getPodsErr
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
	if err := rm.K8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		getPodErr := fmt.Errorf("error getting Pod: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting Pod %q in namespace %q, error: %s\n",
			name,
			namespace,
			getPodErr,
		)

		return nil, getPodErr
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

	if err := rm.K8sClient.List(
		ctx,
		&deployments,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance": releaseName,
		},
	); err != nil {
		getDeploymentsErr := fmt.Errorf("error getting list of Deployments: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting list of Deployments in namespace %q with release %q, error: %s\n",
			namespace,
			releaseName,
			getDeploymentsErr,
		)

		return nil, getDeploymentsErr
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

	if err := rm.K8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: proxyName}, &proxy); err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting NginxProxy %q in namespace %q, error: %s\n",
			proxyName,
			namespace,
			err,
		)

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

	if err = rm.K8sClient.Update(ctx, proxy); err != nil {
		updateNginxProxyErr := fmt.Errorf("error updating NginxProxy: %w", err)
		GinkgoWriter.Printf("ERROR occurred during updating NginxProxy in namespace %q with release name %q, error: %s\n",
			namespace,
			releaseName,
			updateNginxProxyErr,
		)

		return updateNginxProxyErr
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
	if err := rm.K8sClient.List(
		ctx,
		&eventList,
		client.InNamespace(namespace),
	); err != nil {
		getEventsListErr := fmt.Errorf("error getting list of Events: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting Events in namespace %q, error: %s\n",
			namespace,
			getEventsListErr,
		)

		return &core.EventList{}, getEventsListErr
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
	if err := rm.K8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		getDeploymentErr := fmt.Errorf("error getting Deployment: %w", err)
		GinkgoWriter.Printf("ERROR occurred during getting Deployment in namespace %q with name %q, error: %s\n",
			namespace,
			name,
			getDeploymentErr,
		)

		return getDeploymentErr
	}

	deployment.Spec.Replicas = &replicas
	if err := rm.K8sClient.Update(ctx, &deployment); err != nil {
		updateDeploymentErr := fmt.Errorf("error updating Deployment: %w", err)
		GinkgoWriter.Printf("ERROR occurred during updating Deployment in namespace %q with name %q, error: %s\n",
			namespace,
			name,
			updateDeploymentErr,
		)

		return updateDeploymentErr
	}
	GinkgoWriter.Printf("Successfully scaled Deployment %q in namespace %q to %d replicas\n", name, namespace, replicas)

	return nil
}

// GetReadyNGFPodNames returns the name(s) of the NGF Pod(s).
func GetReadyNGFPodNames(
	k8sClient client.Client,
	namespace,
	releaseName string,
	timeout time.Duration,
) ([]string, error) {
	GinkgoWriter.Printf("Getting ready NGF Pod names in namespace %q with release name %q\n", namespace, releaseName)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var ngfPodNames []string

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, // poll immediately
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := k8sClient.List(
				ctx,
				&podList,
				client.InNamespace(namespace),
				client.MatchingLabels{
					"app.kubernetes.io/instance": releaseName,
				},
			); err != nil {
				return false, fmt.Errorf("error getting list of NGF Pods: %w", err)
			}

			ngfPodNames = getReadyPodNames(podList)
			return len(ngfPodNames) > 0, nil
		},
	)
	if err != nil {
		waitingPodsErr := fmt.Errorf("timed out waiting for NGF Pods to be ready: %w", err)
		GinkgoWriter.Printf(
			"ERROR occurred during waiting for NGF Pods to be ready in namespace %q with release name %q, error: %s\n",
			namespace,
			releaseName,
			waitingPodsErr,
		)

		return nil, waitingPodsErr
	}
	GinkgoWriter.Printf(
		"Successfully found ready NGF Pod names in namespace %q with release name %q: %v\n",
		namespace,
		releaseName,
		ngfPodNames,
	)

	return ngfPodNames, nil
}

// GetReadyNginxPodNames returns the name(s) of the NGINX Pod(s).
func GetReadyNginxPodNames(
	k8sClient client.Client,
	namespace string,
	timeout time.Duration,
) ([]string, error) {
	GinkgoWriter.Printf("Getting ready NGINX Pod names in namespace %q\n", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var nginxPodNames []string

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, // poll immediately
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := k8sClient.List(
				ctx,
				&podList,
				client.InNamespace(namespace),
				client.HasLabels{"gateway.networking.k8s.io/gateway-name"},
			); err != nil {
				return false, fmt.Errorf("error getting list of NGINX Pods: %w", err)
			}

			nginxPodNames = getReadyPodNames(podList)
			return len(nginxPodNames) > 0, nil
		},
	)
	if err != nil {
		waitingPodsErr := fmt.Errorf("timed out waiting for NGINX Pods to be ready: %w", err)
		GinkgoWriter.Printf("ERROR occurred during waiting for NGINX Pods to be ready in namespace %q, error: %s\n",
			namespace,
			waitingPodsErr,
		)

		return nil, waitingPodsErr
	}
	GinkgoWriter.Printf(
		"Successfully found ready NGINX Pod names in namespace %q: %v\n",
		namespace,
		nginxPodNames,
	)

	return nginxPodNames, nil
}

func getReadyPodNames(podList core.PodList) []string {
	var names []string
	for _, pod := range podList.Items {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == core.PodReady && cond.Status == core.ConditionTrue {
				names = append(names, pod.Name)
			}
		}
	}
	GinkgoWriter.Printf("Found %d ready pod names: %v\n", len(names), names)

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
func (rm *ResourceManager) WaitForPodsToBeReadyWithCount(ctx context.Context, namespace string, count int) error {
	GinkgoWriter.Printf("Waiting for %d pods to be ready in namespace %q\n", count, namespace)
	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var podList core.PodList
			if err := rm.K8sClient.List(ctx, &podList, client.InNamespace(namespace)); err != nil {
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
			GinkgoWriter.Printf("Found %d/%d ready pods in namespace %q\n", podsReady, count, namespace)

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
			if err := rm.K8sClient.Get(ctx, key, &gw); err != nil {
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
func (rm *ResourceManager) GetNginxConfig(nginxPodName, namespace, crossplaneImageRepo string) (*Payload, error) {
	GinkgoWriter.Printf("Getting NGINX config from pod %q in namespace %q\n", nginxPodName, namespace)
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
		GinkgoWriter.Printf("ERROR occurred during waiting for NGINX Pods to be ready in namespace %q, error: %s\n",
			namespace,
			containerErr,
		)

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
