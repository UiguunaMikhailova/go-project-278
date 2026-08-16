# Сокращатель ссылок

### Hexlet tests and linter status:
[![Actions Status](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/UiguunaMikhailova/go-project-278/actions)
[![CI](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/UiguunaMikhailova/go-project-278/actions/workflows/ci.yml)

Веб-сервис на Go, который превращает длинный URL в короткий код и переадресует по нему.

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
