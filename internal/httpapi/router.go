package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pricesapi/internal/config"
	"pricesapi/internal/httpapi/handlers"
	"pricesapi/internal/prices"
)

func NewRouter(pool *pgxpool.Pool, logger *slog.Logger, cfg config.Config) http.Handler {
	mux := http.NewServeMux()

// проверочка, жив ли вообще сайт
	mux.HandleFunc("/health", handlers.Health)

	svc := prices.NewService(pool, logger)

	mux.HandleFunc("/api/v0/prices", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlers.PostPrices(svc, cfg)(w, r)
		case http.MethodGet:
			handlers.GetPrices(svc)(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return withMiddlewares(mux, logger, 60*time.Second)
}

func withMiddlewares(next http.Handler, logger *slog.Logger, timeout time.Duration) http.Handler {
	return Timeout(timeout)(
		RequestLogger(logger)(
			Recoverer(next),
		),
	)
}
