// Package store reads and writes the .cairn folder.
package store

import (
	"io/fs"
	"os"
	"path/filepath"
)

// tempPattern names WriteFileAtomic's temporary files, for os.CreateTemp.
const tempPattern = ".cairn-tmp-*"

// WriteFileAtomic replaces path with data. It writes a temporary file in the same folder (its name starts with
// ".cairn-tmp-"), syncs it, sets perm on it, and renames it over path. On any error the temporary file is removed
// and path is left as it was. The folder must already exist.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), tempPattern)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(f.Name()) // the write has already failed; a failed cleanup changes nothing for the caller
		}
	}()
	if err = fill(f, data, perm); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// fill writes data to f, syncs it, and sets perm on it.
func fill(f *os.File, data []byte, perm fs.FileMode) error {
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Chmod(perm)
}
