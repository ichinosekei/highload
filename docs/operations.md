# Deployment Document: FoodDelivery Platform

## Часть 1: Deployment-документ

### 1.1. Описание развёртывания

### Назначение
FoodDelivery Platform — это высоконагруженная система доставки еды, объединяющая клиентов, рестораны и курьеров. Система обеспечивает поиск по каталогу, оформление и оплату заказов, отслеживание доставки в реальном времени и систему уведомлений.

### Нагрузка
Профиль нагрузки характеризуется ярко выраженными «обеденными» и «вечерними» пиками (коэффициент пика — до 6).
*   **MAU:** 12 млн.
*   **DAU:** 3 млн.
*   **Пиковый онлайн:** ~81 000 одновременных пользователей.
*   **Заказов в день:** **1 125 000** (реалистичный сценарий для зрелой системы).

| Операция | Avg RPS | Peak RPS | SLO p99 |
| :--- | :--- | :--- | :--- |
| Поиск (Catalog/Search) | 1 625 | 10 000 | < 300 ms |
| Просмотр меню | 1 084 | 6 000 | < 400 ms |
| Создание заказа | 136 | 500 | < 500 ms |
| Подтверждение оплаты | 108 | 200 | < 2500 ms (incl. PSP) |
| Статусы/Трекинг | 1 500 | 2 000 | < 200 ms |

### Топология и сетевые зоны
Архитектура развернута в Yandex Cloud в одном регионе (`ru-central1`) и распределена по 3 зонам доступности (AZ-a, b, c).

| Зона (Subnet) | CIDR | Назначение | Что внутри |
| :--- | :--- | :--- | :--- |
| **Public DMZ** | `10.128.0.0/16` | Входной трафик | ALB, Traefik Ingress, NAT Gateways |
| **Private App-a** | `10.128.1.0/24` | Backend AZ-a | Catalog, Order, Payment (Replica 1) |
| **Private App-b** | `10.129.1.0/24` | Backend AZ-b | Catalog, Order, Payment (Replica 2) |
| **Private App-c** | `10.130.1.0/24` | Backend AZ-c | Catalog, Order, Payment (Replica 3) |
| **Private Data** | `10.128.2.0/24, 10.129.2.0/24, 10.130.2.0/24` | Хранилища | Managed PG, Valkey Cluster, NATS nodes |

### Связи и протоколы (Service Connections)

| Источник → Назначение | Протокол / Порт | Назначение |
| :--- | :--- | :--- |
| Client → ALB | HTTPS :443 | Вход в систему (Public VIP) |
| ALB → Traefik | HTTP :80 | Проксирование внутрь DMZ |
| Traefik → Catalog | HTTP :8080 | Поиск и меню |
| Traefik → Notification | HTTP :8081 | Система уведомлений |
| Traefik → Order | HTTP :8082 | Создание и статусы заказов |
| Traefik → Payment | HTTP :8083 | Инициация платежей |
| Microservices → Valkey | TCP :6379 | Кэширование и корзины |
| Catalog → PostgreSQL | TCP :5432 | SQL запросы (меню, рестораны) |
| Catalog → Meilisearch | HTTP :7700 | Полнотекстовый поиск |
| Order → NATS | TCP :4222 | Публикация событий (Outbox pattern) |
| Payment → PSP (Внешний) | HTTPS :443 | Инициация транзакций |

### Список сервисов
| Сервис | Replicas / AZ | Ответственность | Stateful / Stateless |
| :--- | :--- | :--- | :--- |
| **API Gateway (Traefik)** | x2 per AZ (6 total) | Routing, Rate Limiting, TLS | Stateless |
| **Catalog Service** | x2 per AZ (6 total) | Поиск, меню, рестораны | Stateless |
| **Order Service** | x2 per AZ (6 total) | Корзина, заказы, статусы | Stateless |
| **Payment Service** | x2 per AZ (6 total) | Платежи, интеграция с PSP | Stateless |
| **Notification Service** | x1 per AZ (3 total) | Push, SMS уведомления | Stateless |
| **PostgreSQL 18** | HA (3 nodes) | Основное хранилище (ACID) | Managed |
| **Valkey™ 7** | 3 hosts (Cluster) | Кэш, корзины, трекинг | Managed |
| **Meilisearch** | 3 nodes | Поисковый индекс ресторанов | Compute VMs |
| **NATS JetStream** | 3 nodes (Cluster) | Брокер событий (Event-driven) | Compute VMs |

