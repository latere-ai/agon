package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAssetCacheControl verifies path-scoped cache policy: hashed
// /assets/* are immutable, fonts/static get stale-while-revalidate,
// and SPA routes fall through (no cache) to the no-store index.
func TestAssetCacheControl(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/assets/app-abc123.js", immutableAssetCache},
		{"/fonts/inter-400.woff2", staticAssetCache},
		{"/static/og.svg", staticAssetCache},
		{"/about", ""},
		{"/", ""},
	}
	for _, tc := range cases {
		if got := assetCacheControl(tc.path); got != tc.want {
			t.Errorf("assetCacheControl(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// With only dist/PLACEHOLDER embedded (no index.html — the state of
// `go build ./...` and CI without the Bun stage), MountSPA must
// report "not mounted" and SPAFallback must serve a clean 503 rather
// than panic or 500.
func TestSPAWithoutFrontendBuild(t *testing.T) {
	mux := http.NewServeMux()
	if MountSPA(mux) {
		t.Fatal("MountSPA returned true without an embedded index.html")
	}
	SPAFallback(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET / = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
