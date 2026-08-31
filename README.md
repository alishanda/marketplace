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

## Каталог под нагрузкой

Горячий запрос витрины (`GET /api/catalog`):

```sql
SELECT sku, name, type, price, currency, image, stock
FROM products
WHERE stock > 0
ORDER BY type, sku
LIMIT $1 OFFSET $2;
```

На старте сидируется 2500+ SKU. Остаток на карточке — кэш числа свободных ключей. Источник правды — `inventory_keys`; `products.stock` пересчитывается в той же транзакции, что резерв ключа, поэтому витрина не врёт.

Индекс под этот запрос:

```sql
CREATE INDEX idx_products_storefront
  ON products (type, sku)
  INCLUDE (name, price, currency, image, stock)
  WHERE stock > 0;
```

- `WHERE stock > 0` — частичный: в индексе только то, что на витрине.
- `(type, sku)` совпадает с `ORDER BY`, отдельная сортировка не нужна.
- `INCLUDE` кладёт поля `SELECT` в листья — индекс покрывает запрос целиком.

План: `Limit → Index Only Scan using idx_products_storefront`. Postgres читает индекс и не ходит в кучу. После сида полезен `VACUUM ANALYZE products`, чтобы visibility map была свежей и IOS не срывался в таблицу.

На тысячах SKU `OFFSET` нормален. На миллионах лучше seek, без пропуска уже пройденных строк:

```sql
WHERE stock > 0
  AND (type, sku) > ($1, $2)
ORDER BY type, sku
LIMIT 48;
```

`$1/$2` — `type` и `sku` последней строки предыдущей страницы. Тот же индекс встаёт в нужный лист и читает пачку.

## Гонки и отказ поставщика

Контейнеры уже подняты (`docker compose up --build`). Скрипты бьют в `http://127.0.0.1:8180`.

**50 параллельных вебхуков** — разные `event_id`, один заказ, ровно одна выдача:

```bash
bash scripts/race.sh
```

Ожидание: `delivered` и один `delivery_code`. Повтор того же `event_id` можно проверить вторым `curl` на `/webhook/payment` с тем же телом — заказ не меняется.

**A недоступен, фолбэк на B:**

```bash
bash scripts/fallback.sh
```

Ставит A в 100% 5xx, B живой. Ожидание: `delivered`, `delivery_provider=B`, один ключ.

**Таймаут A (ключ уже выдан, ответ не дошёл):**

```bash
bash scripts/timeout.sh
```

A зависает после резерва, B тоже «мёртвый», чтобы не спрятать ошибку фолбэком. Ожидание: `delivered` через A, тот же `request_id` (`req_{order}-a`), второго ключа нет.

Доли ошибок руками: `POST /admin/providers/config` — `error_rate` (5xx без резерва) и `timeout_rate` (зависание после резерва). Автотесты тех же сценариев: `go test ./internal/app -count=1` при живом Postgres из compose.

## Решения и нагрузка

- Вебхук идемпотентен по `event_id` (`ON CONFLICT DO NOTHING`). Переход заказа в `paid` — `SELECT … FOR UPDATE`, в выдачу уходит только первый.
- Выдача с стабильным `request_id` (`req_{order}-a` / `-b`). Поставщик по нему всегда возвращает тот же код. Резерв ключа — `FOR UPDATE SKIP LOCKED`.
- Таймаут ≠ отказ: заглушка резервирует ключ и потом зависает. Ретраим A тем же `request_id`. На B идём только при явном 5xx / `out_of_stock`.
- Деньги: двойная запись `customer → escrow` при оплате, `escrow → revenue` при выдаче. Сверка: `GET /admin/reconcile`. Воркер добирает `paid` / `delivering` / `out_of_stock` / `delivery_failed`.
- Вебхук отвечает `200` сразу, выдача в фоне. Если вебхук пришёл раньше заказа — событие ждёт `POST /api/orders` с тем же `id`.

Под нагрузкой: несколько инстансов API за балансировщиком, выдача через outbox + очередь (`SKIP LOCKED` или брокер). Витрину можно читать с реплики. Ключи, `event_id` и проводки журнала остаются на master — идемпотентность держится уникальными ограничениями, не «сообщением один раз». Глубокий `OFFSET` витрины на миллионах меняется на seek `(type, sku) > last`.
