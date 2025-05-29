package quotehandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"quotes-service/internal/http-server/handlers/quotehandler"
	"quotes-service/internal/models"
)

var errTestQuoteNotFound = errors.New("test: quote not found")
var errTestStorageInternal = errors.New("test: internal storage error")

type MockQuoteStore struct {
	AddQuoteFunc          func(ctx context.Context, text string, author string) (int64, error)
	GetAllQuotesFunc      func(ctx context.Context) ([]models.Quote, error)
	GetRandomQuoteFunc    func(ctx context.Context) (models.Quote, error)
	GetQuotesByAuthorFunc func(ctx context.Context, authorFilter string) ([]models.Quote, error)
	DeleteQuoteFunc       func(ctx context.Context, id int64) error
}

func (m *MockQuoteStore) AddQuote(ctx context.Context, text string, author string) (int64, error) {
	if m.AddQuoteFunc != nil {
		return m.AddQuoteFunc(ctx, text, author)
	}
	return 0, errors.New("AddQuoteFunc not implemented")
}

func (m *MockQuoteStore) GetAllQuotes(ctx context.Context) ([]models.Quote, error) {
	if m.GetAllQuotesFunc != nil {
		return m.GetAllQuotesFunc(ctx)
	}
	return nil, errors.New("GetAllQuotesFunc not implemented")
}

func (m *MockQuoteStore) GetRandomQuote(ctx context.Context) (models.Quote, error) {
	if m.GetRandomQuoteFunc != nil {
		return m.GetRandomQuoteFunc(ctx)
	}
	return models.Quote{}, errors.New("GetRandomQuoteFunc not implemented")
}

func (m *MockQuoteStore) GetQuotesByAuthor(ctx context.Context, authorFilter string) ([]models.Quote, error) {
	if m.GetQuotesByAuthorFunc != nil {
		return m.GetQuotesByAuthorFunc(ctx, authorFilter)
	}
	return nil, errors.New("GetQuotesByAuthorFunc not implemented")
}

func (m *MockQuoteStore) DeleteQuote(ctx context.Context, id int64) error {
	if m.DeleteQuoteFunc != nil {
		return m.DeleteQuoteFunc(ctx, id)
	}
	return errors.New("DeleteQuoteFunc not implemented")
}

func TestAddQuoteHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	originalErrorsIs := quotehandler.ErrorsIs
	defer func() { quotehandler.ErrorsIs = originalErrorsIs }()

	tests := []struct {
		name           string
		reqBody        interface{}
		mockStoreSetup func(*MockQuoteStore)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "success",
			reqBody: models.AddQuoteRequest{Text: "Test", Author: "Author"},
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.AddQuoteFunc = func(ctx context.Context, text, author string) (int64, error) {
					return 1, nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"status":"success","id":1,"text":"Test","author":"Author"}`,
		},
		{
			name:           "empty body",
			reqBody:        "",
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Request body is empty."}`,
		},
		{
			name:           "malformed json",
			reqBody:        `{"text": "Test", "author": "Author"`,
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Failed to decode request body."}`,
		},
		{
			name:           "validation error text",
			reqBody:        models.AddQuoteRequest{Text: " ", Author: "Valid Author"},
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Invalid request.","fields":["text cannot be empty"]}`,
		},
		{
			name:           "validation error author",
			reqBody:        models.AddQuoteRequest{Text: "Valid Text", Author: " "},
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Invalid request.","fields":["author cannot be empty"]}`,
		},
		{
			name: "storage error",
			reqBody: models.AddQuoteRequest{Text: "Test", Author: "Author"},
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.AddQuoteFunc = func(ctx context.Context, text, author string) (int64, error) {
					return 0, errTestStorageInternal
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"status":"error","error":"Failed to add quote."}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockQuoteStore{}
			if tc.mockStoreSetup != nil {
				tc.mockStoreSetup(mockStore)
			}
			handler := quotehandler.NewAddQuoteHandler(logger, mockStore)

			var bodyReader io.Reader
			if reqBodyStr, ok := tc.reqBody.(string); ok && reqBodyStr == "" && tc.name == "empty body" {
				quotehandler.ErrorsIs = func(err, target error) bool { return errors.Is(err, io.EOF) }
				bodyReader = bytes.NewBuffer(nil)
			} else if reqBodyStr, ok := tc.reqBody.(string); ok {
				quotehandler.ErrorsIs = errors.Is
				bodyReader = strings.NewReader(reqBodyStr)
			} else {
				quotehandler.ErrorsIs = errors.Is
				jsonData, _ := json.Marshal(tc.reqBody)
				bodyReader = bytes.NewBuffer(jsonData)
			}

			req := httptest.NewRequest(http.MethodPost, "/quotes", bodyReader)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req.WithContext(context.Background()))

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
			if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(tc.expectedBody) {
				t.Errorf("expected body %q, got %q", tc.expectedBody, rr.Body.String())
			}
			quotehandler.ErrorsIs = originalErrorsIs
		})
	}
}

func TestGetAllQuotesHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name           string
		mockStoreSetup func(*MockQuoteStore)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "success empty",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetAllQuotesFunc = func(ctx context.Context) ([]models.Quote, error) {
					return []models.Quote{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"success","data":[]}`,
		},
		{
			name: "success non-empty",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetAllQuotesFunc = func(ctx context.Context) ([]models.Quote, error) {
					return []models.Quote{{ID: 1, Text: "Hello", Author: "World"}}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"success","data":[{"id":1,"text":"Hello","author":"World"}]}`,
		},
		{
			name: "storage error",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetAllQuotesFunc = func(ctx context.Context) ([]models.Quote, error) {
					return nil, errTestStorageInternal
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"status":"error","error":"Failed to retrieve quotes."}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockQuoteStore{}
			tc.mockStoreSetup(mockStore)
			handler := quotehandler.NewGetAllQuotesHandler(logger, mockStore)

			req := httptest.NewRequest(http.MethodGet, "/quotes", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req.WithContext(context.Background()))

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
			if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(tc.expectedBody) {
				t.Errorf("expected body %q, got %q", tc.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestGetRandomQuoteHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	originalErrorsIs := quotehandler.ErrorsIs
	defer func() { quotehandler.ErrorsIs = originalErrorsIs }()

	tests := []struct {
		name           string
		mockStoreSetup func(*MockQuoteStore)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "success",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetRandomQuoteFunc = func(ctx context.Context) (models.Quote, error) {
					return models.Quote{ID: 42, Text: "Be random", Author: "Universe"}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"success","data":{"id":42,"text":"Be random","author":"Universe"}}`,
		},
		{
			name: "quote not found",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetRandomQuoteFunc = func(ctx context.Context) (models.Quote, error) {
					return models.Quote{}, errTestQuoteNotFound
				}
				quotehandler.ErrorsIs = func(err, target error) bool {
					return err == errTestQuoteNotFound
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"status":"error","error":"No quotes found."}`,
		},
		{
			name: "storage error",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetRandomQuoteFunc = func(ctx context.Context) (models.Quote, error) {
					return models.Quote{}, errTestStorageInternal
				}
				quotehandler.ErrorsIs = errors.Is
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"status":"error","error":"Failed to retrieve random quote."}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockQuoteStore{}
			tc.mockStoreSetup(mockStore)

			handler := quotehandler.NewGetRandomQuoteHandler(logger, mockStore)
			req := httptest.NewRequest(http.MethodGet, "/quotes/random", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req.WithContext(context.Background()))

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
			if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(tc.expectedBody) {
				t.Errorf("expected body %q, got %q", tc.expectedBody, rr.Body.String())
			}
			quotehandler.ErrorsIs = originalErrorsIs
		})
	}
}

func TestGetQuotesByAuthorHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name           string
		authorQuery    string
		mockStoreSetup func(*MockQuoteStore)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:        "success found",
			authorQuery: "KnownAuthor",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetQuotesByAuthorFunc = func(ctx context.Context, author string) ([]models.Quote, error) {
					if author == "KnownAuthor" {
						return []models.Quote{{ID: 7, Text: "A quote", Author: "KnownAuthor"}}, nil
					}
					return []models.Quote{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"success","data":[{"id":7,"text":"A quote","author":"KnownAuthor"}]}`,
		},
		{
			name:        "success not found",
			authorQuery: "UnknownAuthor",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetQuotesByAuthorFunc = func(ctx context.Context, author string) ([]models.Quote, error) {
					return []models.Quote{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"success","data":[]}`,
		},
		{
			name:           "missing author query",
			authorQuery:    "",
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Author query parameter is required."}`,
		},
		{
			name:        "storage error",
			authorQuery: "AnyAuthor",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.GetQuotesByAuthorFunc = func(ctx context.Context, author string) ([]models.Quote, error) {
					return nil, errTestStorageInternal
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"status":"error","error":"Failed to retrieve quotes by author."}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockQuoteStore{}
			tc.mockStoreSetup(mockStore)
			handler := quotehandler.NewGetQuotesByAuthorHandler(logger, mockStore)

			req := httptest.NewRequest(http.MethodGet, "/quotes/search?author="+tc.authorQuery, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req.WithContext(context.Background()))

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
			if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(tc.expectedBody) {
				t.Errorf("expected body %q, got %q", tc.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestDeleteQuoteHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	originalErrorsIs := quotehandler.ErrorsIs
	defer func() { quotehandler.ErrorsIs = originalErrorsIs }()

	tests := []struct {
		name           string
		quoteID        string
		mockStoreSetup func(*MockQuoteStore)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "success",
			quoteID: "1",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.DeleteQuoteFunc = func(ctx context.Context, id int64) error {
					if id == 1 {
						return nil
					}
					return errors.New("unexpected id for delete")
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"success","message":"Quote deleted successfully."}`,
		},
		{
			name:           "id not in path",
			quoteID:        "",
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Quote ID is missing in path."}`,
		},
		{
			name:           "invalid id format",
			quoteID:        "abc",
			mockStoreSetup: func(ms *MockQuoteStore) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"status":"error","error":"Invalid quote ID format."}`,
		},
		{
			name:    "quote not found",
			quoteID: "999",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.DeleteQuoteFunc = func(ctx context.Context, id int64) error {
					return errTestQuoteNotFound
				}
				quotehandler.ErrorsIs = func(err, target error) bool { return err == errTestQuoteNotFound }
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"status":"error","error":"Quote not found."}`,
		},
		{
			name:    "storage error",
			quoteID: "777",
			mockStoreSetup: func(ms *MockQuoteStore) {
				ms.DeleteQuoteFunc = func(ctx context.Context, id int64) error {
					return errTestStorageInternal
				}
				quotehandler.ErrorsIs = errors.Is
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"status":"error","error":"Failed to delete quote."}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockQuoteStore{}
			tc.mockStoreSetup(mockStore)

			router := mux.NewRouter()
			handlerFunc := quotehandler.NewDeleteQuoteHandler(logger, mockStore)

			var reqPath string
			if tc.name == "id not in path" {
				router.HandleFunc("/quotes/delete_no_id", handlerFunc).Methods(http.MethodDelete)
				reqPath = "/quotes/delete_no_id"
			} else {
				router.HandleFunc("/quotes/{id}", handlerFunc).Methods(http.MethodDelete)
				reqPath = "/quotes/" + tc.quoteID
			}


			req := httptest.NewRequest(http.MethodDelete, reqPath, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req.WithContext(context.Background()))

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
			if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(tc.expectedBody) {
				t.Errorf("expected body %q, got %q", tc.expectedBody, rr.Body.String())
			}
			quotehandler.ErrorsIs = originalErrorsIs
		})
	}
}