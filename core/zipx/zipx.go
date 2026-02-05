package zipx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type File struct {
	Path string
	Data []byte
	Mode os.FileMode
}

const fixedTime = "1980-01-01T00:00:00Z"

// WriteDeterministicZip writes a byte-stable zip to the provided writer.
func WriteDeterministicZip(w io.Writer, files []File) error {
	if len(files) == 0 {
		return nil
	}
	items := make([]File, len(files))
	copy(items, files)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	t, _ := time.Parse(time.RFC3339, fixedTime)
	zw := zip.NewWriter(w)
	for _, f := range items {
		h := &zip.FileHeader{
			Name:   filepath.ToSlash(f.Path),
			Method: zip.Deflate,
		}
		h.Modified = t
		h.SetMode(normalizeMode(f.Mode))
		wr, err := zw.CreateHeader(h)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := io.Copy(wr, bytes.NewReader(f.Data)); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func normalizeMode(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o644
	}
	if mode&0o111 != 0 {
		return 0o755
	}
	return 0o644
}
