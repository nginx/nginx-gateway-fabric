package file_test

import (
	"errors"
	"os"
	"path/filepath"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/file"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/file/filefakes"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent"
)

var _ = Describe("Write files", Ordered, func() {
	var (
		mgr                        file.OSFileManager
		tmpDir                     string
		regular1, regular2, secret file.File
	)

	ensureFiles := func(files []file.File) {
		entries, err := os.ReadDir(tmpDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(entries).Should(HaveLen(len(files)))

		entriesMap := make(map[string]os.DirEntry)
		for _, entry := range entries {
			entriesMap[entry.Name()] = entry
		}

		for _, f := range files {
			_, ok := entriesMap[filepath.Base(f.Path)]
			Expect(ok).Should(BeTrue())

			info, err := os.Stat(f.Path)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.IsDir()).To(BeFalse())

			if f.Type == file.TypeRegular {
				Expect(info.Mode()).To(Equal(os.FileMode(0o644)))
			} else {
				Expect(info.Mode()).To(Equal(os.FileMode(0o640)))
			}

			bytes, err := os.ReadFile(f.Path)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytes).To(Equal(f.Content))
		}
	}

	BeforeAll(func() {
		mgr = file.NewStdLibOSFileManager()
		tmpDir = GinkgoT().TempDir()

		regular1 = file.File{
			Type:    file.TypeRegular,
			Path:    filepath.Join(tmpDir, "regular-1.conf"),
			Content: []byte("regular-1"),
		}
		regular2 = file.File{
			Type:    file.TypeRegular,
			Path:    filepath.Join(tmpDir, "regular-2.conf"),
			Content: []byte("regular-2"),
		}
		secret = file.File{
			Type:    file.TypeSecret,
			Path:    filepath.Join(tmpDir, "secret.conf"),
			Content: []byte("secret"),
		}
	})

	It("should write files", func() {
		files := []file.File{regular1, regular2, secret}

		for _, f := range files {
			Expect(file.Write(mgr, f)).To(Succeed())
		}

		ensureFiles(files)
	})

	When("file type is not supported", func() {
		It("should panic", func() {
			mgr = file.NewStdLibOSFileManager()

			f := file.File{
				Type: 123,
				Path: "unsupported.conf",
			}

			replace := func() {
				_ = file.Write(mgr, f)
			}

			Expect(replace).Should(Panic())
		})
	})

	Describe("Edge cases with IO errors", func() {
		var (
			files = []file.File{
				{
					Type:    file.TypeRegular,
					Path:    "regular.conf",
					Content: []byte("regular"),
				},
				{
					Type:    file.TypeSecret,
					Path:    "secret.conf",
					Content: []byte("secret"),
				},
			}
			errTest = errors.New("test error")
		)

		DescribeTable(
			"should return error on file IO error",
			func(fakeOSMgr *filefakes.FakeOSFileManager) {
				mgr := fakeOSMgr

				for _, f := range files {
					err := file.Write(mgr, f)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(errTest))
				}
			},
			Entry(
				"Create",
				&filefakes.FakeOSFileManager{
					CreateStub: func(_ string) (*os.File, error) {
						return nil, errTest
					},
				},
			),
			Entry(
				"Chmod",
				&filefakes.FakeOSFileManager{
					ChmodStub: func(_ *os.File, _ os.FileMode) error {
						return errTest
					},
				},
			),
			Entry(
				"Write",
				&filefakes.FakeOSFileManager{
					WriteStub: func(_ *os.File, _ []byte) error {
						return errTest
					},
				},
			),
		)
	})

	It("converts agent files to internal files", func() {
		agentFile := agent.File{
			Contents: []byte("file contents"),
			Meta: &pb.FileMeta{
				Name:        "regular-file",
				Permissions: file.RegularFileMode,
			},
		}
		expFile := file.File{
			Path:    "regular-file",
			Content: []byte("file contents"),
			Type:    file.TypeRegular,
		}

		secretAgentFile := agent.File{
			Contents: []byte("secret contents"),
			Meta: &pb.FileMeta{
				Name:        "secret-file",
				Permissions: file.SecretFileMode,
			},
		}
		expSecretFile := file.File{
			Path:    "secret-file",
			Content: []byte("secret contents"),
			Type:    file.TypeSecret,
		}

		Expect(file.Convert(agentFile)).To(Equal(expFile))
		Expect(file.Convert(secretAgentFile)).To(Equal(expSecretFile))
	})
})
