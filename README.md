# Marketplace

## Запуск

Нужны Go 1.23+ и Docker.

```bash
make run
```

Поднимает Postgres и API. Витрина: http://127.0.0.1:8180  
Остановка: `Ctrl+C`, затем `make down`.

## Docker

```bash
docker compose up --build
```

Тоже http://127.0.0.1:8180  
Логи: `docker compose logs -f api`  
Остановка: `docker compose down`
