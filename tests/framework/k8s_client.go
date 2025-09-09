package framework

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8sClient wraps controller-runtime Client to add logging and custom behavior.
type K8sClient struct {
	inner client.Client
}

func (c *K8sClient) InNamespace(namespace string) client.ListOption {
	return client.InNamespace(namespace)
}

// HasLabels returns a ListOption that filters for objects which have
// all of the given label keys (regardless of value).
func (c *K8sClient) HasLabels(keys ...string) client.ListOption {
	reqs := make([]metav1.LabelSelectorRequirement, 0, len(keys))
	for _, k := range keys {
		reqs = append(reqs, metav1.LabelSelectorRequirement{
			Key:      k,
			Operator: metav1.LabelSelectorOpExists,
		})
	}
	sel := &metav1.LabelSelector{MatchExpressions: reqs}
	ls, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		GinkgoWriter.Printf("error constructing label selector: %v\n", err)
		// fallback to a selector that matches nothing
		return client.MatchingLabelsSelector{Selector: labels.Nothing()}
	}
	return client.MatchingLabelsSelector{Selector: ls}
}

// MatchingLabels is just a passthrough to the real helper.
func (c *K8sClient) MatchingLabels(m map[string]string) client.ListOption {
	return client.MatchingLabels(m)
}

func logOptions(opts ...Option) *Options {
	options := &Options{logEnabled: true}
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// NewK8sClient returns a new wrapped Kubernetes client.
func NewK8sClient(config *rest.Config, options client.Options) (K8sClient, error) {
	inner, err := client.New(config, options)
	if err != nil {
		clientErr := fmt.Errorf("error creating k8s client: %w", err)
		GinkgoWriter.Printf("%v\n", clientErr)

		return K8sClient{}, err
	}

	return K8sClient{inner: inner}, nil
}

// Get retrieves a resource by key, logging errors if enabled.
func (c *K8sClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...Option,
) error {
	options := logOptions(opts...)
	err := c.inner.Get(ctx, key, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if options.logEnabled {
				GinkgoWriter.Printf("Not found k8s resource %q error: %v\n", obj.GetName(), err)
			}
			return err
		}
		getErr := fmt.Errorf("error getting k8s resource %q: %w", obj.GetName(), err)
		if options.logEnabled {
			GinkgoWriter.Printf("%v\n", getErr)
		}

		return getErr
	}

	return nil
}

// Create adds a new resource, returning an error on failure.
func (c *K8sClient) Create(
	ctx context.Context,
	obj client.Object,
) error {
	err := c.inner.Create(ctx, obj)
	if err != nil {
		createErr := fmt.Errorf("error creating k8s resource %q: %w", obj.GetName(), err)
		GinkgoWriter.Printf("%v\n", createErr)

		return createErr
	}
	return nil
}

// Delete removes a resource, returning an error on failure.
func (c *K8sClient) Delete(
	ctx context.Context,
	obj client.Object,
	deleteOpts []client.DeleteOption,
	opts ...Option,
) error {
	options := logOptions(opts...)
	var dOpts []client.DeleteOption
	for _, do := range deleteOpts {
		if do != nil {
			dOpts = append(dOpts, do)
		}
	}

	err := c.inner.Delete(ctx, obj, dOpts...)
	if err != nil {
		deleteErr := fmt.Errorf("error deleting k8s resource %q: %w", obj.GetName(), err)
		if options.logEnabled {
			GinkgoWriter.Printf("%v\n", deleteErr)
		}

		return deleteErr
	}
	return nil
}

// Update modifies a resource.
func (c *K8sClient) Update(
	ctx context.Context,
	obj client.Object,
	updateOpts []client.UpdateOption,
	opts ...Option,
) error {
	options := logOptions(opts...)
	var uOpts []client.UpdateOption
	for _, uo := range updateOpts {
		if uo != nil {
			uOpts = append(uOpts, uo)
		}
	}

	if err := c.inner.Update(ctx, obj, uOpts...); err != nil {
		updateDeploymentErr := fmt.Errorf("error updating Deployment: %w", err)
		if options.logEnabled {
			GinkgoWriter.Printf(
				"ERROR occurred during updating Deployment in namespace %q with name %q, error: %s\n",
				obj.GetNamespace(),
				obj.GetName(),
				updateDeploymentErr,
			)
		}

		return updateDeploymentErr
	}

	return nil
}

// List retrieves a list of resources, returning an error on failure.
func (c *K8sClient) List(
	ctx context.Context,
	list client.ObjectList,
	listOpts ...client.ListOption,
) error {
	var opts []client.ListOption
	for _, o := range listOpts {
		if o != nil {
			opts = append(opts, o)
		}
	}

	err := c.inner.List(ctx, list, opts...)
	if err != nil {
		listErr := fmt.Errorf("error listing k8s resources: %w", err)
		GinkgoWriter.Printf("%v\n", listErr)

		return listErr
	}
	return nil
}
