package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	ngxclient "github.com/nginxinc/nginx-plus-go-client/client"
	"k8s.io/apimachinery/pkg/util/wait"
)

//go:generate go tool counterfeiter -generate

const (
	// PidFile specifies the location of the PID file for the Nginx process.
	PidFile = "/var/run/nginx/nginx.pid"
	// PidFileTimeout defines the timeout duration for accessing the PID file.
	PidFileTimeout = 10000 * time.Millisecond
	// NginxReloadTimeout sets the timeout duration for reloading the Nginx configuration.
	NginxReloadTimeout = 60000 * time.Millisecond
)

type (
	ReadFileFunc  func(string) ([]byte, error)
	CheckFileFunc func(string) (fs.FileInfo, error)
)

var childProcPathFmt = "/proc/%[1]v/task/%[1]v/children"

//counterfeiter:generate . NginxPlusClient

type NginxPlusClient interface {
	UpdateHTTPServers(
		upstream string,
		servers []ngxclient.UpstreamServer,
	) (
		added []ngxclient.UpstreamServer,
		deleted []ngxclient.UpstreamServer,
		updated []ngxclient.UpstreamServer,
		err error,
	)
	GetUpstreams() (*ngxclient.Upstreams, error)
	UpdateStreamServers(
		upstream string,
		servers []ngxclient.StreamUpstreamServer,
	) (
		added []ngxclient.StreamUpstreamServer,
		deleted []ngxclient.StreamUpstreamServer,
		updated []ngxclient.StreamUpstreamServer,
		err error,
	)
	GetStreamUpstreams() (*ngxclient.StreamUpstreams, error)
}

//counterfeiter:generate . Manager

// Manager manages the runtime of NGINX.
type Manager interface {
	// Reload reloads NGINX configuration. It is a blocking operation.
	Reload(ctx context.Context, configVersion int) error
	// IsPlus returns whether or not we are running NGINX plus.
	IsPlus() bool
	// GetUpstreams uses the NGINX Plus API to get the upstreams.
	// Only usable if running NGINX Plus.
	GetUpstreams() (ngxclient.Upstreams, ngxclient.StreamUpstreams, error)
	// UpdateHTTPServers uses the NGINX Plus API to update HTTP upstream servers.
	// Only usable if running NGINX Plus.
	UpdateHTTPServers(string, []ngxclient.UpstreamServer) error
	// UpdateStreamServers uses the NGINX Plus API to update stream upstream servers.
	// Only usable if running NGINX Plus.
	UpdateStreamServers(string, []ngxclient.StreamUpstreamServer) error
}

// MetricsCollector is an interface for the metrics of the NGINX runtime manager.
//
//counterfeiter:generate . MetricsCollector
type MetricsCollector interface {
	IncReloadCount()
	IncReloadErrors()
	ObserveLastReloadTime(ms time.Duration)
}

// ManagerImpl implements Manager.
type ManagerImpl struct {
	processHandler   ProcessHandler
	metricsCollector MetricsCollector
	verifyClient     nginxConfigVerifier
	ngxPlusClient    NginxPlusClient
	logger           logr.Logger
}

// NewManagerImpl creates a new ManagerImpl.
func NewManagerImpl(
	ngxPlusClient NginxPlusClient,
	collector MetricsCollector,
	logger logr.Logger,
	processHandler ProcessHandler,
	verifyClient nginxConfigVerifier,
) *ManagerImpl {
	return &ManagerImpl{
		processHandler:   processHandler,
		metricsCollector: collector,
		verifyClient:     verifyClient,
		ngxPlusClient:    ngxPlusClient,
		logger:           logger,
	}
}

// IsPlus returns whether or not we are running NGINX plus.
func (m *ManagerImpl) IsPlus() bool {
	return m.ngxPlusClient != nil
}

