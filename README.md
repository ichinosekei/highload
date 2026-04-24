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
