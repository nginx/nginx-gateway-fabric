package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/broadcast"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc"
)

// ignoreFiles is a list of static or base files that live in the
// nginx container that should not be touched by the agent. Any files
// that we add directly into the container should be added here.
var ignoreFiles = []string{
	"/etc/nginx/nginx.conf",
	"/etc/nginx/mime.types",
	"/etc/nginx/grpc-error-locations.conf",
	"/etc/nginx/grpc-error-pages.conf",
	"/usr/share/nginx/html/50x.html",
	"/usr/share/nginx/html/dashboard.html",
	"/usr/share/nginx/html/index.html",
	"/usr/share/nginx/html/nginx-modules-reference.pdf",
}

const fileMode = "0644"

// Deployment represents an nginx Deployment. It contains its own nginx configuration files,
// a broadcaster for sending those files to all of its pods that are subscribed, and errors
// that may have occurred while applying configuration.
type Deployment struct {
	// podStatuses is a map of all Pods for this Deployment and the most recent error
	// (or nil if successful) that occurred on a config call to the nginx agent.
	podStatuses map[string]error

	broadcaster broadcast.Broadcaster

	configVersion string
	// error that is set if a ConfigApply call failed for a Pod. This is needed
	// because if subsequent upstream API calls are made within the same update event,
	// and are successful, the previous error would be lost in the podStatuses map.
	// It's used to preserve the error for when we write status after fully updating nginx.
	latestConfigError error
	// error that is set when at least one upstream API call failed for a Pod.
	// This is needed because subsequent API calls within the same update event could succeed,
	// and therefore the previous error would be lost in the podStatuses map. It's used to preserve
	// the error for when we write status after fully updating nginx.
	latestUpstreamError error

	nginxPlusActions []*pb.NGINXPlusAction
	fileOverviews    []*pb.File
	files            []File

	FileLock sync.RWMutex
	errLock  sync.RWMutex
}

// newDeployment returns a new Deployment object.
func newDeployment(broadcaster broadcast.Broadcaster) *Deployment {
	return &Deployment{
		broadcaster: broadcaster,
		podStatuses: make(map[string]error),
	}
}

// GetBroadcaster returns the deployment's broadcaster.
func (d *Deployment) GetBroadcaster() broadcast.Broadcaster {
	return d.broadcaster
}

// SetLatestConfigError sets the latest config apply error for the deployment.
func (d *Deployment) SetLatestConfigError(err error) {
	d.errLock.Lock()
	defer d.errLock.Unlock()

	d.latestConfigError = err
}

// SetLatestUpstreamError sets the latest upstream update error for the deployment.
func (d *Deployment) SetLatestUpstreamError(err error) {
	d.errLock.Lock()
	defer d.errLock.Unlock()

	d.latestUpstreamError = err
}

// GetLatestConfigError gets the latest config apply error for the deployment.
func (d *Deployment) GetLatestConfigError() error {
	d.errLock.RLock()
	defer d.errLock.RUnlock()

	return d.latestConfigError
}

// GetLatestUpstreamError gets the latest upstream update error for the deployment.
func (d *Deployment) GetLatestUpstreamError() error {
	d.errLock.RLock()
	defer d.errLock.RUnlock()

	return d.latestUpstreamError
}

// SetPodErrorStatus sets the error status of a Pod in this Deployment if applying the config failed.
func (d *Deployment) SetPodErrorStatus(pod string, err error) {
	d.errLock.Lock()
	defer d.errLock.Unlock()

	d.podStatuses[pod] = err
}

// RemovePodStatus deletes a pod from the pod status map.
func (d *Deployment) RemovePodStatus(podName string) {
	d.errLock.Lock()
	defer d.errLock.Unlock()

	delete(d.podStatuses, podName)
}

// GetConfigurationStatus returns the current config status for this Deployment. It combines
// the most recent errors (if they exist) for all Pods in the Deployment into a single error.
func (d *Deployment) GetConfigurationStatus() error {
	d.errLock.RLock()
	defer d.errLock.RUnlock()

	errs := make([]error, 0, len(d.podStatuses))
	for _, err := range d.podStatuses {
		errs = append(errs, err)
	}

	if len(errs) == 1 {
		return errs[0]
	}

	return errors.Join(errs...)
}

/*
The following functions for the Deployment object are UNLOCKED, meaning that they are unsafe.
Callers of these functions MUST ensure the FileLock is set before calling.

These functions are called as part of the ConfigApply or APIRequest processes. These entire processes
are locked by the caller, hence why the functions themselves do not set the locks.
*/

