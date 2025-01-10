package agent

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast"
	agentgrpc "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/hack"
)

// Deployment represents an nginx Deployment. It contains its own nginx configuration files,
// and a broadcaster for sending those files to all of its pods that are subscribed.
type Deployment struct {
	broadcaster broadcast.Broadcaster

	configVersion string
	fileOverviews []*pb.File
	files         []File

	lock sync.RWMutex
}

// newDeployment returns a new deployment object.
func newDeployment(ctx context.Context) *Deployment {
	return &Deployment{
		broadcaster: broadcast.NewDeploymentBroadcaster(ctx),
	}
}

// GetBroadcaster returns the deployment's broadcaster.
func (d *Deployment) GetBroadcaster() broadcast.Broadcaster {
	return d.broadcaster
}

// GetFileOverviews returns the current list of fileOverviews and configVersion for the deployment.
func (d *Deployment) GetFileOverviews() ([]*pb.File, string) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.fileOverviews, d.configVersion
}

// GetFile gets the requested file for the deployment and returns its contents.
func (d *Deployment) GetFile(name, hash string) []byte {
	d.lock.RLock()
	defer d.lock.RUnlock()

	for _, file := range d.files {
		if name == file.Meta.GetName() && hash == file.Meta.GetHash() {
			return file.Contents
		}
	}

	return nil
}

// SetFiles updates the nginx files and fileOverviews for the deployment and returns the message to send.
func (d *Deployment) SetFiles(files []File) broadcast.NginxAgentMessage {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.files = files

	fileOverviews := make([]*pb.File, 0, len(files))
	for _, file := range files {
		fileOverviews = append(fileOverviews, &pb.File{FileMeta: file.Meta})
	}

	// hack to include unchanging static files in the payload so they don't get deleted
	staticFiles := hack.GetStaticFiles()
	for _, file := range staticFiles {
		meta := &pb.FileMeta{
			Name:        file.Name,
			Hash:        filesHelper.GenerateHash(file.Contents),
			Permissions: file.Permissions,
		}

		fileOverviews = append(fileOverviews, &pb.File{
			FileMeta: meta,
		})

		d.files = append(d.files, File{Meta: meta, Contents: file.Contents})
	}

	d.configVersion = filesHelper.GenerateConfigVersion(fileOverviews)
	d.fileOverviews = fileOverviews

	return broadcast.NginxAgentMessage{
		Type:          broadcast.ConfigApplyRequest,
		FileOverviews: fileOverviews,
		ConfigVersion: d.configVersion,
	}
}

// DeploymentStore holds a map of all Deployments.
type DeploymentStore struct {
	connTracker *agentgrpc.ConnectionsTracker
	deployments sync.Map
}

// NewDeploymentStore returns a new instance of a DeploymentStore.
func NewDeploymentStore(connTracker *agentgrpc.ConnectionsTracker) *DeploymentStore {
	return &DeploymentStore{
		connTracker: connTracker,
	}
}

// Get returns the desired deployment from the store.
func (d *DeploymentStore) Get(nsName types.NamespacedName) (*Deployment, bool) {
	val, ok := d.deployments.Load(nsName)
	if !ok {
		return nil, false
	}

	deployment, ok := val.(*Deployment)
	if !ok {
		panic(fmt.Sprintf("expected Deployment, got type %T", val))
	}

	return deployment, true
}

// GetOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
func (d *DeploymentStore) GetOrStore(ctx context.Context, nsName types.NamespacedName) *Deployment {
	if deployment, ok := d.Get(nsName); ok {
		return deployment
	}

	deployment := newDeployment(ctx)
	d.deployments.Store(nsName, deployment)

	return deployment
}

// Remove cleans up any connections that are tracked for this deployment, and then removes
// the deployment from the store.
func (d *DeploymentStore) Remove(nsName types.NamespacedName) {
	d.connTracker.UntrackConnectionsForParent(nsName)
	d.deployments.Delete(nsName)
}
