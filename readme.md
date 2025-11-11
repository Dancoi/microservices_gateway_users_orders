# Micro Task Template — Go PoC

Короткое описание
------------------
Это PoC миграции небольшого микросервисного проекта на Go.
Проект содержит три сервиса:
- `service_users` — управление пользователями (регистрация, вход, профиль)
- `service_orders` — управление заказами (создание, получение, список, смена статуса)
- `api_gateway` — проксирование запросов к сервисам, базовая авторизация и rate-limit

Окружение
---------
В проекте используется Docker Compose с PostgreSQL как runtime DB.
Файл `docker-compose.yml` содержит конфигурацию сервисов и сети.

Ключевые настройки в `docker-compose.yml` (важное):
- Postgres: образ `postgres:15-alpine`, порт `5432:5432`, БД `app_db`, пользователь `postgres`/`postgres`.
- `service_users` и `service_orders` подключаются к Postgres через переменную `DATABASE_DSN`.
- `api_gateway` слушает порт `8000` на хосте (проброшен `8000:8000`).

Запуск локально (Docker)
------------------------
1. В корне репозитория запустите:

```powershell
# в PowerShell
docker-compose up --build
```

2. После сборки и запуска сервисы будут доступны по умолчанию:
- API Gateway: http://localhost:8000
- Postgres: localhost:5432

Переменные окружения
--------------------
В файлах Dockerfile / compose используются:
- `DATABASE_DSN` — DSN для подключения к Postgres (пример в compose).
- `JWT_SECRET` — секрет для подписи JWT (в compose задан `dev-secret`).
- `PORT` — порт, на котором запускается сервис (по умолчанию 8000).

Запуск отдельных сервисов локально (без Docker)
----------------------------------------------
Для разработки и тестов можно запускать сервисы напрямую через `go run`.
Пример:

```powershell
Set-Location -Path .\service_users
# установить зависимости
go mod tidy
# запустить сервис
$env:DATABASE_DSN = 'host=localhost user=postgres password=postgres dbname=app_db port=5432 sslmode=disable'
$env:JWT_SECRET = 'dev-secret'
go run .
```

Тесты
-----
- Интеграционные тесты написаны с использованием `net/http/httptest` и in-memory SQLite.
- В тестах используется pure-Go драйвер SQLite `github.com/glebarez/sqlite` — это позволяет запускать тесты в средах с CGO disabled (например, CI).

Запуск тестов для сервиса (пример для PowerShell):

```powershell
# service_users
Set-Location -Path .\service_users
go test ./...

# service_orders
Set-Location -Path .\service_orders
go test ./...
```

CI (рекомендация)
-----------------
Рекомендуется добавить GitHub Actions workflow, который выполняет:
- `go mod tidy`
- `go test ./...` для каждого сервиса
- `golangci-lint run` (опционально)

Пример заметки: используйте `CGO_ENABLED=0` и ensure tests use `github.com/glebarez/sqlite` for in-memory DB.

Особенности и примечания
------------------------
- Для runtime используется PostgreSQL (в `docker-compose.yml`).
- Для тестов используется in-memory SQLite (pure-Go driver) — быстрее и не требует внешней БД в CI.
- JWT секрет и другие конфигурации задаются через переменные окружения.
- OpenAPI спецификация находится в `docs/openapi-v1.yaml`.
- Postman коллекция находится в `docs/postman_collection.json`.

Дальше (рекомендации)
--------------------
- Добавить CI workflow (я могу создать `.github/workflows/ci.yml`).
- Добавить README секции: Endpoints (пример запросов), Swagger / how to run API docs.
- Интегрировать структурное логирование (zerolog) и OpenTelemetry для трассировки.

Связь
-----
Если нужно — могу автоматически создать:
- README с примерами запросов (curl/PowerShell) и HTTP response samples;
- GitHub Actions workflow для автоматизации тестов;
- Дополнительную документацию для деплоя.
