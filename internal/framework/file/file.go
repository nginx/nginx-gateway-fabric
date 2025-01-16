package file

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	// RegularFileModeInt defines the default file mode for regular files as an integer.
	RegularFileModeInt = 0o644
	// RegularFileMode defines the default file mode for regular files.
	RegularFileMode = "0644"
	// secretFileMode defines the default file mode for files with secrets as an integer.
	secretFileModeInt = 0o640
	// SecretFileMode defines the default file mode for files with secrets.
	SecretFileMode = "0640"
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
		if err := fileMgr.Chmod(f, RegularFileModeInt); err != nil {
			resultErr = fmt.Errorf(
				"failed to set file mode to %#o for %q: %w", RegularFileModeInt, file.Path, err)
			return resultErr
		}
	case TypeSecret:
		if err := fileMgr.Chmod(f, secretFileModeInt); err != nil {
			resultErr = fmt.Errorf("failed to set file mode to %#o for %q: %w", secretFileModeInt, file.Path, err)
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

// Convert an agent File to an internal File type.
func Convert(agentFile agent.File) File {
	if agentFile.Meta == nil {
		return File{}
	}

	var t Type
	switch agentFile.Meta.Permissions {
	case RegularFileMode:
		t = TypeRegular
	case SecretFileMode:
		t = TypeSecret
	}

	return File{
		Content: agentFile.Contents,
		Path:    agentFile.Meta.Name,
		Type:    t,
	}
}
