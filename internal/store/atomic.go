// Package store reads and writes the .cairn folder.
package store

import "io/fs"

// WriteFileAtomic replaces path with data. It writes a temporary file in the same folder (its name starts with
// ".cairn-tmp-"), syncs it, sets perm on it, and renames it over path. On any error the temporary file is removed
// and path is left as it was. The folder must already exist.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	panic("todo: step 1")
}
