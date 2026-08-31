# Marketplace

Магазин цифровых товаров: каталог, заказ, вебхук оплаты, автовыдача ключа. Два поставщика-заглушки, повтор вебхуков без двойной выдачи, сверка и витрина.

## Запуск

```bash
docker compose up --build
```

- Витрина: http://127.0.0.1:8180
- Swagger: http://127.0.0.1:8180/swagger
- OpenAPI: http://127.0.0.1:8180/openapi.yaml

Логи: `docker compose logs -f api`  
Остановка: `docker compose down`

## API

Спека и Try it out — в Swagger.

- `GET /api/catalog` — витрина (`?featured=1` — товары из задания)
- `GET /api/catalog/{sku}` — карточка
- `POST /api/orders` — создать заказ (`{"sku":"STEAM-TOPUP-500"}`)
- `GET /api/orders/{id}` — статус и ключ после выдачи
- `POST /api/orders/{id}/pay` — эмуляция оплаты
- `POST /webhook/payment` — вебхук платежки (`event_id`, `order_id`, `status`)
- `GET /admin/reconcile` — оплачен/не выдан, баланс журнала

Заказ: `created → paid → delivering → delivered`. Если кода нет — `out_of_stock` или `delivery_failed`, можно повторить выдачу.
