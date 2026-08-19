package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/UiguunaMikhailova/go-project-278/internal/db"
)

// Ошибки бизнес-логики, по ним обработчики выбирают код ответа.
var (
	ErrLinkNotFound   = errors.New("link not found")
	ErrShortNameTaken = errors.New("short name is already taken")
)

// уникальное нарушение ограничения в PostgreSQL
const uniqueViolationCode = "23505"

const (
	shortNameAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	shortNameLength   = 6
	// сколько раз пробуем сгенерировать имя, если оно уже занято
	shortNameAttempts = 5
)

// LinkService содержит бизнес-логику работы с короткими ссылками.
type LinkService struct {
	queries *db.Queries
}

func NewLinkService(database *sql.DB) *LinkService {
	return &LinkService{queries: db.New(database)}
}

func (s *LinkService) Get(ctx context.Context, id int64) (db.Link, error) {
	link, err := s.queries.GetLink(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Link{}, ErrLinkNotFound
	}

	return link, err
}

// Create сохраняет ссылку. Если короткое имя не задано, генерирует уникальное.
func (s *LinkService) Create(ctx context.Context, originalURL, shortName string) (db.Link, error) {
	if shortName != "" {
		link, err := s.queries.CreateLink(ctx, db.CreateLinkParams{
			OriginalUrl: originalURL,
			ShortName:   shortName,
		})
		if isUniqueViolation(err) {
			return db.Link{}, ErrShortNameTaken
		}

		return link, err
	}

	// имя генерируем случайно, поэтому редкие совпадения разруливаем повтором
	for attempt := 0; attempt < shortNameAttempts; attempt++ {
		generated, err := generateShortName()
		if err != nil {
			return db.Link{}, err
		}

		link, err := s.queries.CreateLink(ctx, db.CreateLinkParams{
			OriginalUrl: originalURL,
			ShortName:   generated,
		})
		if isUniqueViolation(err) {
			continue
		}

		return link, err
	}

	return db.Link{}, errors.New("failed to pick a free short name")
}

func (s *LinkService) Update(ctx context.Context, id int64, originalURL, shortName string) (db.Link, error) {
	link, err := s.queries.UpdateLink(ctx, db.UpdateLinkParams{
		ID:          id,
		OriginalUrl: originalURL,
		ShortName:   shortName,
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return db.Link{}, ErrLinkNotFound
	case isUniqueViolation(err):
		return db.Link{}, ErrShortNameTaken
	}

	return link, err
}

func (s *LinkService) Delete(ctx context.Context, id int64) error {
	affected, err := s.queries.DeleteLink(ctx, id)
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrLinkNotFound
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

func generateShortName() (string, error) {
	bytes := make([]byte, shortNameLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate short name: %w", err)
	}

	name := make([]byte, shortNameLength)
	for i, b := range bytes {
		name[i] = shortNameAlphabet[int(b)%len(shortNameAlphabet)]
	}

	return string(name), nil
}

// ListPage возвращает страницу ссылок: limit записей, пропустив offset первых.
func (s *LinkService) ListPage(ctx context.Context, offset, limit int32) ([]db.Link, error) {
	return s.queries.ListLinksPage(ctx, db.ListLinksPageParams{Limit: limit, Offset: offset})
}

// Count возвращает общее количество ссылок, нужно для заголовка Content-Range.
func (s *LinkService) Count(ctx context.Context) (int64, error) {
	return s.queries.CountLinks(ctx)
}

// GetByShortName ищет ссылку по короткому имени, нужно для редиректа.
func (s *LinkService) GetByShortName(ctx context.Context, shortName string) (db.Link, error) {
	link, err := s.queries.GetLinkByShortName(ctx, shortName)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Link{}, ErrLinkNotFound
	}

	return link, err
}

// RecordVisit сохраняет факт перехода по короткой ссылке.
func (s *LinkService) RecordVisit(ctx context.Context, visit db.CreateLinkVisitParams) (db.LinkVisit, error) {
	return s.queries.CreateLinkVisit(ctx, visit)
}

// ListVisitsPage возвращает страницу посещений: limit записей, пропустив offset первых.
func (s *LinkService) ListVisitsPage(ctx context.Context, offset, limit int32) ([]db.LinkVisit, error) {
	return s.queries.ListLinkVisitsPage(ctx, db.ListLinkVisitsPageParams{Limit: limit, Offset: offset})
}

func (s *LinkService) CountVisits(ctx context.Context) (int64, error) {
	return s.queries.CountLinkVisits(ctx)
}
