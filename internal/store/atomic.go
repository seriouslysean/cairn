// Package store reads and writes the .cairn folder.
package store

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data. It writes a temporary file in the same folder (its name starts with
// ".cairn-tmp-"), syncs it, sets perm on it, and renames it over path. On any error the temporary file is removed
// and path is left as it was. The folder must already exist.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".cairn-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
