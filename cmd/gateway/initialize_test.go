package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/configfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file/filefakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestInitialize_OSS(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	fakeFileMgr := &filefakes.OSFileManagerMock{
		OpenFunc: func(_ string) (*os.File, error) {
			return os.CreateTemp(t.TempDir(), "initialize-open-*")
		},
		CreateFunc: func(_ string) (*os.File, error) {
			return os.CreateTemp(t.TempDir(), "initialize-create-*")
		},
		CopyFunc: func(io.Writer, io.Reader) error {
			return nil
		},
		ChmodFunc: func(*os.File, os.FileMode) error {
			return nil
		},
	}

	ic := initializeConfig{
		fileManager: fakeFileMgr,
		logger:      logr.Discard(),
		copy: []fileToCopy{
			{
				destDirName: "destDir",
				srcFileName: "src1",
				permissions: file.RegularFileMode,
			},
			{
				destDirName: "destDir2",
				srcFileName: "src2",
				permissions: file.RegularFileMode,
			},
		},
		plus: false,
	}

	err := initialize(ic)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fakeFileMgr.CreateCalls()).To(HaveLen(2))
	g.Expect(fakeFileMgr.OpenCalls()).To(HaveLen(2))
	g.Expect(fakeFileMgr.CopyCalls()).To(HaveLen(2))
}

func TestInitialize_OSS_Error(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	openErr := errors.New("open error")
	fakeFileMgr := &filefakes.OSFileManagerMock{
		OpenFunc: func(_ string) (*os.File, error) {
			return nil, openErr
		},
	}

	ic := initializeConfig{
		fileManager: fakeFileMgr,
		logger:      logr.Discard(),
		copy: []fileToCopy{
			{
				destDirName: "destDir",
				srcFileName: "src1",
				permissions: file.RegularFileMode,
			},
			{
				destDirName: "destDir2",
				srcFileName: "src2",
				permissions: file.RegularFileMode,
			},
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
				Integration:    "ngf",
				ClusterID:      helpers.GetPointer("cluster-id"),
				InstallationID: helpers.GetPointer("install-id"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeFileMgr := &filefakes.OSFileManagerMock{
				OpenFunc: func(_ string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "initialize-open-*")
				},
				CreateFunc: func(_ string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "initialize-create-*")
				},
				CopyFunc: func(io.Writer, io.Reader) error {
					return nil
				},
				ChmodFunc: func(*os.File, os.FileMode) error {
					return nil
				},
				WriteFunc: func(*os.File, []byte) error {
					return nil
				},
			}
			fakeGenerator := &configfakes.GeneratorMock{}
			fakeGenerator.GenerateDeploymentContextFunc = func(dataplane.DeploymentContext) (agent.File, error) {
				return agent.File{
					Meta: &pb.FileMeta{
						Name:        "/etc/nginx/main-includes/deployment_ctx.json",
						Permissions: file.RegularFileMode,
					},
					Contents: []byte(`{"integration":"ngf"}`),
				}, nil
			}

			ic := initializeConfig{
				fileManager:   fakeFileMgr,
				logger:        logr.Discard(),
				fileGenerator: fakeGenerator,
				copy: []fileToCopy{
					{
						destDirName: "destDir",
						srcFileName: "src1",
						permissions: file.RegularFileMode,
					},
					{
						destDirName: "destDir2",
						srcFileName: "src2",
						permissions: file.RegularFileMode,
					},
				},
				podUID:     "install-id",
				clusterUID: "cluster-id",
				plus:       true,
			}

			g.Expect(initialize(ic)).To(Succeed())
			// copies
			g.Expect(fakeFileMgr.OpenCalls()).To(HaveLen(2))
			g.Expect(fakeFileMgr.CopyCalls()).To(HaveLen(2))

			// 2 copies, 1 write deploy ctx
			g.Expect(fakeFileMgr.CreateCalls()).To(HaveLen(3))
			// write deploy ctx
			g.Expect(fakeGenerator.GenerateDeploymentContextCalls()).To(HaveLen(1))
			g.Expect(fakeGenerator.GenerateDeploymentContextCalls()[0].DepCtx).To(Equal(test.depCtx))
			g.Expect(fakeFileMgr.WriteCalls()).To(HaveLen(1))
			g.Expect(fakeFileMgr.ChmodCalls()).To(HaveLen(3))
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

	g.Expect(copyFile(file.NewStdLibOSFileManager(), src.Name(), dest, file.RegularFileMode)).To(Succeed())
	_, err = os.Stat(filepath.Join(dest, filepath.Base(src.Name())))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestCopyFileErrors(t *testing.T) {
	t.Parallel()

	openErr := errors.New("open error")
	createErr := errors.New("create error")
	copyErr := errors.New("copy error")
	chmodErr := errors.New("chmod error")

	tests := []struct {
		fileMgr *filefakes.OSFileManagerMock
		expErr  error
		name    string
	}{
		{
			name: "can't open src file",
			fileMgr: &filefakes.OSFileManagerMock{
				OpenFunc: func(string) (*os.File, error) {
					return nil, openErr
				},
			},
			expErr: openErr,
		},
		{
			name: "can't create dest file",
			fileMgr: &filefakes.OSFileManagerMock{
				OpenFunc: func(string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "copy-open-*")
				},
				CreateFunc: func(string) (*os.File, error) {
					return nil, createErr
				},
			},
			expErr: createErr,
		},
		{
			name: "can't copy contents",
			fileMgr: &filefakes.OSFileManagerMock{
				OpenFunc: func(string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "copy-open-*")
				},
				CreateFunc: func(string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "copy-create-*")
				},
				CopyFunc: func(io.Writer, io.Reader) error {
					return copyErr
				},
			},
			expErr: copyErr,
		},
		{
			name: "can't set permissions",
			fileMgr: &filefakes.OSFileManagerMock{
				OpenFunc: func(string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "copy-open-*")
				},
				CreateFunc: func(string) (*os.File, error) {
					return os.CreateTemp(t.TempDir(), "copy-create-*")
				},
				CopyFunc: func(io.Writer, io.Reader) error {
					return nil
				},
				ChmodFunc: func(*os.File, os.FileMode) error {
					return chmodErr
				},
			},
			expErr: chmodErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			err := copyFile(test.fileMgr, "source", "destDir", file.RegularFileMode)

			g.Expect(err).To(MatchError(test.expErr))
		})
	}
}

func TestCopyFileInvalidPermissions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	src, err := os.CreateTemp(os.TempDir(), "testfile")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.Remove(src.Name())

	dest, err := os.MkdirTemp(os.TempDir(), "testdir")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(dest)

	err = copyFile(file.NewStdLibOSFileManager(), src.Name(), dest, "not-octal")

	expErr := `invalid file permissions "not-octal": strconv.ParseUint: parsing "not-octal": invalid syntax`
	g.Expect(err).To(MatchError(expErr))
}
