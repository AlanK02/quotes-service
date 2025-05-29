package quotehandler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"quotes-service/internal/models"
	"quotes-service/internal/storage"
)


var ErrorsIs = errors.Is

type QuoteStore interface {
	AddQuote(ctx context.Context, text string, author string) (int64, error)
	GetAllQuotes(ctx context.Context) ([]models.Quote, error)
	GetRandomQuote(ctx context.Context) (models.Quote, error)
	GetQuotesByAuthor(ctx context.Context, authorFilter string) ([]models.Quote, error)
	DeleteQuote(ctx context.Context, id int64) error
}

func sendJSONResponse(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode and write JSON response", slog.String("error", err.Error()))
	}
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, message string, fields []string) {
	response := models.ErrorResponse{
		Status: "error",
		Error:  message,
	}
	if len(fields) > 0 {
		response.Fields = fields
	}
	sendJSONResponse(w, statusCode, response)
}

func NewAddQuoteHandler(logger *slog.Logger, qs QuoteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handler.quote.AddQuote"
		log := logger.With(slog.String("op", op))
		ctx := r.Context()

		var req models.AddQuoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if ErrorsIs(err, io.EOF) {
				log.WarnContext(ctx, "request body is empty")
				sendErrorResponse(w, http.StatusBadRequest, "Request body is empty.", nil)
				return
			}
			log.ErrorContext(ctx, "failed to decode request body", slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusBadRequest, "Failed to decode request body.", nil)
			return
		}
		defer r.Body.Close()

		log.InfoContext(ctx, "request body decoded", slog.Group("request", slog.String("text", req.Text), slog.String("author", req.Author)))


		var validationErrors []string
		if strings.TrimSpace(req.Text) == "" {
			validationErrors = append(validationErrors, "text cannot be empty")
		}
		if strings.TrimSpace(req.Author) == "" {
			validationErrors = append(validationErrors, "author cannot be empty")
		}

		if len(validationErrors) > 0 {
			log.WarnContext(ctx, "invalid request", slog.Any("validation_errors", validationErrors))
			sendErrorResponse(w, http.StatusBadRequest, "Invalid request.", validationErrors)
			return
		}

		id, err := qs.AddQuote(ctx, req.Text, req.Author)
		if err != nil {
			log.ErrorContext(ctx, "failed to add quote to storage", slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to add quote.", nil)
			return
		}

		log.InfoContext(ctx, "quote added successfully", slog.Int64("id", id))
		sendJSONResponse(w, http.StatusCreated, models.AddQuoteResponse{
			Status: "success",
			ID:     id,
			Text:   req.Text,
			Author: req.Author,
		})
	}
}

func NewGetAllQuotesHandler(logger *slog.Logger, qs QuoteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handler.quote.GetAllQuotes"
		log := logger.With(slog.String("op", op))
		ctx := r.Context()

		quotes, err := qs.GetAllQuotes(ctx)
		if err != nil {
			log.ErrorContext(ctx, "failed to get all quotes", slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve quotes.", nil)
			return
		}

		log.InfoContext(ctx, "retrieved all quotes", slog.Int("count", len(quotes)))
		sendJSONResponse(w, http.StatusOK, models.SuccessDataResponse{
			Status: "success",
			Data:   quotes,
		})
	}
}

func NewGetRandomQuoteHandler(logger *slog.Logger, qs QuoteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handler.quote.GetRandomQuote"
		log := logger.With(slog.String("op", op))
		ctx := r.Context()

		quote, err := qs.GetRandomQuote(ctx)
		if err != nil {
			if ErrorsIs(err, storage.ErrQuoteNotFound) {
				log.InfoContext(ctx, "no quotes found to get a random one")
				sendErrorResponse(w, http.StatusNotFound, "No quotes found.", nil)
				return
			}
			log.ErrorContext(ctx, "failed to get random quote", slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve random quote.", nil)
			return
		}

		log.InfoContext(ctx, "retrieved random quote", slog.Int64("id", quote.ID))
		sendJSONResponse(w, http.StatusOK, models.SuccessDataResponse{
			Status: "success",
			Data:   quote,
		})
	}
}

func NewGetQuotesByAuthorHandler(logger *slog.Logger, qs QuoteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handler.quote.GetQuotesByAuthor"
		log := logger.With(slog.String("op", op))
		ctx := r.Context()

		author := r.URL.Query().Get("author")
		if strings.TrimSpace(author) == "" {
			log.WarnContext(ctx, "author query parameter is missing or empty")
			sendErrorResponse(w, http.StatusBadRequest, "Author query parameter is required.", nil)
			return
		}

		log.InfoContext(ctx, "fetching quotes by author", slog.String("author", author))

		quotes, err := qs.GetQuotesByAuthor(ctx, author)
		if err != nil {
			log.ErrorContext(ctx, "failed to get quotes by author", slog.String("author", author), slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve quotes by author.", nil)
			return
		}

		log.InfoContext(ctx, "retrieved quotes by author", slog.String("author", author), slog.Int("count", len(quotes)))
		sendJSONResponse(w, http.StatusOK, models.SuccessDataResponse{
			Status: "success",
			Data:   quotes,
		})
	}
}

func NewDeleteQuoteHandler(logger *slog.Logger, qs QuoteStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handler.quote.DeleteQuote"
		log := logger.With(slog.String("op", op))
		ctx := r.Context()

		vars := mux.Vars(r)
		idStr, ok := vars["id"]
		if !ok {
			log.WarnContext(ctx, "quote ID not found in path")
			sendErrorResponse(w, http.StatusBadRequest, "Quote ID is missing in path.", nil)
			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			log.WarnContext(ctx, "invalid quote ID format", slog.String("id", idStr), slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusBadRequest, "Invalid quote ID format.", nil)
			return
		}

		log.InfoContext(ctx, "attempting to delete quote", slog.Int64("id", id))

		err = qs.DeleteQuote(ctx, id)
		if err != nil {
			if ErrorsIs(err, storage.ErrQuoteNotFound) {
				log.InfoContext(ctx, "quote not found for deletion", slog.Int64("id", id))
				sendErrorResponse(w, http.StatusNotFound, "Quote not found.", nil)
				return
			}
			log.ErrorContext(ctx, "failed to delete quote", slog.Int64("id", id), slog.String("error", err.Error()))
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to delete quote.", nil)
			return
		}

		log.InfoContext(ctx, "quote deleted successfully", slog.Int64("id", id))
		sendJSONResponse(w, http.StatusOK, models.GenericMessageResponse{
			Status:  "success",
			Message: "Quote deleted successfully.",
		})
	}
}