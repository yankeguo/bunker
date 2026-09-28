package bunker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPServerDoesNotExposePprof(t *testing.T) {
	app := &App{
		log:             slog.New(slog.DiscardHandler),
		signInLimiter:   newRateLimiter(signInRateLimit, signInRateWindow),
		passwordLimiter: newRateLimiter(passwordRateLimit, passwordRateWindow),
	}
	h := newHandler(app)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("pprof status = %d, want 404", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "profile") {
		t.Fatalf("pprof body leaked: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/metrics", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("metrics status = %d, want 404", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/alive", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "OK" {
		t.Fatalf("liveness = %d %q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" || rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers = %#v", rr.Header())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/ready", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "OK" {
		t.Fatalf("readiness = %d %q", rr.Code, rr.Body.String())
	}

	form := httptest.NewRequest(http.MethodPost, "/backend/sign_in", strings.NewReader("username=a&password=b"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, form)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form sign-in status = %d, body %s", rr.Code, rr.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/backend/sign_in", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, get)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET sign-in status = %d, want 405", rr.Code)
	}

	empty := httptest.NewRequest(http.MethodPost, "/backend/sign_in", strings.NewReader(`{}`))
	empty.Header.Set("Content-Type", "application/json; charset=utf-8")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, empty)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "username and password are required") {
		t.Fatalf("empty sign-in = %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("index status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/not-a-page", nil))
	if rr.Code != http.StatusNotFound || !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("spa 404 = %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/backend/missing", nil))
	if rr.Code != http.StatusNotFound || strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("api 404 = %d %s body %s", rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
	}
}

func TestDecodeJSON(t *testing.T) {
	var dst struct {
		Username string `json:"username"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	if err := decodeJSON(req, &dst); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("[]"))
	if err := decodeJSON(req, &dst); err == nil {
		t.Fatal("expected an array to be rejected")
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"username":"a"}{"username":"b"}`))
	if err := decodeJSON(req, &dst); err == nil {
		t.Fatal("expected trailing json to be rejected")
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"username":"a"}`))
	req.Body = http.MaxBytesReader(nil, req.Body, 8)
	if err := decodeJSON(req, &dst); err == nil {
		t.Fatal("expected a truncated body to be rejected")
	} else {
		var se *statusError
		if !errors.As(err, &se) || se.status != http.StatusRequestEntityTooLarge {
			t.Fatalf("err = %v", err)
		}
	}
}

func TestRequestBodyLimit(t *testing.T) {
	app := &App{
		log:             slog.New(slog.DiscardHandler),
		signInLimiter:   newRateLimiter(signInRateLimit, signInRateWindow),
		passwordLimiter: newRateLimiter(passwordRateLimit, passwordRateWindow),
	}
	body := `{"username":"` + strings.Repeat("a", 300<<10) + `","password":"secret1"}`
	rr := doJSON(t, newHandler(app), http.MethodPost, "/backend/sign_in", body, "")
	requireStatus(t, rr, http.StatusRequestEntityTooLarge)
}

func TestHandlerRecoversFromPanic(t *testing.T) {
	app := &App{log: slog.New(slog.DiscardHandler)}
	rr := httptest.NewRecorder()
	app.call(rr, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) error {
		panic("boom")
	})
	requireStatus(t, rr, http.StatusInternalServerError)
	if strings.Contains(rr.Body.String(), "boom") {
		t.Fatalf("panic leaked: %s", rr.Body.String())
	}
}

func TestSecureResponseWriterSetsHeadersOnce(t *testing.T) {
	rr := httptest.NewRecorder()
	w := &secureResponseWriter{ResponseWriter: rr}
	w.Flush()
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("flush status = %d headers %#v", rr.Code, rr.Header())
	}
	w.WriteHeader(http.StatusCreated)
	if rr.Code != http.StatusOK {
		t.Fatalf("second WriteHeader changed status to %d", rr.Code)
	}
	if w.Unwrap() != rr {
		t.Fatal("Unwrap did not return the underlying writer")
	}
}

func TestHTTPServerTimeoutsAndShutdown(t *testing.T) {
	cfg := Config{}
	cfg.Server.Listen = "127.0.0.1:0"
	server := NewHTTPServer(cfg, nil, slog.New(slog.DiscardHandler))
	if server.srv.ReadHeaderTimeout != 10*time.Second || server.srv.IdleTimeout != 2*time.Minute || server.srv.MaxHeaderBytes != 1<<16 {
		t.Fatalf("server = %+v", server.srv)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cfg.Server.Listen = addr
	server = NewHTTPServer(cfg, nil, slog.New(slog.DiscardHandler))
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()

	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	for {
		resp, err = http.Get("http://" + addr + "/debug/alive")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "OK" {
		t.Fatalf("alive = %d %q", resp.StatusCode, body)
	}
	if err = server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("http server did not stop")
	}
}

func TestIsServicePath(t *testing.T) {
	for _, path := range []string{"/backend", "/backend/sign_in", "/debug", "/debug/alive"} {
		if !isServicePath(path) {
			t.Fatalf("%s should be a service path", path)
		}
	}
	for _, path := range []string{"/", "/dashboard", "/backender", "/debugger"} {
		if isServicePath(path) {
			t.Fatalf("%s should be a page path", path)
		}
	}
}
