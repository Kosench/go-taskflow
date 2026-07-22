# go-taskflow

API сохраняет задачу в PostgreSQL и публикует её в Kafka. Worker получает сообщения через consumer group, атомарно захватывает задачи в БД и сохраняет результат. PostgreSQL Poller используется для scheduled tasks, retry и как fallback при ошибке публикации в Kafka.

## Стек

- Go;
- Apache Kafka и Sarama;
- PostgreSQL и `sqlx`;
- Docker Compose;
- `zerolog`;
- `golang-migrate`.

## Архитектура

```text
HTTP API → PostgreSQL → Kafka Producer
                         ↓
                    Kafka topics
                         ↓
Kafka Consumer ──────────┐
                        ├→ Processor → PostgreSQL
PostgreSQL Poller ───────┘
```


## Запуск

Требуются Go, Docker и Docker Compose.

```bash
# PostgreSQL, Kafka, Kafka UI, Prometheus и Grafana
make infra-up

# Миграции
make migrate-up

# API: порт 8081 используется из-за Kafka UI на 8080
TASKQUEUE_SERVER_HTTP_PORT=8081 go run ./cmd/api

# В другом терминале
go run ./cmd/worker
```

Создание задачи:

```bash
curl -X POST http://localhost:8081/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "webhook",
    "priority": 1,
    "payload": {"url": "https://example.com/hook"}
  }'
```

## Основные endpoints

```text
POST /api/v1/tasks
GET  /api/v1/tasks
GET  /api/v1/tasks/{id}
POST /api/v1/tasks/{id}/retry
POST /api/v1/tasks/{id}/cancel
GET  /api/v1/tasks/stats
GET  /health/live
GET  /health/ready
```

## Проверка

```bash
go test -race -short ./...
go vet ./...
go build ./...
```

