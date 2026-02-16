package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/api/routes"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

func statusColor(code int) string {
	switch {
	case code >= 500:
		return colorRed
	case code >= 400:
		return colorYellow
	case code >= 300:
		return colorCyan
	default:
		return colorGreen
	}
}

func methodColor(method string) string {
	switch method {
	case "GET":
		return colorGreen
	case "POST":
		return colorCyan
	case "PUT", "PATCH":
		return colorYellow
	case "DELETE":
		return colorRed
	default:
		return colorReset
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		status := ww.Status()
		duration := time.Since(start)

		fmt.Fprintf(os.Stdout, "%s%-7s%s %s %s%d%s %s%s%s\n",
			methodColor(r.Method), r.Method, colorReset,
			r.URL.Path,
			statusColor(status), status, colorReset,
			colorDim, duration, colorReset,
		)
	})
}

func NewServer(pool *pgxpool.Pool, port string) *http.Server {
	r := chi.NewRouter()

	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Mount("/projects", routes.ProjectRoutes(pool))
	r.Post("/scan", routes.ScanHandler())
	r.Mount("/search", routes.SearchRoutes())
	r.Mount("/debug", routes.DebugRoutes())

	return &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
}

func Run(pool *pgxpool.Pool, port string) error {
	srv := NewServer(pool, port)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
