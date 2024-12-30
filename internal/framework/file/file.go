package file

import (
	"errors"
	"fmt"
	"io"
	"os"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

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

// OSFileManager is an interface that exposes File I/O operations.
type OSFileManager interface {
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

func Write(fileMgr OSFileManager, file File) error {
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
