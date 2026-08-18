# Сокращатель ссылок

### Hexlet tests and linter status:
[![Actions Status](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/UiguunaMikhailova/go-project-278/actions)
[![CI](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/ci.yml)

Веб-сервис на Go, который превращает длинный URL в короткий код и переадресует по нему.

Развернутое приложение: https://go-project-278-ptjx.onrender.com

## Переменные окружения

Локально они читаются из файла `.env`:

```bash
cp .env.example .env   # затем подставить свои значения
```

`PORT` - порт приложения, по умолчанию `8080`
`SENTRY_DSN` - DSN проекта в Bugsink/Sentry
`APP_ENV` - окружение в событиях мониторинга, например `production`
`DATABASE_URL` - строка подключения к PostgreSQL
`TEST_DATABASE_URL` - строка подключения к базе для тестов
`BASE_URL` - адрес сервиса
`CORS_ORIGINS` - разрешенные источники для CORS через запятую, по умолчанию `http://localhost:5173`

## Разработка

```bash
make db-up   # поднять PostgreSQL в docker
make migrate # применить миграции goose
make run     # запустить приложение на http://localhost:8080
make dev     # запустить API и веб-интерфейс вместе
make test    # тесты
make lint    # golangci-lint
make check   # тесты + линтер
make build   # собрать бинарник в bin/app
```

Проверить, что сервис отвечает:

```bash
curl http://localhost:8080/ping
# pong
```

## API

`GET` `/api/links` - 200 список ссылок
`POST` `/api/links` - 201 созданная ссылка
`GET` `/api/links/:id` - 200 ссылка - 404 если не найдена
`PUT` `/api/links/:id` - 200 обновленная ссылка - 404 если не найдена 
`DELETE` `/api/links/:id` - 204 без тела - 404 если не найдена

Поле `short_name` можно не передавать - сервер сгенерирует уникальное имя сам.
Если имя занято, ответ будет 409 Conflict.

### Пагинация

`GET /api/links?range=[0,10]` - первые 10 ссылок, `?range=[5,10]` - пять ссылок начиная с шестой.
Конец диапазона не включается, границы обрезаются по количеству записей.
Без параметра возвращается весь список.

В ответе приходит заголовок `Content-Range: links 0-10/42`, где последнее число - всего записей.
Некорректный диапазон - 400 Bad Request.

```bash
curl -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{"original_url": "https://example.com/long-url", "short_name": "exmpl"}'
```

## Веб-интерфейс

Интерфейс поставляется npm-пакетом `@hexlet/project-url-shortener-frontend`.

```bash
npm install          # один раз, поставит фронтенд и concurrently
make dev             # API на :8080 и интерфейс на http://localhost:5173
```

Фронтенд проксирует свои запросы `/api` на бэкенд, адрес которого можно
переопределить переменной `API_URL`.

## Мониторинг ошибок

Ошибки и паники отправляются в Bugsink через sentry-go. Проверить связку:

```bash
make run                                     # DSN берется из .env
curl -i http://localhost:8080/debug/sentry   # 500, событие уходит в Bugsink
```

## Docker

В образе фронтенд и бэкенд работают вместе: Caddy слушает 80, раздает статику
из `/app/public` и проксирует остальные запросы в приложение на 8080.

```bash
docker build --platform linux/amd64 -t go-project-278 .
docker run --rm --platform linux/amd64 -p 8090:80 \
  -e DATABASE_URL="postgres://postgres:password@host.docker.internal:5432/appdb?sslmode=disable" \
  go-project-278
# интерфейс на http://localhost:8090
```