func (m *ManagerImpl) Reload(ctx context.Context, configVersion int) error {
	start := time.Now()
	// We find the main NGINX PID on every reload because it will change if the NGINX container is restarted.
	pid, err := m.processHandler.FindMainProcess(ctx, PidFileTimeout)
	if err != nil {
		return fmt.Errorf("failed to find NGINX main process: %w", err)
	}

	childProcFile := fmt.Sprintf(childProcPathFmt, pid)
	previousChildProcesses, err := m.processHandler.ReadFile(childProcFile)
	if err != nil {
		return err
	}

	// send HUP signal to the NGINX main process reload configuration
	// See https://nginx.org/en/docs/control.html
	if errP := m.processHandler.Kill(pid); errP != nil {
		m.metricsCollector.IncReloadErrors()
		return fmt.Errorf("failed to send the HUP signal to NGINX main: %w", errP)
	}

	if err = m.verifyClient.WaitForCorrectVersion(
		ctx,
		configVersion,
		childProcFile,
		previousChildProcesses,
		os.ReadFile,
	); err != nil {
		m.metricsCollector.IncReloadErrors()
		return err
	}
	m.metricsCollector.IncReloadCount()

	finish := time.Now()
	m.metricsCollector.ObserveLastReloadTime(finish.Sub(start))
	return nil
}

// GetUpstreams uses the NGINX Plus API to get the upstreams.
// Only usable if running NGINX Plus.
func (m *ManagerImpl) GetUpstreams() (ngxclient.Upstreams, ngxclient.StreamUpstreams, error) {
	if !m.IsPlus() {
		panic("cannot get upstream servers: NGINX Plus not enabled")
	}

	upstreams, err := m.ngxPlusClient.GetUpstreams()
	if err != nil {
		return nil, nil, err
	}

	if upstreams == nil {
		return nil, nil, errors.New("GET upstreams returned nil value")
	}

	streamUpstreams, err := m.ngxPlusClient.GetStreamUpstreams()
	if err != nil {
		return nil, nil, err
	}

	if streamUpstreams == nil {
		return nil, nil, errors.New("GET stream upstreams returned nil value")
	}

	return *upstreams, *streamUpstreams, nil
}

// UpdateHTTPServers uses the NGINX Plus API to update HTTP upstream servers.
// Only usable if running NGINX Plus.
func (m *ManagerImpl) UpdateHTTPServers(upstream string, servers []ngxclient.UpstreamServer) error {
	if !m.IsPlus() {
		panic("cannot update HTTP upstream servers: NGINX Plus not enabled")
	}

	added, deleted, updated, err := m.ngxPlusClient.UpdateHTTPServers(upstream, servers)
	m.logger.V(1).Info("Added upstream servers", "count", len(added))
	m.logger.V(1).Info("Deleted upstream servers", "count", len(deleted))
	m.logger.V(1).Info("Updated upstream servers", "count", len(updated))

	return err
}

// UpdateStreamServers uses the NGINX Plus API to update stream upstream servers.
// Only usable if running NGINX Plus.
func (m *ManagerImpl) UpdateStreamServers(upstream string, servers []ngxclient.StreamUpstreamServer) error {
	if !m.IsPlus() {
		panic("cannot update stream upstream servers: NGINX Plus not enabled")
	}

	added, deleted, updated, err := m.ngxPlusClient.UpdateStreamServers(upstream, servers)
	m.logger.V(1).Info("Added stream upstream servers", "count", len(added))
	m.logger.V(1).Info("Deleted stream upstream servers", "count", len(deleted))
	m.logger.V(1).Info("Updated stream upstream servers", "count", len(updated))

	return err
}

//counterfeiter:generate . ProcessHandler

type ProcessHandler interface {
	FindMainProcess(
		ctx context.Context,
		timeout time.Duration,
	) (int, error)
	ReadFile(file string) ([]byte, error)
	Kill(pid int) error
}

type ProcessHandlerImpl struct {
	readFile  ReadFileFunc
	checkFile CheckFileFunc
}

func NewProcessHandlerImpl(readFile ReadFileFunc, checkFile CheckFileFunc) *ProcessHandlerImpl {
	return &ProcessHandlerImpl{
		readFile:  readFile,
		checkFile: checkFile,
	}
}

func (p *ProcessHandlerImpl) FindMainProcess(
	ctx context.Context,
	timeout time.Duration,
) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(_ context.Context) (bool, error) {
			_, err := p.checkFile(PidFile)
			if err == nil {
				return true, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return false, err
			}
			return false, nil
		})
	if err != nil {
		return 0, err
	}

	content, err := p.readFile(PidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid file content %q: %w", content, err)
	}

	return pid, nil
}

func (p *ProcessHandlerImpl) ReadFile(file string) ([]byte, error) {
	return p.readFile(file)
}

func (p *ProcessHandlerImpl) Kill(pid int) error {
	return syscall.Kill(pid, syscall.SIGHUP)
}
