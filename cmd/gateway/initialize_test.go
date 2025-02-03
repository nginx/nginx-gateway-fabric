package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/licensing/licensingfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/configfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file/filefakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

func TestInitialize_OSS(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	fakeFileMgr := &filefakes.FakeOSFileManager{}

	ic := initializeConfig{
		fileManager: fakeFileMgr,
		logger:      zap.New(),
		copy: copyFiles{
			destDirName:  "destDir",
			srcFileNames: []string{"src1", "src2"},
		},
		plus: false,
	}

	err := initialize(ic)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fakeFileMgr.CreateCallCount()).To(Equal(2))
	g.Expect(fakeFileMgr.OpenCallCount()).To(Equal(2))
	g.Expect(fakeFileMgr.CopyCallCount()).To(Equal(2))
}

func TestInitialize_OSS_Error(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	openErr := errors.New("open error")
	fakeFileMgr := &filefakes.FakeOSFileManager{
		OpenStub: func(_ string) (*os.File, error) {
			return nil, openErr
		},
	}

	ic := initializeConfig{
		fileManager: fakeFileMgr,
		logger:      zap.New(),
		copy: copyFiles{
			destDirName:  "destDir",
			srcFileNames: []string{"src1", "src2"},
		},
		plus: false,
	}

	err := initialize(ic)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(openErr))
}

func TestInitialize_Plus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		collectErr error
		depCtx     dataplane.DeploymentContext
	}{
		{
			name:       "normal",
			collectErr: nil,
			depCtx: dataplane.DeploymentContext{
				Integration:      "ngf",
				ClusterID:        helpers.GetPointer("cluster-id"),
				InstallationID:   helpers.GetPointer("install-id"),
				ClusterNodeCount: helpers.GetPointer(2),
			},
		},
		{
			name:       "collecting deployment context errors",
			collectErr: errors.New("collect error"),
			depCtx: dataplane.DeploymentContext{
				Integration:    "ngf",
				InstallationID: helpers.GetPointer("install-id"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeFileMgr := &filefakes.FakeOSFileManager{}
			fakeCollector := &licensingfakes.FakeCollector{
				CollectStub: func(_ context.Context) (dataplane.DeploymentContext, error) {
					return test.depCtx, test.collectErr
				},
			}
			fakeGenerator := &configfakes.FakeGenerator{}

			ic := initializeConfig{
				fileManager:   fakeFileMgr,
				logger:        zap.New(),
				collector:     fakeCollector,
				fileGenerator: fakeGenerator,
				copy: copyFiles{
					destDirName:  "destDir",
					srcFileNames: []string{"src1", "src2"},
				},
				plus: true,
			}

			g.Expect(initialize(ic)).To(Succeed())
			// copies
			g.Expect(fakeFileMgr.OpenCallCount()).To(Equal(2))
			g.Expect(fakeFileMgr.CopyCallCount()).To(Equal(2))

			// 2 copies, 1 write deploy ctx
			g.Expect(fakeFileMgr.CreateCallCount()).To(Equal(3))
			// write deploy ctx
			g.Expect(fakeGenerator.GenerateDeploymentContextCallCount()).To(Equal(1))
			g.Expect(fakeGenerator.GenerateDeploymentContextArgsForCall(0)).To(Equal(test.depCtx))
			g.Expect(fakeCollector.CollectCallCount()).To(Equal(1))
			g.Expect(fakeFileMgr.WriteCallCount()).To(Equal(1))
			g.Expect(fakeFileMgr.ChmodCallCount()).To(Equal(1))
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	src, err := os.CreateTemp(os.TempDir(), "testfile")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.Remove(src.Name())

	dest, err := os.MkdirTemp(os.TempDir(), "testdir")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(dest)

	g.Expect(copyFile(file.NewStdLibOSFileManager(), src.Name(), dest)).To(Succeed())
	_, err = os.Stat(filepath.Join(dest, filepath.Base(src.Name())))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestCopyFileErrors(t *testing.T) {
	t.Parallel()

	openErr := errors.New("open error")
	createErr := errors.New("create error")
	copyErr := errors.New("copy error")

	tests := []struct {
		fileMgr *filefakes.FakeOSFileManager
		expErr  error
		name    string
	}{
		{
			name: "can't open src file",
			fileMgr: &filefakes.FakeOSFileManager{
				OpenStub: func(_ string) (*os.File, error) {
					return nil, openErr
				},
			},
			expErr: openErr,
		},
		{
			name: "can't create dest file",
			fileMgr: &filefakes.FakeOSFileManager{
				CreateStub: func(_ string) (*os.File, error) {
					return nil, createErr
				},
			},
			expErr: createErr,
		},
		{
			name: "can't copy contents",
			fileMgr: &filefakes.FakeOSFileManager{
				CopyStub: func(_ io.Writer, _ io.Reader) error {
					return copyErr
				},
			},
			expErr: copyErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			err := copyFile(test.fileMgr, "source", "destDir")

			g.Expect(err).To(MatchError(test.expErr))
		})
	}
}
