package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

const testBaseURL = "https://short.test"

// testDB заполняется в TestMain, если задан TEST_DATABASE_URL.
var testDB *sql.DB

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	_ = godotenv.Load()

	if databaseURL := os.Getenv("TEST_DATABASE_URL"); databaseURL != "" {
		database, err := openDB(databaseURL)
		if err != nil {
			log.Fatalf("failed to connect to test database: %v", err)
		}

		if err := goose.SetDialect("postgres"); err != nil {
			log.Fatalf("failed to set migration dialect: %v", err)
		}

		if err := goose.Up(database, "db/migrations"); err != nil {
			log.Fatalf("failed to apply migrations: %v", err)
		}

		testDB = database
	}

	os.Exit(m.Run())
}

// newTestRouter поднимает роутер на чистой таблице links.
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	if testDB == nil {
		t.Skip("TEST_DATABASE_URL is not set, database tests are skipped")
	}

	if _, err := testDB.Exec("TRUNCATE link_visits, links RESTART IDENTITY"); err != nil {
		t.Fatalf("failed to truncate table: %v", err)
	}

	return newRouter(NewLinksHandler(NewLinkService(testDB), testBaseURL))
}

func doRequest(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func decodeLink(t *testing.T, rec *httptest.ResponseRecorder) linkResponse {
	t.Helper()

	var link linkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}

	return link
}

// createLink создает ссылку через API и возвращает ее.
func createLink(t *testing.T, router *gin.Engine, originalURL, shortName string) linkResponse {
	t.Helper()

	body := `{"original_url":"` + originalURL + `","short_name":"` + shortName + `"}`

	rec := doRequest(t, router, http.MethodPost, "/api/links", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create link: code %d, body %s", rec.Code, rec.Body.String())
	}

	return decodeLink(t, rec)
}

