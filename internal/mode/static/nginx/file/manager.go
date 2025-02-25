package file

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/go-logr/logr"
)

//go:generate go tool counterfeiter -generate

const (
	// regularFileMode defines the default file mode for regular files.
	regularFileMode = 0o644
	// secretFileMode defines the default file mode for files with secrets.
	secretFileMode = 0o640
)

// Type is the type of File.
type Type int

func (t Type) String() string {
	switch t {
	case TypeRegular:
		return "Regular"
	case TypeSecret:
		return "Secret"
	default:
		return fmt.Sprintf("Unknown Type %d", t)
	}
}

const (
	// TypeRegular is the type for regular configuration files.
	TypeRegular Type = iota
	// TypeSecret is the type for secret files.
	TypeSecret
)

// File is a file that is part of NGINX configuration to be written to the file system.
type File struct {
	Path    string
	Content []byte
	Type    Type
}

//counterfeiter:generate . OSFileManager

// OSFileManager is an interface that exposes File I/O operations for ManagerImpl.
// Used for unit testing.
type OSFileManager interface {
	// ReadDir returns the directory entries for the directory.
	ReadDir(dirname string) ([]fs.DirEntry, error)
	// Remove file with given name.
	Remove(name string) error
	// Create file at the provided filepath.
	Create(name string) (*os.File, error)
	// Chmod sets the mode of the file.
	Chmod(file *os.File, mode os.FileMode) error
	// Write writes contents to the file.
	Write(file *os.File, contents []byte) error
	// Open opens the file.
	Open(name string) (*os.File, error)
	// Copy copies from src to dst.
	Copy(dst io.Writer, src io.Reader) error
}

//counterfeiter:generate . Manager

// Manager manages NGINX configuration files.
type Manager interface {
	// ReplaceFiles replaces the files on the file system with the given files removing any previous files.
	ReplaceFiles(files []File) error
}

// ManagerImpl is an implementation of Manager.
// Note: It is not thread safe.
type ManagerImpl struct {
	logger           logr.Logger
	osFileManager    OSFileManager
	lastWrittenPaths []string
}

// NewManagerImpl creates a new NewManagerImpl.
func NewManagerImpl(logger logr.Logger, osFileManager OSFileManager) *ManagerImpl {
	return &ManagerImpl{
		logger:        logger,
		osFileManager: osFileManager,
	}
}

// ReplaceFiles replaces the files on the file system with the given files removing any previous files.
// It panics if a file type is unknown.
func (m *ManagerImpl) ReplaceFiles(files []File) error {
	for _, path := range m.lastWrittenPaths {
		if err := m.osFileManager.Remove(path); err != nil {
			if os.IsNotExist(err) {
				m.logger.Info(
					"File not found when attempting to delete",
					"path", path,
					"error", err,
				)
				continue
			}
			return fmt.Errorf("failed to delete file %q: %w", path, err)
		}

		m.logger.V(1).Info("Deleted file", "path", path)
	}

	// In some cases, NGINX reads files in runtime, like a JWK. If you remove such file, NGINX will fail
	// any request (return 500 status code) that involves reading the file.
	// However, we don't have such files yet, so we're not considering this case.

	m.lastWrittenPaths = make([]string, 0, len(files))

	for _, file := range files {
		if err := WriteFile(m.osFileManager, file); err != nil {
			return fmt.Errorf("failed to write file %q of type %v: %w", file.Path, file.Type, err)
		}

		m.lastWrittenPaths = append(m.lastWrittenPaths, file.Path)
		m.logger.V(1).Info("Wrote file", "path", file.Path)
	}

	return nil
}

func WriteFile(fileMgr OSFileManager, file File) error {
	ensureType(file.Type)

	f, err := fileMgr.Create(file.Path)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", file.Path, err)
	}

	var resultErr error

	defer func() {
		if err := f.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("failed to close file %q: %w", file.Path, err))
		}
	}()

	switch file.Type {
	case TypeRegular:
		if err := fileMgr.Chmod(f, regularFileMode); err != nil {
			resultErr = fmt.Errorf(
				"failed to set file mode to %#o for %q: %w", regularFileMode, file.Path, err)
			return resultErr
		}
	case TypeSecret:
		if err := fileMgr.Chmod(f, secretFileMode); err != nil {
			resultErr = fmt.Errorf("failed to set file mode to %#o for %q: %w", secretFileMode, file.Path, err)
			return resultErr
		}
	default:
		panic(fmt.Sprintf("unknown file type %d", file.Type))
	}

	if err := fileMgr.Write(f, file.Content); err != nil {
		resultErr = fmt.Errorf("failed to write file %q: %w", file.Path, err)
		return resultErr
	}

	return resultErr
}

func ensureType(fileType Type) {
	if fileType != TypeRegular && fileType != TypeSecret {
		panic(fmt.Sprintf("unknown file type %d", fileType))
	}
}
