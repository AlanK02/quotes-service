package router

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gorilla/mux"
	"quotes-service/internal/http-server/handlers/quotehandler"
	mwLogger "quotes-service/internal/http-server/middleware/logger"
)

func New(logger *slog.Logger, qs quotehandler.QuoteStore) http.Handler {
	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					logger.Error("panic recovered", slog.Any("panic", rvr), slog.String("stack", string(debug.Stack())))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	})

	router.Use(mwLogger.New(logger))
	router.HandleFunc("/quotes", quotehandler.NewAddQuoteHandler(logger, qs)).Methods(http.MethodPost)
	router.HandleFunc("/quotes", quotehandler.NewGetQuotesByAuthorHandler(logger, qs)).Methods(http.MethodGet).Queries("author", "{author}")
	router.HandleFunc("/quotes", quotehandler.NewGetAllQuotesHandler(logger, qs)).Methods(http.MethodGet)
	router.HandleFunc("/quotes/random", quotehandler.NewGetRandomQuoteHandler(logger, qs)).Methods(http.MethodGet)
	router.HandleFunc("/quotes/{id:[0-9]+}", quotehandler.NewDeleteQuoteHandler(logger, qs)).Methods(http.MethodDelete)

	return router
}