// fieldErrors разбирает тело вида {"errors": {"<поле>": "<сообщение>"}}.
func fieldErrors(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	var body struct {
		Errors map[string]string `json:"errors"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}

	return body.Errors
}

func TestPing(t *testing.T) {
	router := newRouter(NewLinksHandler(nil, testBaseURL))

	rec := doRequest(t, router, http.MethodGet, "/ping", "")

	if rec.Code != http.StatusOK {
		t.Errorf("response code = %d, want %d", rec.Code, http.StatusOK)
	}

	if body := rec.Body.String(); body != "pong" {
		t.Errorf("response body = %q, want %q", body, "pong")
	}
}

func TestCreateLink(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/links",
		`{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("response code = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	link := decodeLink(t, rec)

	if link.ID == 0 {
		t.Error("id is empty")
	}

	if link.OriginalURL != "https://example.com/long-url" {
		t.Errorf("original_url = %q", link.OriginalURL)
	}

	if link.ShortName != "exmpl" {
		t.Errorf("short_name = %q, want %q", link.ShortName, "exmpl")
	}

	if want := testBaseURL + "/r/exmpl"; link.ShortURL != want {
		t.Errorf("short_url = %q, want %q", link.ShortURL, want)
	}
}

func TestCreateLinkGeneratesShortName(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/links",
		`{"original_url":"https://example.com/long-url"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("response code = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	link := decodeLink(t, rec)

	if link.ShortName == "" {
		t.Fatal("short name was not generated")
	}

	if !strings.HasSuffix(link.ShortURL, "/r/"+link.ShortName) {
		t.Errorf("short_url = %q does not contain short name %q", link.ShortURL, link.ShortName)
	}
}

func TestCreateLinkDuplicateShortName(t *testing.T) {
	router := newTestRouter(t)

	createLink(t, router, "https://example.com/first", "exmpl")

	rec := doRequest(t, router, http.MethodPost, "/api/links",
		`{"original_url":"https://example.com/second","short_name":"exmpl"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	if got := fieldErrors(t, rec)["short_name"]; got != "short name already in use" {
		t.Errorf("errors.short_name = %q, want %q", got, "short name already in use")
	}
}

func TestCreateLinkValidation(t *testing.T) {
	router := newTestRouter(t)

	cases := map[string]struct {
		body  string
		field string
	}{
		"without original_url": {`{"short_name":"exmpl"}`, "original_url"},
		"invalid url":          {`{"original_url":"not a url"}`, "original_url"},
		"short name too short": {`{"original_url":"https://example.com","short_name":"ab"}`, "short_name"},
		"short name too long": {
			`{"original_url":"https://example.com","short_name":"` + strings.Repeat("a", 33) + `"}`,
			"short_name",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(t, router, http.MethodPost, "/api/links", tc.body)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("response code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}

			if message := fieldErrors(t, rec)[tc.field]; message == "" {
				t.Errorf("no message for field %q, body %s", tc.field, rec.Body.String())
			}
		})
	}
}

func TestCreateLinkBrokenJSON(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/links", `{"original_url":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}

	if body.Error != "invalid request" {
		t.Errorf("error = %q, want %q", body.Error, "invalid request")
	}
}

func TestUpdateLinkValidation(t *testing.T) {
	router := newTestRouter(t)
	link := createLink(t, router, "https://example.com/long-url", "exmpl")

	rec := doRequest(t, router, http.MethodPut, "/api/links/"+itoa(link.ID),
		`{"original_url":"not a url","short_name":"ok"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	fields := fieldErrors(t, rec)

	for _, field := range []string{"original_url", "short_name"} {
		if fields[field] == "" {
			t.Errorf("no message for field %q, body %s", field, rec.Body.String())
		}
	}
}

func TestListLinks(t *testing.T) {
	router := newTestRouter(t)

	createLink(t, router, "https://example.com/first", "first")
	createLink(t, router, "https://example.com/second", "second")

	rec := doRequest(t, router, http.MethodGet, "/api/links", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusOK)
	}

	var links []linkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &links); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}

	if links[0].ShortName != "first" || links[1].ShortName != "second" {
		t.Errorf("unexpected order: %q, %q", links[0].ShortName, links[1].ShortName)
	}
}

func TestListLinksEmpty(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodGet, "/api/links", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusOK)
	}

	// пустой список должен быть [], а не null
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("response body = %q, want %q", body, "[]")
	}
}

func TestGetLink(t *testing.T) {
	router := newTestRouter(t)

	created := createLink(t, router, "https://example.com/long-url", "exmpl")

	rec := doRequest(t, router, http.MethodGet, "/api/links/"+itoa(created.ID), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusOK)
	}

	link := decodeLink(t, rec)

	if link.ID != created.ID || link.ShortName != "exmpl" {
		t.Errorf("got link %+v, want %+v", link, created)
	}
}

func TestGetLinkNotFound(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodGet, "/api/links/999999", "")

	if rec.Code != http.StatusNotFound {
		t.Errorf("response code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetLinkInvalidID(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodGet, "/api/links/abc", "")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("response code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateLink(t *testing.T) {
	router := newTestRouter(t)

	created := createLink(t, router, "https://example.com/long-url", "exmpl")

	rec := doRequest(t, router, http.MethodPut, "/api/links/"+itoa(created.ID),
		`{"original_url":"https://example.com/updated","short_name":"updtd"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	link := decodeLink(t, rec)

	if link.OriginalURL != "https://example.com/updated" {
		t.Errorf("original_url = %q", link.OriginalURL)
	}

	if link.ShortName != "updtd" {
		t.Errorf("short_name = %q, want %q", link.ShortName, "updtd")
	}

	if want := testBaseURL + "/r/updtd"; link.ShortURL != want {
		t.Errorf("short_url = %q, want %q", link.ShortURL, want)
	}
}

func TestUpdateLinkNotFound(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodPut, "/api/links/999999",
		`{"original_url":"https://example.com/updated","short_name":"updtd"}`)

	if rec.Code != http.StatusNotFound {
		t.Errorf("response code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateLinkDuplicateShortName(t *testing.T) {
	router := newTestRouter(t)

	createLink(t, router, "https://example.com/first", "first")
	second := createLink(t, router, "https://example.com/second", "second")

	rec := doRequest(t, router, http.MethodPut, "/api/links/"+itoa(second.ID),
		`{"original_url":"https://example.com/second","short_name":"first"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	if got := fieldErrors(t, rec)["short_name"]; got != "short name already in use" {
		t.Errorf("errors.short_name = %q, want %q", got, "short name already in use")
	}
}

func TestDeleteLink(t *testing.T) {
	router := newTestRouter(t)

	created := createLink(t, router, "https://example.com/long-url", "exmpl")

	rec := doRequest(t, router, http.MethodDelete, "/api/links/"+itoa(created.ID), "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if body := rec.Body.String(); body != "" {
		t.Errorf("response body = %q, want empty", body)
	}

	after := doRequest(t, router, http.MethodGet, "/api/links/"+itoa(created.ID), "")
	if after.Code != http.StatusNotFound {
		t.Errorf("code after delete = %d, want %d", after.Code, http.StatusNotFound)
	}
}

func TestDeleteLinkNotFound(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodDelete, "/api/links/999999", "")

	if rec.Code != http.StatusNotFound {
		t.Errorf("response code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

// seedLinks создает count ссылок с именами link-1, link-2 и так далее.
func seedLinks(t *testing.T, router *gin.Engine, count int) {
	t.Helper()

	for i := 1; i <= count; i++ {
		createLink(t, router, "https://example.com/"+strconv.Itoa(i), "link-"+strconv.Itoa(i))
	}
}

func listWithRange(t *testing.T, router *gin.Engine, rangeParam string) ([]linkResponse, string) {
	t.Helper()

	path := "/api/links"
	if rangeParam != "" {
		path += "?range=" + url.QueryEscape(rangeParam)
	}

	rec := doRequest(t, router, http.MethodGet, path, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var links []linkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &links); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return links, rec.Header().Get("Content-Range")
}

func TestListLinksRange(t *testing.T) {
	router := newTestRouter(t)
	seedLinks(t, router, 5)

	cases := []struct {
		name       string
		rangeParam string
		wantIDs    []int64
		wantHeader string
	}{
		{"first page", "[0,2]", []int64{1, 2}, "links 0-2/5"},
		{"second page", "[2,4]", []int64{3, 4}, "links 2-4/5"},
		{"tail of the list", "[4,10]", []int64{5}, "links 4-5/5"},
		{"space after comma", "[1, 3]", []int64{2, 3}, "links 1-3/5"},
		{"empty range", "[2,2]", nil, "links 2-2/5"},
		{"beyond the list", "[10,20]", nil, "links 5-5/5"},
		{"no range param", "", []int64{1, 2, 3, 4, 5}, "links 0-5/5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			links, header := listWithRange(t, router, tc.rangeParam)

			if header != tc.wantHeader {
				t.Errorf("Content-Range = %q, want %q", header, tc.wantHeader)
			}

			if len(links) != len(tc.wantIDs) {
				t.Fatalf("got %d links, want %d", len(links), len(tc.wantIDs))
			}

			for i, want := range tc.wantIDs {
				if links[i].ID != want {
					t.Errorf("links[%d].id = %d, want %d", i, links[i].ID, want)
				}
			}
		})
	}
}

func TestListLinksRangeInvalid(t *testing.T) {
	router := newTestRouter(t)
	seedLinks(t, router, 3)

	cases := map[string]string{
		"single element":   "[5]",
		"not a number":     "[a,b]",
		"not an array":     "5",
		"end before start": "[3,1]",
		"negative start":   "[-1,2]",
	}

	for name, rangeParam := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(t, router, http.MethodGet, "/api/links?range="+url.QueryEscape(rangeParam), "")

			if rec.Code != http.StatusBadRequest {
				t.Errorf("response code = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestListLinksAcceptRangesHeader(t *testing.T) {
	router := newTestRouter(t)

	rec := doRequest(t, router, http.MethodGet, "/api/links", "")

	if got := rec.Header().Get("Accept-Ranges"); got != "links" {
		t.Errorf("Accept-Ranges = %q, want %q", got, "links")
	}
}

func TestCORSPreflight(t *testing.T) {
	router := newRouter(NewLinksHandler(nil, testBaseURL))

	req := httptest.NewRequest(http.MethodOptions, "/api/links", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}

	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodDelete) {
		t.Errorf("Access-Control-Allow-Methods = %q, must contain %q", got, http.MethodDelete)
	}
}

func TestCORSExposesContentRange(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/links", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "Content-Range") {
		t.Errorf("Access-Control-Expose-Headers = %q, must contain %q", got, "Content-Range")
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	router := newRouter(NewLinksHandler(nil, testBaseURL))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://evil.example")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

// doRedirect делает запрос на короткую ссылку без автоматического перехода.
func doRedirect(t *testing.T, router *gin.Engine, code string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/r/"+code, nil)
	req.Header.Set("User-Agent", "curl/8.7.1")
	req.Header.Set("Referer", "https://example.com/from")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func decodeVisits(t *testing.T, rec *httptest.ResponseRecorder) []visitResponse {
	t.Helper()

	var visits []visitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &visits); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}

	return visits
}

func TestRedirect(t *testing.T) {
	router := newTestRouter(t)
	createLink(t, router, "https://example.com/long-url", "exmpl")

	rec := doRedirect(t, router, "exmpl")

	if rec.Code != http.StatusFound {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusFound)
	}

	if got := rec.Header().Get("Location"); got != "https://example.com/long-url" {
		t.Errorf("Location = %q, want %q", got, "https://example.com/long-url")
	}
}

func TestRedirectNotFound(t *testing.T) {
	router := newTestRouter(t)

	rec := doRedirect(t, router, "missing")

	if rec.Code != http.StatusNotFound {
		t.Errorf("response code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRedirectRecordsVisit(t *testing.T) {
	router := newTestRouter(t)
	link := createLink(t, router, "https://example.com/long-url", "exmpl")

	doRedirect(t, router, "exmpl")

	rec := doRequest(t, router, http.MethodGet, "/api/link_visits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusOK)
	}

	visits := decodeVisits(t, rec)

	if len(visits) != 1 {
		t.Fatalf("got %d visits, want 1", len(visits))
	}

	visit := visits[0]

	if visit.LinkID != link.ID {
		t.Errorf("link_id = %d, want %d", visit.LinkID, link.ID)
	}

	if visit.Status != http.StatusFound {
		t.Errorf("status = %d, want %d", visit.Status, http.StatusFound)
	}

	if visit.UserAgent != "curl/8.7.1" {
		t.Errorf("user_agent = %q, want %q", visit.UserAgent, "curl/8.7.1")
	}

	if visit.Referer != "https://example.com/from" {
		t.Errorf("referer = %q, want %q", visit.Referer, "https://example.com/from")
	}

	if visit.IP == "" {
		t.Error("ip is empty")
	}

	if visit.CreatedAt.IsZero() {
		t.Error("created_at is empty")
	}
}

func TestRedirectNotFoundDoesNotRecordVisit(t *testing.T) {
	router := newTestRouter(t)

	doRedirect(t, router, "missing")

	visits := decodeVisits(t, doRequest(t, router, http.MethodGet, "/api/link_visits", ""))

	if len(visits) != 0 {
		t.Errorf("got %d visits, want 0", len(visits))
	}
}

func TestListVisitsRange(t *testing.T) {
	router := newTestRouter(t)
	createLink(t, router, "https://example.com/long-url", "exmpl")

	for i := 0; i < 3; i++ {
		doRedirect(t, router, "exmpl")
	}

	rec := doRequest(t, router, http.MethodGet, "/api/link_visits?range="+url.QueryEscape("[1,3]"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Range"); got != "link_visits 1-3/3" {
		t.Errorf("Content-Range = %q, want %q", got, "link_visits 1-3/3")
	}

	visits := decodeVisits(t, rec)

	if len(visits) != 2 {
		t.Fatalf("got %d visits, want 2", len(visits))
	}

	if visits[0].ID != 2 || visits[1].ID != 3 {
		t.Errorf("got ids %d, %d, want 2, 3", visits[0].ID, visits[1].ID)
	}
}

func TestListVisitsRangeFromHeader(t *testing.T) {
	router := newTestRouter(t)
	createLink(t, router, "https://example.com/long-url", "exmpl")
	doRedirect(t, router, "exmpl")
	doRedirect(t, router, "exmpl")

	req := httptest.NewRequest(http.MethodGet, "/api/link_visits", nil)
	req.Header.Set("Range", "[0,1]")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Range"); got != "link_visits 0-1/2" {
		t.Errorf("Content-Range = %q, want %q", got, "link_visits 0-1/2")
	}

	if visits := decodeVisits(t, rec); len(visits) != 1 {
		t.Errorf("got %d visits, want 1", len(visits))
	}
}

func TestDeleteLinkRemovesVisits(t *testing.T) {
	router := newTestRouter(t)
	link := createLink(t, router, "https://example.com/long-url", "exmpl")
	doRedirect(t, router, "exmpl")

	rec := doRequest(t, router, http.MethodDelete, "/api/links/"+itoa(link.ID), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusNoContent)
	}

	visits := decodeVisits(t, doRequest(t, router, http.MethodGet, "/api/link_visits", ""))

	if len(visits) != 0 {
		t.Errorf("got %d visits after link delete, want 0", len(visits))
	}
}
