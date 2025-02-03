package file_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file/filefakes"
)

func writeFile(t *testing.T, name string, data []byte) {
	t.Helper()
	g := NewWithT(t)

	//nolint:gosec // the file permission is ok for unit testing
	g.Expect(os.WriteFile(name, data, 0o644)).To(Succeed())
}

func TestClearFoldersRemoves(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tempDir := t.TempDir()

	path1 := filepath.Join(tempDir, "path1")
	writeFile(t, path1, []byte("test"))
	path2 := filepath.Join(tempDir, "path2")
	writeFile(t, path2, []byte("test"))

	removedFiles, err := file.ClearFolders(file.NewStdLibOSFileManager(), []string{tempDir})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(removedFiles).To(ConsistOf(path1, path2))

	entries, err := os.ReadDir(tempDir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(entries).To(BeEmpty())
}

func TestClearFoldersIgnoresPaths(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeFileMgr := &filefakes.FakeClearFoldersOSFileManager{
		ReadDirStub: func(_ string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				&filefakes.FakeDirEntry{
					NameStub: func() string {
						return "deployment_ctx.json"
					},
				},
				&filefakes.FakeDirEntry{
					NameStub: func() string {
						return "mgmt.conf"
					},
				},
				&filefakes.FakeDirEntry{
					NameStub: func() string {
						return "main.conf"
					},
				},
				&filefakes.FakeDirEntry{
					NameStub: func() string {
						return "can-be-removed.conf"
					},
				},
			}, nil
		},
	}

	removed, err := file.ClearFolders(fakeFileMgr, []string{"/etc/nginx/main-includes"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(removed).To(HaveLen(1))
	g.Expect(removed[0]).To(Equal("/etc/nginx/main-includes/can-be-removed.conf"))
}

func TestClearFoldersFails(t *testing.T) {
	t.Parallel()
	files := []string{"file"}

	testErr := errors.New("test error")

	tests := []struct {
		fileMgr *filefakes.FakeClearFoldersOSFileManager
		name    string
	}{
		{
			fileMgr: &filefakes.FakeClearFoldersOSFileManager{
				ReadDirStub: func(_ string) ([]os.DirEntry, error) {
					return nil, testErr
				},
			},
			name: "ReadDir fails",
		},
		{
			fileMgr: &filefakes.FakeClearFoldersOSFileManager{
				ReadDirStub: func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{
						&filefakes.FakeDirEntry{
							NameStub: func() string {
								return "file"
							},
						},
					}, nil
				},
				RemoveStub: func(_ string) error {
					return testErr
				},
			},
			name: "Remove fails",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			removedFiles, err := file.ClearFolders(test.fileMgr, files)

			g.Expect(err).To(MatchError(testErr))
			g.Expect(removedFiles).To(BeNil())
		})
	}
}
