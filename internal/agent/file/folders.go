package file

import (
	"fmt"
	"os"
	"path/filepath"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 io/fs.DirEntry

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ClearFoldersOSFileManager

// ClearFoldersOSFileManager is an interface that exposes File I/O operations for ClearFolders.
// Used for unit testing.
type ClearFoldersOSFileManager interface {
	// ReadDir returns the directory entries for the directory.
	ReadDir(dirname string) ([]os.DirEntry, error)
	// Remove removes the file with given name.
	Remove(name string) error
}

// ClearFolders removes all files in the given folders and returns the removed files' full paths.
func ClearFolders(fileMgr ClearFoldersOSFileManager, paths []string) (removedFiles []string, e error) {
	for _, path := range paths {
		entries, err := fileMgr.ReadDir(path)
		if err != nil {
			return removedFiles, fmt.Errorf("failed to read directory %q: %w", path, err)
		}

		for _, entry := range entries {
			path := filepath.Join(path, entry.Name())
			if err := fileMgr.Remove(path); err != nil {
				return removedFiles, fmt.Errorf("failed to remove %q: %w", path, err)
			}

			removedFiles = append(removedFiles, path)
		}
	}

	return removedFiles, nil
}
