package file_test

import (
	"errors"
	"os"
	"path/filepath"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file/filefakes"
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

		It("returns a close error from the deferred close path", func() {
			tmpFile, err := os.CreateTemp(GinkgoT().TempDir(), "close-error-*.conf")
			Expect(err).ToNot(HaveOccurred())

			mgr := &filefakes.FakeOSFileManager{
				CreateStub: func(_ string) (*os.File, error) {
					return tmpFile, nil
				},
				ChmodStub: func(_ *os.File, _ os.FileMode) error {
					return nil
				},
				WriteStub: func(file *os.File, _ []byte) error {
					return file.Close()
				},
			}

			err = file.Write(mgr, files[0])
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to close file"))
		})

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

		convertedFile, err := file.Convert(agentFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(convertedFile).To(Equal(expFile))

		convertedSecretFile, err := file.Convert(secretAgentFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(convertedSecretFile).To(Equal(expSecretFile))
	})

	It("rejects agent files with unsupported permissions", func() {
		_, err := file.Convert(agent.File{
			Contents: []byte("file contents"),
			Meta: &pb.FileMeta{
				Name:        "invalid-file",
				Permissions: "0600",
			},
		})

		Expect(err).To(MatchError("unknown file permissions \"0600\""))
	})

	It("rejects agent files without metadata", func() {
		_, err := file.Convert(agent.File{})

		Expect(err).To(MatchError("agent file metadata is required"))
	})
})
