# Сокращатель ссылок

### Hexlet tests and linter status:
[![Actions Status](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/UiguunaMikhailova/go-project-278/actions)
[![CI](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/ci.yml)

Веб-сервис на Go, который превращает длинный URL в короткий код и переадресует по нему.

## Переменные окружения

Локально они читаются из файла `.env`:

```bash
cp .env.example .env   # затем подставить свои значения
```

`PORT` - порт приложения, по умолчанию `8080`
`SENTRY_DSN` - DSN проекта в Bugsink/Sentry
`APP_ENV` - окружение в событиях мониторинга, например `production`
`DATABASE_URL` - строка подключения к PostgreSQL

## Разработка

```bash
make run     # запустить приложение на http://localhost:8080
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

## Мониторинг ошибок

Ошибки и паники отправляются в Bugsink через sentry-go. Проверить связку:

```bash
make run                                     # DSN берется из .env
curl -i http://localhost:8080/debug/sentry   # 500, событие уходит в Bugsink
```

## Docker

```bash
docker build --platform linux/amd64 -t go-project-278 .
docker run --rm --platform linux/amd64 -p 8080:8080 -e PORT=8080 go-project-278
```
