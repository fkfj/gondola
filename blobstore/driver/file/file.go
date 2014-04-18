package file

import (
	"os"
	"path"
	"path/filepath"

	"gnd.la/blobstore/driver"
	"gnd.la/config"
	"gnd.la/util/pathutil"
)

type fsDriver struct {
	dir    string
	tmpDir string
}

func (f *fsDriver) tmp(id string) string {
	return filepath.Join(f.tmpDir, id)
}

func (f *fsDriver) path(id string) string {
	// Use the last two bytes as the dirname, since the
	// two first increase monotonically with time
	ext := path.Ext(id)
	if ext != "" {
		id = id[:len(id)-len(ext)]
	}
	sep := len(id) - 2
	return filepath.Join(f.dir, id[sep:], id[:sep]+ext)
}

func (f *fsDriver) Create(id string) (driver.WFile, error) {
	tmp := filepath.Join(f.tmpDir, id)
	fp, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, err
	}
	return &wfile{
		File: fp,
		path: f.path(id),
	}, nil
}

func (f *fsDriver) Open(id string) (driver.RFile, error) {
	return os.Open(f.path(id))
}

func (f *fsDriver) Remove(id string) error {
	return os.Remove(f.path(id))
}

func (f *fsDriver) Close() error {
	return nil
}

func fsOpener(url *config.URL) (driver.Driver, error) {
	value := url.Value
	if !filepath.IsAbs(value) {
		value = pathutil.Relative(value)
	}
	tmpDir := filepath.Join(value, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, err
	}
	return &fsDriver{
		dir:    value,
		tmpDir: tmpDir,
	}, nil
}

func init() {
	driver.Register("file", fsOpener)
}
