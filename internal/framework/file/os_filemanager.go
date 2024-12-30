package file

import (
	"io"
	"io/fs"
	"os"
)

// StdLibOSFileManager wraps the standard library's file operations.
// Clients can define an interface with all or a subset StdLibOSFileManager methods and use it in their types or
// functions, so that they can be unit tested.
// It is expected that clients generate fakes.
type StdLibOSFileManager struct{}

func NewStdLibOSFileManager() *StdLibOSFileManager {
	return &StdLibOSFileManager{}
}

// ReadDir wraps os.ReadDir.
func (s *StdLibOSFileManager) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirname)
}

// Remove wraps os.Remove.
func (s *StdLibOSFileManager) Remove(name string) error {
	return os.Remove(name)
}

// Write wraps os.File.Write.
func (s *StdLibOSFileManager) Write(file *os.File, contents []byte) error {
	_, err := file.Write(contents)

	return err
}

// Create wraps os.Create.
func (s *StdLibOSFileManager) Create(name string) (*os.File, error) {
	return os.Create(name)
}

// Chmod wraps os.File.Chmod.
func (s *StdLibOSFileManager) Chmod(file *os.File, mode os.FileMode) error {
	return file.Chmod(mode)
}

// Open wraps os.Open.
func (s *StdLibOSFileManager) Open(name string) (*os.File, error) { return os.Open(name) }

// Copy wraps io.Copy.
func (s *StdLibOSFileManager) Copy(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	return err
}