> **Примечание по Valkey**: В Yandex Cloud используется **Managed Service for Valkey™** — полностью совместимая с Redis OSS альтернатива, обеспечивающая высокую доступность и производительность для кэширования и работы с состоянием.

---

### 1.2. Стратегия деплоя

### Ключевые сервисы
| Сервис | Стратегия | Почему подходит | Миграции БД |
| :--- | :--- | :--- | :--- |
| **Catalog** | Rolling | Read-heavy, допускается сосуществование версий. | Expand/Contract. |
| **Payment** | Canary | **Критичный контур.** Риск бага = прямые убытки. Сначала 5%. | Expand/Contract. |
| **Order** | Canary -> Rolling | Самый критичный write-path. Ошибка ведет к потере заказов. | Только Expand/Contract. |

### Zero-downtime контракт
Применимо ко всем stateless-сервисам (Catalog, Order, Payment):

**Probes (K8s style):**
- **Readiness:** `/health/ready` — проверка связи с PG/NATS/Valkey. При неудаче под исключается из балансировки.
- **Liveness:** `/health/live` — проверка, что процесс не «завис». Перезапуск пода при провале.
- **Startup:** `/health/ready` (initialDelay: 10s) — время на прогрев соединений.

**Graceful Shutdown:**
1. **SIGTERM** — сервис переключает `/health/ready` в 503.
2. **PreStopHook (10-15s)** — ожидание, пока Ingress/ALB обновит список эндпоинтов.
3. **Drain** — завершение обработки активных (in-flight) запросов (до 30s; для Payment — до 60s).
4. **Exit** — корректное закрытие соединений с БД и брокером.

### Подход к миграциям БД (Expand/Contract)
1.  **Expand**: Добавление новых колонок/таблиц (nullable). Приложение пишет в обе версии.
2.  **Migrate**: Фоновая миграция данных батчами.
3.  **Contract**: Переключение чтения на новую схему. Удаление старых колонок в следующем релизе.

**Где критично:** `orders`, `payments`, `saga_state` (финансовая целостность).
**Где не применимо:** `Valkey` (кэш можно прогреть), `Meilisearch` (атомарное переключение алиаса индекса).

---

### 1.3. Observability

### Алерты (Golden Signals)
Алерты настроены на критический путь (Checkout и Payment).

| Сигнал | Метрика (PromQL) | Порог | Почему? |
| :--- | :--- | :--- | :--- |
| **Latency** | `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{route="/api/v1/orders"}[5m]))` | > 500ms | Прямое влияние на брошенные корзины. |
| **Errors** | `sum(rate(http_requests_total{status=~"5..", service="payment"}[5m])) / sum(rate(http_requests_total{service="payment"}[5m]))` | > 1.0% | Ошибки оплаты = прямые убытки. |
| **Traffic** | `sum(rate(traefik_requests_total[10m]))` | < 50% vs baseline | Сигнал о проблемах на уровне DNS/LB или магистрали. |
| **Saturation** | `pg_stat_activity_count / pg_settings_max_connections` | > 85% | Предвестник каскадного отказа БД. |

### Дашборды (3 уровня)
1.  **Overview (Level 1)**: GMV (руб/мин), % успешных оплат, суммарный RPS, статус SLO. Для дежурного менеджера.
2.  **Service RED (Level 2)**: **R**ate, **E**rrors, **D**uration в разрезе каждого микросервиса. Версия пода, CPU/RAM лимиты.
3.  **Infrastructure USE (Level 3)**: **U**tilization, **S**aturation, **E**rrors по ресурсам (PG locks, NATS queue depth, Valkey hit rate, Meilisearch index size).

