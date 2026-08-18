package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/UiguunaMikhailova/go-project-278/internal/db"
)

// linkRequest - тело запроса на создание и обновление ссылки.
type linkRequest struct {
	OriginalURL string `json:"original_url" binding:"required,url"`
	ShortName   string `json:"short_name"`
}

// linkResponse - представление ссылки в ответах API.
type linkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

// единица измерения диапазонов в заголовках Content-Range и Accept-Ranges
const rangeUnit = "links"

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

	start, end, err := parseRange(c.Query("range"), total)
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

	c.Header("Accept-Ranges", rangeUnit)
	c.Header("Content-Range", fmt.Sprintf("%s %d-%d/%d", rangeUnit, start, end, total))
	c.JSON(http.StatusOK, responses)
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
