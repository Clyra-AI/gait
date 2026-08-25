package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func safeTokenPath(path string) error {
	p := filepath.Dir(path)
	i, e := os.Lstat(p)
	if e != nil {
		return e
	}
	if i.Mode()&os.ModeSymlink != 0 {
		return errors.New("token parent symlink")
	}
	if d, e := os.Lstat(path); e == nil && !d.Mode().IsRegular() {
		return errors.New("token destination invalid")
	}
	return nil
}
func readTokenFile(path string) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("token file invalid")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, 4<<20))
}
func strictTokenDecode(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	e := d.Decode(&extra)
	if e == io.EOF {
		return nil
	}
	return errors.New("trailing token JSON")
}
