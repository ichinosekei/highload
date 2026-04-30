# Highload Food Delivery Service

Проект по разработке архитектуры высоконагруженного сервиса доставки еды (Россия + СНГ) с аудиторией 5–20 млн MAU.

## 📚 Документация

Вся проектная документация разделена на ключевые разделы:

*   **[Требования к системе](docs/requirements.md)** — контекст, ФТ/НФТ, оценка нагрузки (Capacity Estimation) и Latency Budget.
*   **[Архитектура системы](docs/architecture.md)** — описание выбранного стиля (SBA), C4 диаграммы, API Design и модель данных.
*   **[Architectural Decision Records (ADR)](docs/adr/)** — реестр принятых архитектурных решений.
*   **[Оптимизация ресурсов (ADR-004)](docs/adr/004-resource-optimization.md)** — стратегия работы в условиях 2 vCPU / 8 GB RAM.

## 🏗 Ключевые архитектурные решения

*   **Стиль:** Service-Based Architecture (SBA) с элементами Event-Driven.
*   **Базы данных:** Polyglot Persistence (PostgreSQL для ACID, Meilisearch для поиска, Redis для кэша и корзин).
*   **Надежность:** Паттерн Transactional Outbox + NATS JetStream для гарантии доставки событий.
*   **Оптимизация:** Замена тяжелых JVM-компонентов на нативные (Go/Rust) аналоги для работы на лимитированных ресурсах.

## 🛠 Технологический стек

*   **Backend:** Go
*   **Databases:** PostgreSQL 18, Meilisearch, Redis 7
*   **Messaging:** NATS JetStream
*   **API Gateway:** Traefik OSS
*   **Infrastructure:** Cloud-based (VM: 2 vCPU, 8 GB RAM) + CDN для статики

## 🚀 Как запустить
30: 
31: ```bash
32: # 1. Клонируйте репозиторий
33: git clone <repo_url>
34: cd highload
35: 
36: # 2. Запустите все сервисы
37: docker compose up -d
38: ```
39: 
40: ### Проверка работоспособности
41: - **Catalog Service:** `http://localhost:8080/health`
42: - **Notification Service:** `http://localhost:8081/health`
43: - **NATS Management:** `http://localhost:8222`
44: 
45: ## 🧩 Примененные паттерны
46: 
47: 1. **Event-Driven Consumer (Notification Service)**:
48:    - Решает проблему связности (decoupling). Уведомления отправляются асинхронно при поступлении событий в NATS.
49:    - Код: [consumer.go](services/notification/internal/delivery/nats/consumer.go)
50: 2. **Transactional Outbox (в планах для Order/Payment)**:
51:    - Обеспечивает At-Least-Once доставку событий из БД в NATS.
52: 3. **Dependency Injection**:
53:    - Обеспечивает тестируемость через интерфейсы.
54:    - Код: [service.go](services/notification/internal/app/service.go)
55: 4. **Mock Object (для тестов)**:
56:    - Позволяет тестировать бизнес-логику без реальных Push/SMS провайдеров.
57:    - Код: [mocks.go](services/notification/internal/domain/mocks.go)