### Логирование
*   **Формат**: Структурированный JSON.
*   **Пример**: `{"ts": "...", "level": "error", "trace_id": "...", "service": "order-service", "msg": "failed to create order", "user_id_hash": "sha256:...", "env": "prod"}`
*   **Обязательные поля**: `ts`, `level`, `service`, `trace_id`, `msg`, `user_id_hash`, `env`.
*   **Политика безопасности (PII)**:
    - **НЕ логируем**: ФИО, полные адреса, телефоны (только хэш или маска), данные карт (PCI DSS), пароли.
    - **Маскирование**: `email: a***o@mail.ru`.

---

## Часть 2: Доступность

### 2.1 Целевая доступность (Composite SLA)
Мы используем разные уровни SLA для разных контуров, так как их бизнес-вес различен.

| Контур | Целевая доступность | Простой/год | Обоснование |
| :--- | :--- | :--- | :--- |
| **Платежи** | **99.99%** | 53 мин | Прямая выручка. Ошибка = потеря денег (НФТ-006). |
| **Заказы (Checkout)** | **99.95%** | 4.4 часа | Критичный write-path. Простой = клиент уходит к конкуренту. |
| **Каталог и Поиск** | **99.9%** | 8.8 часа | Read-heavy. При падении возможна деградация на кэш. |
| **Уведомления** | **99.5%** | 1.8 дня | Best-effort. События в NATS не теряются, дойдут позже. |

### 2.2 Бизнес-обоснование (Money Risk)
Расчёт стоимости простоя для обоснования затрат на HA-инфраструктуру:

- **Заказов в день**: 1 125 000.
- **Средний чек**: 1 200 руб.
- **Выручка платформы (GMV)**: 1.35 млрд руб/день.
- **Комиссия (Revenue)**: 270 млн руб/день (~11.25 млн руб/час).
- **В пик (Peak_coef=2)**: **~22.5 млн руб/час** потерь при полном простое.

**Дельта SLA:**
- Переход с 99.9% (8.8ч простоя) на 99.99% (0.9ч простоя) спасает до **~170 млн руб. выручки в год**, что полностью окупает затраты на Managed DB HA-кластеры и SRE-смену.

### 2.3 Стратегия резервирования
| Компонент | Схема | Геораспределение | RPO / RTO |
| :--- | :--- | :--- | :--- |
| **Stateless Services** | Active/Active | 3 AZ (ru-central1-a/b/c) | 0 / < 1 мин |
| **PostgreSQL** | Master + Sync Replica + Async | 3 AZ (M-a, S-b, A-c) | 0 / 2-5 мин |
| **Valkey / NATS** | Clustered | 3 AZ (Quorum) | 0 / < 2 мин |
| **Object Storage** | Native Replication | Multi-AZ (out-of-the-box) | 0 / 0 |

### 2.4 Maintenance Window
- **Когда**: Вторник/Четверг **03:00–05:00 MSK** (минимум заказов).
- **Что делаем**: Мажорные апгрейды БД, тяжелые миграции схемы (Contract phase), учения по failover.
- **Что НЕ делаем**: Релизы в Payment Service, работы без утвержденного rollback-плана.

---

## Часть 3: TCO в Yandex Cloud

