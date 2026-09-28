package bunker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":        {Data: []byte("home")},
		"app.js":            {Data: []byte("js")},
		"nested/index.html": {Data: []byte("nested")},
		"404.html":          {Data: []byte("missing-page")},
	}
	h := spaHandler{fsys: fsys, fileServer: http.FileServer(http.FS(fsys))}

	rr := serveSPA(h, "/")
	if rr.Code != http.StatusOK || rr.Body.String() != "home" {
		t.Fatalf("index = %d %q", rr.Code, rr.Body.String())
	}

	rr = serveSPA(h, "/app.js")
	if rr.Code != http.StatusOK || rr.Body.String() != "js" {
		t.Fatalf("file = %d %q", rr.Code, rr.Body.String())
	}

	rr = serveSPA(h, "/../app.js")
	if rr.Code != http.StatusOK || rr.Body.String() != "js" {
		t.Fatalf("cleaned path = %d %q", rr.Code, rr.Body.String())
	}

	rr = serveSPA(h, "/nested/")
	if rr.Code != http.StatusOK || rr.Body.String() != "nested" {
		t.Fatalf("directory = %d %q", rr.Code, rr.Body.String())
	}

	rr = serveSPA(h, "/no-such")
	if rr.Code != http.StatusNotFound || rr.Body.String() != "missing-page" {
		t.Fatalf("fallback = %d %q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("content type = %s", rr.Header().Get("Content-Type"))
	}
}

func TestSPAHandlerWithoutFallbackPage(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte("home")}}
	h := spaHandler{fsys: fsys, fileServer: http.FileServer(http.FS(fsys))}
	rr := serveSPA(h, "/missing")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func serveSPA(h spaHandler, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr
}
