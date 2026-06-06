package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// TestSPAFallbackServesTopLevelFiles pins the bug fix: top-level static
// files (favicon.svg, robots.txt, ...) live at the dist root and match
// none of MountSPA's prefix routes, so they reach the GET / fallback.
// The fallback must serve the real file, not the HTML index, while an
// unmatched deep link must still fall back to index.html.
func TestSPAFallbackServesTopLevelFiles(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":  {Data: []byte("<html>app</html>")},
		"favicon.svg": {Data: []byte("<svg>icon</svg>")},
		"robots.txt":  {Data: []byte("User-agent: *\n")},
	}
	h := fallbackHandler(dist)

	cases := []struct {
		path     string
		wantBody string
	}{
		{"/favicon.svg", "<svg>icon</svg>"},
		{"/robots.txt", "User-agent: *\n"},
		{"/some/deep/route", "<html>app</html>"}, // unknown route -> index
		{"/", "<html>app</html>"},                // root -> index
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.path, rec.Code)
		}
		if got := rec.Body.String(); got != tc.wantBody {
			t.Errorf("GET %s body = %q, want %q", tc.path, got, tc.wantBody)
		}
	}
}

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
