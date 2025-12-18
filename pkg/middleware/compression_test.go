package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Test_CompressibleContentType verifies content type detection
func Test_CompressibleContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		// Compressible types
		{
			name:        "text/html",
			contentType: "text/html",
			want:        true,
		},
		{
			name:        "text/plain",
			contentType: "text/plain",
			want:        true,
		},
		{
			name:        "application/json",
			contentType: "application/json",
			want:        true,
		},
		{
			name:        "application/json with charset",
			contentType: "application/json; charset=utf-8",
			want:        true,
		},
		{
			name:        "application/javascript",
			contentType: "application/javascript",
			want:        true,
		},
		{
			name:        "application/xml",
			contentType: "application/xml",
			want:        true,
		},
		{
			name:        "application/grpc",
			contentType: "application/grpc",
			want:        true,
		},
		{
			name:        "application/vnd.api+json",
			contentType: "application/vnd.api+json",
			want:        true,
		},
		{
			name:        "application/x-ndjson",
			contentType: "application/x-ndjson",
			want:        true,
		},
		// Non-compressible types
		{
			name:        "image/png",
			contentType: "image/png",
			want:        false,
		},
		{
			name:        "image/jpeg",
			contentType: "image/jpeg",
			want:        false,
		},
		{
			name:        "video/mp4",
			contentType: "video/mp4",
			want:        false,
		},
		{
			name:        "application/pdf",
			contentType: "application/pdf",
			want:        false,
		},
		{
			name:        "application/octet-stream",
			contentType: "application/octet-stream",
			want:        false,
		},
		{
			name:        "empty string",
			contentType: "",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := CompressibleContentType(tt.contentType)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_GzipHandler_CompressionApplied verifies gzip compression is applied
func Test_GzipHandler_CompressionApplied(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	testData := strings.Repeat("Hello, World! ", 100) // Repeatable data compresses well
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testData))
	})

	wrappedHandler := handler(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	// Act
	wrappedHandler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
	assert.Empty(t, rec.Header().Get("Content-Length"))

	// Decompress and verify content
	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, testData, string(decompressed))

	// Verify compression actually reduced size
	compressedSize := rec.Body.Len()
	originalSize := len(testData)
	assert.Less(t, compressedSize, originalSize,
		"Compressed size (%d) should be less than original (%d)", compressedSize, originalSize)
}

// Test_GzipHandler_NoCompressionWithoutAcceptEncoding verifies no compression without header
func Test_GzipHandler_NoCompressionWithoutAcceptEncoding(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	testData := "Hello, World!"
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testData))
	})

	wrappedHandler := handler(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// No Accept-Encoding header
	rec := httptest.NewRecorder()

	// Act
	wrappedHandler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, testData, rec.Body.String())
}

// Test_GzipHandler_NoCompressionForNonAcceptingClient verifies no compression for deflate-only
func Test_GzipHandler_NoCompressionForNonAcceptingClient(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	testData := "Hello, World!"
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testData))
	})

	wrappedHandler := handler(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "deflate, br") // No gzip
	rec := httptest.NewRecorder()

	// Act
	wrappedHandler.ServeHTTP(rec, req)

	// Assert
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, testData, rec.Body.String())
}

// Test_GzipHandler_StatusCodeCaptured verifies status codes are preserved
func Test_GzipHandler_StatusCodeCaptured(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			logger := zaptest.NewLogger(t)
			handler := GzipHandler(GzipDefaultLevel, logger)

			innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error": "test"}`))
			})

			wrappedHandler := handler(innerHandler)
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			// Act
			wrappedHandler.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, tt.statusCode, rec.Code)
			assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
		})
	}
}

// Test_GzipHandler_DefaultStatusCode verifies default 200 when not set
func Test_GzipHandler_DefaultStatusCode(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Don't call WriteHeader - should default to 200
		_, _ = w.Write([]byte(`{"message": "ok"}`))
	})

	wrappedHandler := handler(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	// Act
	wrappedHandler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

// Test_GzipHandlerWithCustomConfig_ExcludedPaths verifies paths are excluded
func Test_GzipHandlerWithCustomConfig_ExcludedPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	cfg := &GzipConfig{
		Level:         GzipDefaultLevel,
		ExcludedPaths: []string{"/health", "/metrics"},
	}
	handler := GzipHandlerWithCustomConfig(cfg, logger)

	testData := "Health OK"
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testData))
	})

	wrappedHandler := handler(innerHandler)

	tests := []struct {
		path           string
		shouldCompress bool
	}{
		{"/health", false},
		{"/metrics", false},
		{"/api/data", true},
		{"/healthz", true}, // Different path
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			// Act
			wrappedHandler.ServeHTTP(rec, req)

			// Assert
			if tt.shouldCompress {
				assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			} else {
				assert.Empty(t, rec.Header().Get("Content-Encoding"))
				assert.Equal(t, testData, rec.Body.String())
			}
		})
	}
}

// Test_GzipHandlerWithCustomConfig_ExcludedMethods verifies methods are excluded
func Test_GzipHandlerWithCustomConfig_ExcludedMethods(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	cfg := &GzipConfig{
		Level:           GzipDefaultLevel,
		ExcludedMethods: []string{http.MethodOptions, http.MethodHead},
	}
	handler := GzipHandlerWithCustomConfig(cfg, logger)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	wrappedHandler := handler(innerHandler)

	tests := []struct {
		method         string
		shouldCompress bool
	}{
		{http.MethodOptions, false},
		{http.MethodHead, false},
		{http.MethodGet, true},
		{http.MethodPost, true},
		{http.MethodPut, true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(tt.method, "/api/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			// Act
			wrappedHandler.ServeHTTP(rec, req)

			// Assert
			if tt.shouldCompress {
				assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			} else {
				assert.Empty(t, rec.Header().Get("Content-Encoding"))
			}
		})
	}
}