### 3.1 Маппинг компонентов на сервисы YC
| Наш компонент | Сервис YC | Конфигурация (Sizing) | Обоснование |
| :--- | :--- | :--- | :--- |
| **K8s Nodes** | Managed K8s | 6 nodes: 4 vCPU / 16 GB / 100 GB SSD | По 2 ноды в каждой AZ для отказоустойчивости приложений. |
| **PostgreSQL** | Managed PG | 3 hosts: 4 vCPU / 16 GB / 500 GB SSD | Master + Sync Slave + Async Slave. RPO=0. |
| **NATS JetStream** | Compute Cloud | 3 VMs: 2 vCPU / 8 GB / 100 GB SSD | Отказоустойчивый брокер (At-least-once). |
| **Valkey** | Managed Valkey | 3 hosts: 2 vCPU / 8 GB / 100 GB SSD | Высокодоступный кэш и сессии. |
| **Meilisearch** | Compute Cloud | 3 VMs: 4 vCPU / 16 GB / 200 GB SSD | Быстрый поиск по ресторанам и меню. |
| **Network & CDN** | ALB + CDN | 5 TB (1x) → 25 TB (5x) egress | Входной балансировщик и раздача статики. |

### 3.2 Сценарии масштабирования (Infra руб/мес)
| Категория | 1x (Текущая) | 2x Рост | 5x Рост |
| :--- | :---: | :---: | :---: |
| **Compute + K8s** | 60 000 ₽ | 115 000 ₽ | 260 000 ₽ |
| **PostgreSQL** | 60 000 ₽ | 115 000 ₽ | 250 000 ₽ |
| **NATS JetStream** | 20 000 ₽ | 35 000 ₽ | 80 000 ₽ |
| **Valkey** | 25 000 ₽ | 40 000 ₽ | 105 000 ₽ |
| **Meilisearch** | 35 000 ₽ | 70 000 ₽ | 160 000 ₽ |
| **Storage + Network** | 25 000 ₽ | 50 000 ₽ | 115 000 ₽ |
| **Monitoring + Log** | 5 000 ₽ | 10 000 ₽ | 25 000 ₽ |
| **Итого Infra** | **~230 000 ₽** | **~435 000 ₽** | **~995 000 ₽** |

### 3.3 Операционные затраты (OPEX)
| Статья OPEX | 1x | 2x | 5x |
| :--- | :--- | :--- | :--- |
| **SRE / On-call** | 190 000 ₽ | 280 000 ₽ | 560 000 ₽ |
| **Maintenance / DR** | 40 000 ₽ | 60 000 ₽ | 100 000 ₽ |
| **Support / Tooling** | 30 000 ₽ | 50 000 ₽ | 120 000 ₽ |
| **Итого OPEX** | **~260 000 ₽** | **~390 000 ₽** | **~780 000 ₽** |
| **TCO (Infra + OPEX)** | **~490 000 ₽** | **~825 000 ₽** | **~1 775 000 ₽** |

### 3.4 Анализ: Что дороже всего?
1.  **Лидеры затрат**: Kubernetes (Compute), PostgreSQL и Meilisearch. Это основные драйверы стоимости, обеспечивающие производительность API и поиска.
2.  **Эффект масштаба**: При росте нагрузки в 5x, инфраструктура растет в 4.3x, а суммарный TCO (включая OPEX) всего в **3.6x**. Это подтверждает эффективность облачной модели.
3.  **Оптимизация**: Основной потенциал экономии — вынос медиа-контента на CDN и оптимизация retention логов.

### 3.5 Тарифная политика (Unit Prices)
Расчеты произведены на основе официальных тарифов Yandex Cloud: [https://yandex.cloud/ru/docs/compute/pricing](https://yandex.cloud/ru/docs/compute/pricing):
- **Compute vCPU (Intel Ice Lake)**: 1.24 ₽ / vCPU-час.
- **Compute RAM (Intel Ice Lake)**: 0.33 ₽ / ГБ-час.
- **Managed PG vCPU (AMD Zen 4)**: ~1.88 ₽ / vCPU-час (Base 1.26 + Managed Markup).
- **Managed PG RAM (AMD Zen 4)**: ~0.51 ₽ / ГБ-час (Base 0.34 + Managed Markup).
- **Network SSD**: 14.33 ₽ / ГБ-месяц (на базе 0.0199 ₽/ГБ-час).
- **ALB Unit**: 2.63 ₽ / час.
- **Egress трафик**: 1.42 ₽ / ГБ (после первых 100 ГБ).
