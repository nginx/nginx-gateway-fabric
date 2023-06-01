package provisioner

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/events"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/conditions"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/status"
)

// eventHandler ensures each Gateway for the specific GatewayClass has a corresponding Deployment
// of NKG configured to use that specific Gateway.
//
// eventHandler implements events.Handler interface.
type eventHandler struct {
	gcName string
	store  *store

	// provisions maps NamespacedName of Gateway to its corresponding Deployment
	provisions map[types.NamespacedName]*v1.Deployment

	statusUpdater status.Updater
	k8sClient     client.Client
	logger        logr.Logger

	staticModeDeploymentYAML []byte
}

func newEventHandler(
	gcName string,
	statusUpdater status.Updater,
	k8sClient client.Client,
	logger logr.Logger,
	staticModeDeploymentYAML []byte,
) *eventHandler {
	return &eventHandler{
		store:                    newStore(),
		provisions:               make(map[types.NamespacedName]*v1.Deployment),
		statusUpdater:            statusUpdater,
		gcName:                   gcName,
		k8sClient:                k8sClient,
		logger:                   logger,
		staticModeDeploymentYAML: staticModeDeploymentYAML,
	}
}

func (h *eventHandler) ensureGatewayClassAccepted(ctx context.Context) {
	gc, exist := h.store.gatewayClasses[types.NamespacedName{Name: h.gcName}]
	if !exist {
		panic(fmt.Errorf("GatewayClass %s must exist", h.gcName))
	}

	statuses := status.Statuses{
		GatewayClassStatus: &status.GatewayClassStatus{
			Conditions:         conditions.NewDefaultGatewayClassConditions(),
			ObservedGeneration: gc.Generation,
		},
	}

	h.statusUpdater.Update(ctx, statuses)
}

func (h *eventHandler) ensureDeploymentsMatchGateways(ctx context.Context) {
	var gwsWithoutDeps, removedGwsWithDeps []types.NamespacedName

	for nsname, gw := range h.store.gateways {
		if string(gw.Spec.GatewayClassName) != h.gcName {
			continue
		}
		if _, exist := h.provisions[nsname]; exist {
			continue
		}

		gwsWithoutDeps = append(gwsWithoutDeps, nsname)
	}

	for nsname := range h.provisions {
		if _, exist := h.store.gateways[nsname]; exist {
			continue
		}

		removedGwsWithDeps = append(removedGwsWithDeps, nsname)
	}

	// Create new deployments

	for _, nsname := range gwsWithoutDeps {
		deployment, err := prepareDeployment(h.staticModeDeploymentYAML, generateDeploymentID(nsname), nsname)
		if err != nil {
			panic(fmt.Errorf("failed to prepare deployment: %w", err))
		}

		err = h.k8sClient.Create(ctx, deployment)
		if err != nil {
			panic(fmt.Errorf("failed to create deployment: %w", err))
		}

		h.provisions[nsname] = deployment

		h.logger.Info("Created deployment", "deployment", client.ObjectKeyFromObject(deployment))
	}

	// Remove unnecessary deployments

	for _, nsname := range removedGwsWithDeps {
		deployment := h.provisions[nsname]

		err := h.k8sClient.Delete(ctx, deployment)
		if err != nil {
			panic(fmt.Errorf("failed to delete deployment: %w", err))
		}

		delete(h.provisions, nsname)

		h.logger.Info("Deleted deployment", "deployment", client.ObjectKeyFromObject(deployment))
	}
}

func (h *eventHandler) HandleEventBatch(ctx context.Context, batch events.EventBatch) {
	h.store.update(batch)
	h.ensureGatewayClassAccepted(ctx)
	h.ensureDeploymentsMatchGateways(ctx)
}

func generateDeploymentID(gatewayNsName types.NamespacedName) string {
	// for production, make sure the ID is:
	// - a valid resource name (ex. can't be too long);
	// - unique among all Gateway resources (Gateways test-test/test and test/test-test should not have the same ID)
	return fmt.Sprintf("nginx-gateway-%s-%s", gatewayNsName.Namespace, gatewayNsName.Name)
}
