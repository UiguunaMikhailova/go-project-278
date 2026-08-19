package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/UiguunaMikhailova/go-project-278/internal/db"
)

// linkRequest - тело запроса на создание и обновление ссылки.
type linkRequest struct {
	OriginalURL string `json:"original_url" binding:"required,url"`
	ShortName   string `json:"short_name"`
}

// visitResponse - представление посещения в ответах API.
type visitResponse struct {
	ID        int64     `json:"id"`
	LinkID    int64     `json:"link_id"`
	CreatedAt time.Time `json:"created_at"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	Status    int32     `json:"status"`
}

// linkResponse - представление ссылки в ответах API.
type linkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

// единицы измерения диапазонов в заголовках Content-Range и Accept-Ranges
const (
	linksRangeUnit  = "links"
	visitsRangeUnit = "link_visits"
)

// код перенаправления с короткой ссылки на исходный адрес
const redirectStatus = http.StatusFound

// LinksHandler обслуживает маршруты /api/links.
type LinksHandler struct {
	service *LinkService
	baseURL string
}

func NewLinksHandler(service *LinkService, baseURL string) *LinksHandler {
	return &LinksHandler{service: service, baseURL: baseURL}
}

// newResponse собирает короткий адрес из базового адреса сервиса и короткого имени.
func (h *LinksHandler) newResponse(link db.Link) linkResponse {
	return linkResponse{
		ID:          link.ID,
		OriginalURL: link.OriginalUrl,
		ShortName:   link.ShortName,
		ShortURL:    h.baseURL + "/r/" + link.ShortName,
	}
}

func (h *LinksHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	total, err := h.service.Count(ctx)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	start, end, err := parseRange(requestedRange(c), total)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	links, err := h.service.ListPage(ctx, int32(start), int32(end-start))
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	responses := make([]linkResponse, 0, len(links))
	for _, link := range links {
		responses = append(responses, h.newResponse(link))
	}

	writeRangeHeaders(c, linksRangeUnit, start, end, total)
	c.JSON(http.StatusOK, responses)
}

// ListVisits отдает список посещений с той же пагинацией, что и список ссылок.
func (h *LinksHandler) ListVisits(c *gin.Context) {
	ctx := c.Request.Context()

	total, err := h.service.CountVisits(ctx)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	start, end, err := parseRange(requestedRange(c), total)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	visits, err := h.service.ListVisitsPage(ctx, int32(start), int32(end-start))
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	responses := make([]visitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, visitResponse{
			ID:        visit.ID,
			LinkID:    visit.LinkID,
			CreatedAt: visit.CreatedAt,
			IP:        visit.Ip,
			UserAgent: visit.UserAgent,
			Referer:   visit.Referer,
			Status:    visit.Status,
		})
	}

	writeRangeHeaders(c, visitsRangeUnit, start, end, total)
	c.JSON(http.StatusOK, responses)
}

// Redirect перенаправляет с короткой ссылки на исходный адрес и записывает посещение.
func (h *LinksHandler) Redirect(c *gin.Context) {
	ctx := c.Request.Context()

	link, err := h.service.GetByShortName(ctx, c.Param("code"))
	if err != nil {
		respondServiceError(c, err)
		return
	}

	// статистика не должна мешать переходу, поэтому ошибку записи только логируем
	_, err = h.service.RecordVisit(ctx, db.CreateLinkVisitParams{
		LinkID:    link.ID,
		Ip:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Referer:   c.Request.Referer(),
		Status:    redirectStatus,
	})
	if err != nil {
		log.Printf("failed to record visit for link %d: %v", link.ID, err)
	}

	c.Redirect(redirectStatus, link.OriginalUrl)
}

func writeRangeHeaders(c *gin.Context, unit string, start, end, total int64) {
	c.Header("Accept-Ranges", unit)
	c.Header("Content-Range", fmt.Sprintf("%s %d-%d/%d", unit, start, end, total))
}

// requestedRange читает диапазон из query-параметра, а если его нет - из заголовка Range.
func requestedRange(c *gin.Context) string {
	if raw := c.Query("range"); raw != "" {
		return raw
	}

	return c.GetHeader("Range")
}

func parseRange(raw string, total int64) (start, end int64, err error) {
	if raw == "" {
		return 0, total, nil
	}

	var bounds []int64
	if err := json.Unmarshal([]byte(raw), &bounds); err != nil || len(bounds) != 2 {
		return 0, 0, errors.New("range must look like [start,end]")
	}

	start, end = bounds[0], bounds[1]

	if start < 0 || end < start {
		return 0, 0, errors.New("range bounds must be non-negative and ordered")
	}

	if end > total {
		end = total
	}

	if start > end {
		start = end
	}

	return start, end, nil
}

func (h *LinksHandler) Create(c *gin.Context) {
	var request linkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	link, err := h.service.Create(c.Request.Context(), request.OriginalURL, request.ShortName)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.newResponse(link))
}

func (h *LinksHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	link, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.newResponse(link))
}

func (h *LinksHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var request linkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	link, err := h.service.Update(c.Request.Context(), id, request.OriginalURL, request.ShortName)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.newResponse(link))
}

func (h *LinksHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		respondServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// parseID читает идентификатор из пути; при ошибке сам отвечает клиенту.
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, errors.New("id must be a number"))
		return 0, false
	}

	return id, true
}

// respondServiceError переводит ошибки бизнес-логики в коды ответа.
func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrLinkNotFound):
		respondError(c, http.StatusNotFound, err)
	case errors.Is(err, ErrShortNameTaken):
		respondError(c, http.StatusConflict, err)
	default:
		respondError(c, http.StatusInternalServerError, err)
	}
}

func respondError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}
