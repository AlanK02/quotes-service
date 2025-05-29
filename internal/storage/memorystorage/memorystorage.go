package memorystorage

import (
	"context"
	"math/rand"
	"sync"

	"quotes-service/internal/models"
	"quotes-service/internal/storage"
)

type Storage struct {
	mu         sync.RWMutex
	quotes     map[int64]models.Quote
	quotesList []models.Quote
	nextID     int64
}

func New() (*Storage, error) {
	return &Storage{
		quotes:     make(map[int64]models.Quote),
		quotesList: make([]models.Quote, 0),
		nextID:     1,
	}, nil
}

func (s *Storage) AddQuote(ctx context.Context, text string, author string) (int64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err() 
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	quote := models.Quote{
		ID:     id,
		Text:   text,
		Author: author,
	}
	s.quotes[id] = quote
	s.quotesList = append(s.quotesList, quote)

	return id, nil
}

func (s *Storage) GetAllQuotes(ctx context.Context) ([]models.Quote, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	listCopy := make([]models.Quote, len(s.quotesList))
	copy(listCopy, s.quotesList)
	return listCopy, nil
}

func (s *Storage) GetRandomQuote(ctx context.Context) (models.Quote, error) {
	select {
	case <-ctx.Done():
		return models.Quote{}, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.quotesList) == 0 {
		return models.Quote{}, storage.ErrQuoteNotFound
	}
	randomIndex := rand.Intn(len(s.quotesList))
	return s.quotesList[randomIndex], nil
}

func (s *Storage) GetQuotesByAuthor(ctx context.Context, authorFilter string) ([]models.Quote, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.Quote
	for _, q := range s.quotesList {
		if q.Author == authorFilter {
			result = append(result, q)
		}
	}

	if result == nil {
		return []models.Quote{}, nil
	}
	return result, nil
}

func (s *Storage) DeleteQuote(ctx context.Context, id int64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.quotes[id]
	if !exists {
		return storage.ErrQuoteNotFound
	}

	delete(s.quotes, id)

	var newList []models.Quote
	if len(s.quotesList) > 0 {
		newList = make([]models.Quote, 0, len(s.quotesList)-1)
	} else {
		newList = make([]models.Quote, 0)
	}


	for _, q := range s.quotesList {
		if q.ID != id {
			newList = append(newList, q)
		}
	}
	s.quotesList = newList

	return nil
}

func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotes = make(map[int64]models.Quote)
	s.quotesList = []models.Quote{}
	s.nextID = 1
	return nil
}