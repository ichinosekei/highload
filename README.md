# Highload Food Delivery Service PoC

Прототип высоконагруженного ядра сервиса доставки еды с реализацией паттернов отказоустойчивости.

---

## 🚀 Быстрый старт

### 1. Запуск системы
Поднимите весь стек (микросервисы + инфраструктура) одной командой:
```bash
docker compose up -d
```

### 2. Проверка доступности (Health Check)
Убедитесь, что все компоненты системы живы и здоровы:

*   **API Gateway (Traefik)**: `curl -i http://localhost/health` (Ожидается: `200 OK`)
*   **Catalog Service**: `curl -i http://localhost:8080/health` (Ожидается: `200 OK`)
*   **Order Service**: `curl -i http://localhost:8082/health` (Ожидается: `200 OK`)
*   **Payment Service**: `curl -i http://localhost:8083/health` (Ожидается: `200 OK`)
*   **Notification Service**: `curl -i http://localhost:8081/health` (Ожидается: `200 OK`)

---

## 🔍 Ручная проверка API

### 1. Поиск ресторанов
```bash
curl -X GET "http://localhost/api/v1/search?q=pizza&cuisine=Italian&limit=5"
```
*   **Ожидается**: `200 OK` и JSON со списком ресторанов (Meilisearch).

### 2. Получение меню ресторана
```bash
curl -X GET "http://localhost/api/v1/catalog/restaurants/0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901/menu"
```
*   **Ожидается**: `200 OK` и JSON с деталями ресторана и списком блюд.

### 3. Создание заказа
```bash
curl -i -X POST http://localhost/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: 12345678-1234-1234-1234-1234567890ab" \
  -d '{
    "restaurant_id": "0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901",
    "items": [{"menu_item_id": "0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001", "quantity": 2}],
    "delivery_address": {"lat": 55.75, "lon": 37.62, "address_text": "ул. Тестовая, 1"}
  }'
```
*   **Ожидается**: `201 Created` и JSON с данными созданного заказа.
*   *Примечание*: При повторном вызове с тем же ключом идемпотентности вернется тот же заказ.

---

## 📈 Нагрузочное тестирование

Мы используем **k6** для проверки системы под нагрузкой (100 RPS read / 30 RPS write).

### Запуск замера Baseline (Iteration 0):

**Вариант А (mise):**
```bash
mise run measure-baseline
```

**Вариант Б (Docker):**
```bash
docker run --rm -i --network=host grafana/k6 run - <k6/stress.js
```

👉 **[Подробный лог оптимизации и результаты тестов (docs/optimization-log.md)](docs/optimization-log.md)**

---

## 🏗 Архитектура и Масштабирование

В проекте реализовано 8 ключевых паттернов устойчивости и проектирования.
👉 **[Подробное обоснование паттернов со ссылками на код (docs/architecture-patterns.md)](docs/architecture-patterns.md)**

### Горизонтальное масштабирование (Scale x2)
Для запуска системы с несколькими репликами сервисов (2 реплики для Catalog и Order) выполните:
```bash
docker compose -f docker-compose.yaml -f docker-compose.scaled.yml up -d
```

### Ресурсы
Лимиты ресурсов строго ограничены в `docker-compose.yaml` (суммарно под 2 vCPU / 8 GB RAM), что соответствует требованиям к развертыванию на VM.
