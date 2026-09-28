package bunker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	startupDelay  = 3 * time.Second
	shutdownDelay = 3 * time.Second

	// maxRequestBody bounds every request. State-changing routes apply a
	// tighter limit before decoding JSON.
	maxRequestBody = 1 << 20
)

// statusError is a client error with an HTTP status. The message is returned
// as JSON {"message": "..."}.
type statusError struct {
	status int
	msg    string
}

func (e *statusError) Error() string { return e.msg }

func httpFail(status int, msg string) error {
	return &statusError{status: status, msg: msg}
}

// HTTPServer is the public HTTP server. It serves the API, the embedded UI,
// and the liveness/readiness probes. Profiling and metrics endpoints are not
// registered.
type HTTPServer struct {
	srv *http.Server
}

func NewHTTPServer(lc fx.Lifecycle, cfg Config, app *App, log *zap.SugaredLogger) *HTTPServer {
	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           newHandler(app),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 16,
		ErrorLog:          zap.NewStdLog(log.Desugar()),
	}

	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				chErr := make(chan error, 1)
				go func() {
					chErr <- srv.ListenAndServe()
				}()
				select {
				case err := <-chErr:
					return err
				case <-ctx.Done():
					return srv.Shutdown(ctx)
				case <-time.After(startupDelay):
					return nil
				}
			},
			OnStop: func(ctx context.Context) error {
				time.Sleep(shutdownDelay)
				return srv.Shutdown(ctx)
			},
		})
	}

	return &HTTPServer{srv: srv}
}

func newHandler(app *App) http.Handler {
	// API routes live on their own mux. The UI catch-all is separate so a
	// GET to a POST-only route is 405 instead of the SPA's 404 page.
	api := http.NewServeMux()
	api.HandleFunc("GET /debug/alive", serveProbe)
	api.HandleFunc("GET /debug/ready", serveProbe)
	if app != nil {
		app.mount(api)
	}

	pages := http.NewServeMux()
	installStatic(pages)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = &secureResponseWriter{ResponseWriter: w}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBody)
		}
		if isServicePath(r.URL.Path) {
			api.ServeHTTP(w, r)
			return
		}
		pages.ServeHTTP(w, r)
	})
}

func isServicePath(p string) bool {
	return p == "/backend" || strings.HasPrefix(p, "/backend/") || p == "/debug" || strings.HasPrefix(p, "/debug/")
}

func serveProbe(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("OK"))
}

func decodeJSON(r *http.Request, dst any) error {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return httpFail(http.StatusRequestEntityTooLarge, "request body is too large")
	}
	return httpFail(http.StatusBadRequest, "invalid json")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}

func writeAPIError(log *zap.SugaredLogger, w http.ResponseWriter, err error) {
	var se *statusError
	if errors.As(err, &se) {
		writeJSON(w, se.status, map[string]string{"message": se.msg})
		return
	}
	if log != nil {
		log.With("err", err).Error("request failed")
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "internal server error"})
}

type secureResponseWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *secureResponseWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	h := w.ResponseWriter.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "frame-ancestors 'none'")
	w.ResponseWriter.WriteHeader(code)
}

func (w *secureResponseWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *secureResponseWriter) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *secureResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
