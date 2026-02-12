package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticHandlerServesIndexAndAssets(t *testing.T) {
	handler, err := NewStaticHandler()
	if err != nil {
		t.Fatalf("new static handler: %v", err)
	}

	indexRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	indexResponse := httptest.NewRecorder()
	handler.ServeHTTP(indexResponse, indexRequest)
	if indexResponse.Code != http.StatusOK {
		t.Fatalf("index status: expected %d got %d", http.StatusOK, indexResponse.Code)
	}
	if !strings.Contains(indexResponse.Body.String(), "Gait Local UI") {
		t.Fatalf("expected index html content")
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	assetResponse := httptest.NewRecorder()
	handler.ServeHTTP(assetResponse, assetRequest)
	if assetResponse.Code != http.StatusOK {
		t.Fatalf("asset status: expected %d got %d", http.StatusOK, assetResponse.Code)
	}
	if !strings.Contains(assetResponse.Body.String(), "<!DOCTYPE html>") {
		t.Fatalf("expected index fallback html content")
	}

	directAssetRequest := httptest.NewRequest(http.MethodGet, "/404.html", nil)
	directAssetResponse := httptest.NewRecorder()
	handler.ServeHTTP(directAssetResponse, directAssetRequest)
	if directAssetResponse.Code != http.StatusOK {
		t.Fatalf("direct asset status: expected %d got %d", http.StatusOK, directAssetResponse.Code)
	}
	if !strings.Contains(directAssetResponse.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected html content type, got %q", directAssetResponse.Header().Get("Content-Type"))
	}

	directoryRequest := httptest.NewRequest(http.MethodGet, "/_next", nil)
	directoryResponse := httptest.NewRecorder()
	handler.ServeHTTP(directoryResponse, directoryRequest)
	if directoryResponse.Code != http.StatusNotFound {
		t.Fatalf("directory request status: expected %d got %d", http.StatusNotFound, directoryResponse.Code)
	}
}