// GetFileOverviews returns the current list of fileOverviews and configVersion for the deployment.
// The deployment FileLock MUST already be locked before calling this function.
func (d *Deployment) GetFileOverviews() ([]*pb.File, string) {
	return d.fileOverviews, d.configVersion
}

// GetNGINXPlusActions returns the current NGINX Plus API Actions for the deployment.
// The deployment FileLock MUST already be locked before calling this function.
func (d *Deployment) GetNGINXPlusActions() []*pb.NGINXPlusAction {
	return d.nginxPlusActions
}

// GetFile gets the requested file for the deployment and returns its contents.
// The deployment FileLock MUST already be locked before calling this function.
func (d *Deployment) GetFile(name, hash string) []byte {
	for _, file := range d.files {
		if name == file.Meta.GetName() && hash == file.Meta.GetHash() {
			return file.Contents
		}
	}

	return nil
}

// SetFiles updates the nginx files and fileOverviews for the deployment and returns the message to send.
// The deployment FileLock MUST already be locked before calling this function.
func (d *Deployment) SetFiles(files []File) *broadcast.NginxAgentMessage {
	d.files = files

	fileOverviews := make([]*pb.File, 0, len(files))
	for _, file := range files {
		fileOverviews = append(fileOverviews, &pb.File{FileMeta: file.Meta})
	}

	// add ignored files to the overview as 'unmanaged' so agent doesn't touch them
	for _, f := range ignoreFiles {
		meta := &pb.FileMeta{
			Name:        f,
			Permissions: fileMode,
		}

		fileOverviews = append(fileOverviews, &pb.File{
			FileMeta:  meta,
			Unmanaged: true,
		})
	}

	newConfigVersion := filesHelper.GenerateConfigVersion(fileOverviews)
	if d.configVersion == newConfigVersion {
		// files have not changed, nothing to send
		return nil
	}

	d.configVersion = newConfigVersion
	d.fileOverviews = fileOverviews

	return &broadcast.NginxAgentMessage{
		Type:          broadcast.ConfigApplyRequest,
		FileOverviews: fileOverviews,
		ConfigVersion: d.configVersion,
	}
}

// SetNGINXPlusActions updates the deployment's latest NGINX Plus Actions to perform if using NGINX Plus.
// Used by a Subscriber when it first connects.
// The deployment FileLock MUST already be locked before calling this function.
func (d *Deployment) SetNGINXPlusActions(actions []*pb.NGINXPlusAction) {
	d.nginxPlusActions = actions
}

//counterfeiter:generate . DeploymentStorer

// DeploymentStorer is an interface to store Deployments.
type DeploymentStorer interface {
	Get(types.NamespacedName) *Deployment
	GetOrStore(context.Context, types.NamespacedName, chan struct{}) *Deployment
	Remove(types.NamespacedName)
}

// DeploymentStore holds a map of all Deployments.
type DeploymentStore struct {
	connTracker agentgrpc.ConnectionsTracker
	deployments sync.Map
}

// NewDeploymentStore returns a new instance of a DeploymentStore.
func NewDeploymentStore(connTracker agentgrpc.ConnectionsTracker) *DeploymentStore {
	return &DeploymentStore{
		connTracker: connTracker,
	}
}

// Get returns the desired deployment from the store.
func (d *DeploymentStore) Get(nsName types.NamespacedName) *Deployment {
	val, ok := d.deployments.Load(nsName)
	if !ok {
		return nil
	}

	deployment, ok := val.(*Deployment)
	if !ok {
		panic(fmt.Sprintf("expected Deployment, got type %T", val))
	}

	return deployment
}

// GetOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
func (d *DeploymentStore) GetOrStore(
	ctx context.Context,
	nsName types.NamespacedName,
	stopCh chan struct{},
) *Deployment {
	if deployment := d.Get(nsName); deployment != nil {
		return deployment
	}

	deployment := newDeployment(broadcast.NewDeploymentBroadcaster(ctx, stopCh))
	d.deployments.Store(nsName, deployment)

	return deployment
}

// StoreWithBroadcaster creates a new Deployment with the supplied broadcaster and stores it.
// Used in unit tests to provide a mock broadcaster.
func (d *DeploymentStore) StoreWithBroadcaster(
	nsName types.NamespacedName,
	broadcaster broadcast.Broadcaster,
) *Deployment {
	deployment := newDeployment(broadcaster)
	d.deployments.Store(nsName, deployment)

	return deployment
}

// Remove the deployment from the store.
func (d *DeploymentStore) Remove(nsName types.NamespacedName) {
	d.deployments.Delete(nsName)
}
