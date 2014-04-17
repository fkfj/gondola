package blobstore

import (
	"fmt"
	"gnd.la/blobstore/driver"
	"gnd.la/config"
	"io"
	"reflect"
)

var (
	imports = map[string]string{
		"file":   "gnd.la/blobstore/driver/file",
		"gridfs": "gnd.la/blobstore/driver/gridfs",
		"s3":     "gnd.la/blobstore/driver/s3",
	}
)

// Store represents a connection to a blobstore. Use New()
// to initialize a Store and Store.Close to close it.
type Store struct {
	drv driver.Driver
}

// New returns a new *Store using the given url as its configure
// the URL scheme represents the driver used and the rest of the
// values in the URL are driver dependent. Please, see the package
// documentation for the available drivers and each driver sub-package
// for driver-specific documentation.
func New(url *config.URL) (*Store, error) {
	if url == nil {
		return nil, fmt.Errorf("blobstore is not configured")
	}
	opener := driver.Get(url.Scheme)
	if opener == nil {
		if imp := imports[url.Scheme]; imp != "" {
			return nil, fmt.Errorf("please import %q to use the blobstore driver %q", imp, url.Scheme)
		}
		return nil, fmt.Errorf("unknown blobstore driver %q. Perhaps you forgot an import?", url.Scheme)
	}
	drv, err := opener(url)
	if err != nil {
		return nil, fmt.Errorf("error opening blobstore driver %q: %s", url.Scheme, err)
	}
	return &Store{
		drv: drv,
	}, nil
}

// Create returns a new file for writing and sets its metadata
// to meta (which might be nil). Note that the file should be
// closed by calling WFile.Close.
func (s *Store) Create(meta interface{}) (*WFile, error) {
	return s.CreateId(newId(), meta)
}

// CreateId works like Create, but uses the given id rather than generating
// a new one. If a file with the same id already exists, it's overwritten.
func (s *Store) CreateId(id string, meta interface{}) (wfile *WFile, err error) {
	var w driver.WFile
	w, err = s.drv.Create(id)
	if err != nil {
		panic(err)
		return
	}
	defer func() {
		if err != nil {
			w.Close()
			s.drv.Remove(id)
		}
	}()
	// Write version number
	if err = bwrite(w, uint8(1)); err != nil {
		return
	}
	// Write flags
	if err = bwrite(w, uint64(0)); err != nil {
		return
	}
	metadataLength := uint64(0)
	if meta != nil && !isNil(meta) {
		var d []byte
		d, err = marshal(meta)
		if err != nil {
			return
		}
		metadataLength = uint64(len(d))
		if err = bwrite(w, metadataLength); err != nil {
			return
		}
		h := newHash()
		h.Write(d)
		if err = bwrite(w, h.Sum64()); err != nil {
			return
		}
		if _, err = w.Write(d); err != nil {
			return
		}
	} else {
		// No metadata. Write 0 for the length and the hash
		if err = bwrite(w, uint64(0)); err != nil {
			return
		}
		if err = bwrite(w, uint64(0)); err != nil {
			return
		}
	}
	seeker, ok := w.(io.Seeker)
	if ok {
		// Reserve 16 bytes for data header
		if err = bwrite(w, uint64(0)); err != nil {
			return
		}
		if err = bwrite(w, uint64(0)); err != nil {
			return
		}
	}
	// File is ready for writing. Hand it to the user.
	return &WFile{
		id:             id,
		metadataLength: metadataLength,
		dataHash:       newHash(),
		wfile:          w,
		seeker:         seeker,
	}, nil
}

// Open opens the file with the given id for reading. Note that
// the file should closed by calling RFile.Close after you're
// done with it.
func (s *Store) Open(id string) (rfile *RFile, err error) {
	var r driver.RFile
	r, err = s.drv.Open(id)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			rfile = nil
			r.Close()
		}
	}()
	var version uint8
	if err = bread(r, &version); err != nil {
		return
	}
	if version != 1 {
		err = fmt.Errorf("can't read files with version %d", version)
		return
	}
	// Skip over the flags for now
	var flags uint64
	if err = bread(r, &flags); err != nil {
		return
	}
	rfile = &RFile{
		id:    id,
		rfile: r,
	}
	var metadataLength uint64
	if err = bread(r, &metadataLength); err != nil {
		return
	}
	if err = bread(r, &rfile.metadataHash); err != nil {
		return
	}
	if metadataLength > 0 {
		rfile.metadataData = make([]byte, int(metadataLength))
		if _, err = r.Read(rfile.metadataData); err != nil {
			return
		}
	}
	if err = bread(r, &rfile.dataLength); err != nil {
		return
	}
	if err = bread(r, &rfile.dataHash); err != nil {
		return
	}
	return
}

// ReadAll is a shorthand for Open(f).ReadAll()
func (s *Store) ReadAll(id string) (data []byte, err error) {
	f, err := s.Open(id)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadAll()
}

// Store works like StoreId, but generates a new id for the file.
func (s *Store) Store(b []byte, meta interface{}) (string, error) {
	return s.StoreId(newId(), b, meta)
}

// StoreId is a shorthand for storing the given data in b and the metadata
// in meta with the given file id. If a file with the same id exists, it's
// overwritten.
func (s *Store) StoreId(id string, b []byte, meta interface{}) (string, error) {
	f, err := s.CreateId(id, meta)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(b); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Id(), nil
}

// Remove deletes the file with the given id.
func (s *Store) Remove(id string) error {
	return s.drv.Remove(id)
}

// Close closes the connection to the blobstore.
func (s *Store) Close() error {
	return s.drv.Close()
}

func isNil(v interface{}) bool {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		return val.IsNil()
	}
	return false
}