// Test_GzipHandlerWithCustomConfig_CompressionLevels verifies different levels work
func Test_GzipHandlerWithCustomConfig_CompressionLevels(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	testData := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)

	tests := []struct {
		name  string
		level int
	}{
		{"Best Speed", GzipBestSpeed},
		{"Default", GzipDefaultLevel},
		{"Best Compression", GzipBestCompression},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := &GzipConfig{
				Level: tt.level,
			}
			handler := GzipHandlerWithCustomConfig(cfg, logger)

			innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testData))
			})

			wrappedHandler := handler(innerHandler)
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			// Act
			wrappedHandler.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

			// Decompress and verify
			gr, err := gzip.NewReader(rec.Body)
			require.NoError(t, err)
			defer gr.Close()

			decompressed, err := io.ReadAll(gr)
			require.NoError(t, err)
			assert.Equal(t, testData, string(decompressed))
		})
	}
}

// Test_DefaultGzipConfig verifies default configuration values
func Test_DefaultGzipConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg := DefaultGzipConfig()

	// Assert
	assert.Equal(t, GzipDefaultLevel, cfg.Level)
	assert.Equal(t, 1024, cfg.MinSize)
	assert.Contains(t, cfg.ExcludedPaths, "/health")
	assert.Contains(t, cfg.ExcludedPaths, "/metrics")
	assert.Empty(t, cfg.ExcludedMethods)
	assert.Contains(t, cfg.CompressibleTypes, "text/")
	assert.Contains(t, cfg.CompressibleTypes, "application/json")
}

// Test_GzipHandler_PoolReuse verifies writer pool is used
func Test_GzipHandler_PoolReuse(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("test data"))
	})

	wrappedHandler := handler(innerHandler)

	// Act - Make multiple requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rec, req)

		// Assert each response is compressed
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	}

	// Pool should have writers available now
	// Note: This is a behavioral test - pool reuse reduces allocations
	// which is verified in benchmarks
}

// Test_GzipHandler_HeadersSet verifies all required headers are set
func Test_GzipHandler_HeadersSet(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key": "value"}`))
	})

	wrappedHandler := handler(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	// Act
	wrappedHandler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"),
		"Content-Encoding should be set to gzip")
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"),
		"Vary header should be set")
	// Note: Content-Length is deleted by middleware, but handler shouldn't set it anyway
	// since compressed length is unknown at handler time
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"),
		"Content-Type should be preserved")
}

// Test_GzipHandler_EmptyResponse verifies empty responses are handled
func Test_GzipHandler_EmptyResponse(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	handler := GzipHandler(GzipDefaultLevel, logger)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		// No body
	})

	wrappedHandler := handler(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	// Act
	wrappedHandler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	// Gzip will write header/footer even for empty content (10-byte minimum)
	// Decompress and verify it's empty
	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Empty(t, decompressed, "Decompressed content should be empty")
}

// Benchmark_GzipHandler_WithPool benchmarks handler with pool
func Benchmark_GzipHandler_WithPool(b *testing.B) {
	logger := zaptest.NewLogger(b)
	handler := GzipHandler(GzipDefaultLevel, logger)

	testData := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testData))
	})

	wrappedHandler := handler(innerHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rec, req)
	}
}

// Benchmark_GzipHandler_WithoutCompression benchmarks without compression
func Benchmark_GzipHandler_WithoutCompression(b *testing.B) {
	logger := zaptest.NewLogger(b)
	handler := GzipHandler(GzipDefaultLevel, logger)

	testData := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testData))
	})

	wrappedHandler := handler(innerHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		// No Accept-Encoding header
		rec := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rec, req)
	}
}

// Benchmark_CompressionLevels compares different compression levels
func Benchmark_CompressionLevels(b *testing.B) {
	logger := zaptest.NewLogger(b)
	testData := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)

	benchmarks := []struct {
		name  string
		level int
	}{
		{"BestSpeed", GzipBestSpeed},
		{"Default", GzipDefaultLevel},
		{"BestCompression", GzipBestCompression},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			cfg := &GzipConfig{
				Level: bm.level,
			}
			handler := GzipHandlerWithCustomConfig(cfg, logger)

			innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testData))
			})

			wrappedHandler := handler(innerHandler)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				rec := httptest.NewRecorder()

				wrappedHandler.ServeHTTP(rec, req)
			}
		})
	}
}

// Benchmark_PayloadSizes compares different payload sizes
func Benchmark_PayloadSizes(b *testing.B) {
	logger := zaptest.NewLogger(b)

	payloads := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, p := range payloads {
		b.Run(p.name, func(b *testing.B) {
			testData := strings.Repeat("x", p.size)
			handler := GzipHandler(GzipDefaultLevel, logger)

			innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testData))
			})

			wrappedHandler := handler(innerHandler)

			b.ResetTimer()
			b.SetBytes(int64(p.size))

			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				rec := httptest.NewRecorder()

				wrappedHandler.ServeHTTP(rec, req)
			}
		})
	}
}
