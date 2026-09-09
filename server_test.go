package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHandleBasemapRange checks that GET /basemap.pmtiles honors Range
// requests: the PMTiles JS client relies on this (docs/WALKMAP.md piece 3).
func TestHandleBasemapRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "basemap.pmtiles")
	content := []byte("0123456789abcdef")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	handler := handleBasemap(path)

	req := httptest.NewRequest("GET", "/basemap.pmtiles", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != 206 {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if got := rec.Body.String(); got != "0123" {
		t.Fatalf("body = %q, want %q", got, "0123")
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 0-3/16" {
		t.Fatalf("Content-Range = %q", got)
	}
}
