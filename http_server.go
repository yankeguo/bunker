package bunker

import (
	"context"
	"net/http"
	"time"

	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
)

// httpServer is the public HTTP server. It deliberately does not expose the
// pprof or Prometheus handlers that the upstream toolkit registers, and it
// sets a header timeout so slow clients cannot hold connections open.
type httpServer struct {
	params ufx.ServerParams
	prober ufx.Prober
	router ufx.Router
}

type httpServerOptions struct {
	fx.In
	fx.Lifecycle

	ufx.ServerParams
	ufx.Prober
	ufx.Router
}

func NewHTTPServer(opts httpServerOptions) ufx.Server {
	s := &httpServer{
		params: opts.ServerParams,
		prober: opts.Prober,
		router: opts.Router,
	}

	if opts.Lifecycle != nil {
		hs := &http.Server{
			Addr:              opts.Listen,
			Handler:           s,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			MaxHeaderBytes:    1 << 16,
		}
		opts.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				chErr := make(chan error, 1)
				go func() {
					chErr <- hs.ListenAndServe()
				}()
				select {
				case err := <-chErr:
					return err
				case <-ctx.Done():
					return hs.Shutdown(ctx)
				case <-time.After(opts.Delay.Start):
					return nil
				}
			},
			OnStop: func(ctx context.Context) error {
				time.Sleep(opts.Delay.Stop)
				return hs.Shutdown(ctx)
			},
		})
	}

	return s
}

func (s *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w = &secureResponseWriter{ResponseWriter: w}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	}

	// readiness is checked first so it wins when both paths are configured
	// to the same URL
	if r.URL.Path == s.params.Path.Readiness {
		s.serveReadiness(w, r)
		return
	}
	if r.URL.Path == s.params.Path.Liveness {
		s.serveLiveness(w, r)
		return
	}

	s.router.ServeHTTP(w, r)
}

func (s *httpServer) serveReadiness(w http.ResponseWriter, r *http.Request) {
	msg, ready := s.prober.CheckReadiness(r.Context())
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !ready {
		w.WriteHeader(http.StatusInternalServerError)
	}
	_, _ = w.Write([]byte(msg))
}

func (s *httpServer) serveLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if s.prober.CheckLiveness() {
		_, _ = w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("CASCADED FAILURE"))
}

type secureResponseWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *secureResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
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
