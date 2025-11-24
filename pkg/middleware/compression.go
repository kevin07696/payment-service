package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Gzip compression levels
const (
	GzipBestSpeed       = gzip.BestSpeed          // 1 - Fastest, lower compression
	GzipBestCompression = gzip.BestCompression    // 9 - Slowest, best compression
	GzipDefaultLevel    = gzip.DefaultCompression // 6 - Balanced
)

// Pool of gzip writers to reduce allocations
// Reusing writers is critical for performance at high RPS
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		// Create writer with default compression level
		w, _ := gzip.NewWriterLevel(io.Discard, GzipDefaultLevel)
		return w
	},
}

// gzipResponseWriter wraps http.ResponseWriter with gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter *gzip.Writer
	statusCode int
	written    bool
}

// WriteHeader captures status code and writes headers
func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write compresses response body
func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
		// Set status code if not already set
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
	}
	return w.gzipWriter.Write(b)
}

// GzipHandler wraps an HTTP handler with gzip compression
// Returns middleware function that can be chained
func GzipHandler(level int, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if client accepts gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Don't compress if content is already compressed or not compressible
			contentType := w.Header().Get("Content-Type")
			if contentType != "" && !CompressibleContentType(contentType) {
				next.ServeHTTP(w, r)
				return
			}

			// Get gzip writer from pool
			gz := gzipWriterPool.Get().(*gzip.Writer)
			defer func() {
				gz.Close()
				gzipWriterPool.Put(gz)
			}()

			// Reset writer to use response writer
			gz.Reset(w)

			// Set compression headers
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			// Remove Content-Length as it will be incorrect after compression
			w.Header().Del("Content-Length")

			// Wrap response writer
			gzipW := &gzipResponseWriter{
				ResponseWriter: w,
				gzipWriter:     gz,
			}

			// Serve request with compressed response
			next.ServeHTTP(gzipW, r)

			// Log compression stats (debug level)
			if logger != nil {
				logger.Debug("Response compressed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", gzipW.statusCode),
				)
			}
		})
	}
}

// CompressibleContentType returns true if content type should be compressed
func CompressibleContentType(contentType string) bool {
	compressible := []string{
		"text/",
		"application/json",
		"application/javascript",
		"application/xml",
		"application/grpc",
		"application/vnd.api+json",
		"application/x-ndjson",
	}

	for _, prefix := range compressible {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}

	return false
}

// GzipHandlerWithConfig creates a gzip handler with custom configuration
type GzipConfig struct {
	Level            int      // Compression level (1-9)
	MinSize          int      // Minimum response size to compress (bytes)
	ExcludedPaths    []string // Paths to exclude from compression
	ExcludedMethods  []string // HTTP methods to exclude
	CompressibleTypes []string // Additional compressible content types
}

// DefaultGzipConfig returns default compression configuration
func DefaultGzipConfig() *GzipConfig {
	return &GzipConfig{
		Level:           GzipDefaultLevel,
		MinSize:         1024, // Don't compress responses < 1KB
		ExcludedPaths:   []string{"/health", "/metrics"},
		ExcludedMethods: []string{},
		CompressibleTypes: []string{
			"text/",
			"application/json",
			"application/javascript",
			"application/xml",
			"application/grpc",
		},
	}
}

// GzipHandlerWithCustomConfig creates handler with custom config
func GzipHandlerWithCustomConfig(cfg *GzipConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	// Log configuration at startup
	if logger != nil {
		logger.Info("Compression middleware configured",
			zap.Int("excluded_paths_count", len(cfg.ExcludedPaths)),
			zap.Strings("excluded_paths", cfg.ExcludedPaths))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check excluded paths (supports both exact match and prefix match)
			for _, path := range cfg.ExcludedPaths {
				// If path ends with '/', treat as prefix match
				if strings.HasSuffix(path, "/") {
					if strings.HasPrefix(r.URL.Path, path) {
						if logger != nil {
							logger.Debug("Skipping compression for excluded prefix",
								zap.String("path", r.URL.Path),
								zap.String("excluded_prefix", path))
						}
						next.ServeHTTP(w, r)
						return
					}
				} else if r.URL.Path == path {
					if logger != nil {
						logger.Debug("Skipping compression for excluded exact path",
							zap.String("path", r.URL.Path))
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check excluded methods
			for _, method := range cfg.ExcludedMethods {
				if r.Method == method {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if client accepts gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Create custom pool for this level if different
			var gz *gzip.Writer
			if cfg.Level == GzipDefaultLevel {
				gz = gzipWriterPool.Get().(*gzip.Writer)
			} else {
				gz, _ = gzip.NewWriterLevel(w, cfg.Level)
			}

			defer func() {
				gz.Close()
				if cfg.Level == GzipDefaultLevel {
					gzipWriterPool.Put(gz)
				}
			}()

			gz.Reset(w)

			// Set compression headers
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			w.Header().Del("Content-Length")

			// Wrap response writer
			gzipW := &gzipResponseWriter{
				ResponseWriter: w,
				gzipWriter:     gz,
			}

			next.ServeHTTP(gzipW, r)

			if logger != nil {
				logger.Debug("Response compressed with custom config",
					zap.String("path", r.URL.Path),
					zap.Int("level", cfg.Level),
					zap.Int("status", gzipW.statusCode),
				)
			}
		})
	}
}
