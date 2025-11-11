# microservices_gateway_users_orders

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
1. В корне репозитория запустите (Linux / macOS):

```bash
# в терминале (bash/zsh)
docker-compose up --build
```

2. После сборки и запуска сервисы будут доступны по умолчанию:
- API Gateway: http://localhost:8000
- Postgres: localhost:5432

Если хотите запустить в фоне:

```bash
docker-compose up --build -d
docker-compose logs -f api_gateway
```

Переменные окружения
--------------------
В файлах Dockerfile / compose используются:
- `DATABASE_DSN` — DSN для подключения к Postgres (пример в compose).
- `JWT_SECRET` — секрет для подписи JWT (в compose задан `dev-secret`).
- `PORT` — порт, на котором запускается сервис (по умолчанию 8000).

go mod tidy
Запуск отдельных сервисов локально (без Docker)
----------------------------------------------
Для разработки и тестов можно запускать сервисы напрямую через `go run`.
Примеры для Linux / macOS (bash):

```bash
cd service_users
# установить зависимости
go mod tidy
# экспортировать переменные окружения и запустить сервис
export DATABASE_DSN='host=localhost user=postgres password=postgres dbname=app_db port=5432 sslmode=disable'
export JWT_SECRET='dev-secret'
go run .
```

Если используете PowerShell на Windows, оставлен старый пример внизу.

Тесты
-----
- Интеграционные тесты написаны с использованием `net/http/httptest` и in-memory SQLite.
- В тестах используется pure-Go драйвер SQLite `github.com/glebarez/sqlite` — это позволяет запускать тесты в средах с CGO disabled (например, CI).

Запуск тестов для сервиса (Linux / macOS):

```bash
# service_users
cd service_users
go test ./...

# service_orders
cd ../service_orders
go test ./...
```

Пример для PowerShell (Windows) оставлен ниже, если потребуется.

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

Быстрые примеры (curl)
----------------------
# Регистрация
curl -X POST http://localhost:8000/v1/users/register -H 'Content-Type: application/json' -d '{"email":"u@example.com","password":"password","name":"User"}'

# Логин
curl -X POST http://localhost:8000/v1/users/login -H 'Content-Type: application/json' -d '{"email":"u@example.com","password":"password"}'

# Создать заказ (замените $TOKEN на полученный токен)
curl -X POST http://localhost:8000/v1/orders/ -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"items":"[]","total":10.5}'

OpenAPI и тесты
---------------
- OpenAPI: `docs/openapi-v1.yaml` — можно загрузить в Swagger UI или Postman.
- Postman collection: `docs/postman_collection.json`.

