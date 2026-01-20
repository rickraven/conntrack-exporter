package procfs

import (
	"os"
	"path/filepath"
)

// FS is a very small helper around a procfs mount point.
//
// We keep it intentionally minimal:
// - reading `/proc/net/nf_conntrack`
// - reading/writing sysctl values under `/proc/sys/...`
//
// This abstraction makes it easy to test the exporter manually by pointing
// --path.procfs to a custom directory layout (e.g. `.code`).
type FS struct {
	Root string
}

func (fs FS) Path(rel string) string {
	return filepath.Join(fs.Root, rel)
}

func (fs FS) ReadFile(rel string) ([]byte, error) {
	return os.ReadFile(fs.Path(rel))
}

func (fs FS) WriteFile(rel string, data []byte, perm os.FileMode) error {
	return os.WriteFile(fs.Path(rel), data, perm)
}

