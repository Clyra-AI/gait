package ui

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/davidahmann/gait/internal/uiassets"
)

func NewStaticHandler() (http.Handler, error) {
	subtree, err := fs.Sub(uiassets.Files, "dist")
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cleanPath := path.Clean("/" + strings.TrimSpace(request.URL.Path))
		relativePath := strings.TrimPrefix(cleanPath, "/")
		if relativePath == "" {
			relativePath = "index.html"
		}
		if _, statErr := fs.Stat(subtree, relativePath); statErr != nil {
			relativePath = "index.html"
		}
		payload, readErr := fs.ReadFile(subtree, relativePath)
		if readErr != nil {
			http.NotFound(writer, request)
			return
		}
		contentType := mime.TypeByExtension(path.Ext(relativePath))
		if contentType == "" {
			contentType = http.DetectContentType(payload)
		}
		writer.Header().Set("Content-Type", contentType)
		http.ServeContent(writer, request, relativePath, time.Time{}, bytes.NewReader(payload))
	}), nil
}
