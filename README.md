# Highload Food Delivery Service

Проект по разработке архитектуры высоконагруженного сервиса доставки еды (Россия + СНГ) с аудиторией 5–20 млн MAU.

## 📚 Документация

Вся проектная документация разделена на ключевые разделы:

*   **[Требования к системе](docs/requirements.md)** — контекст, ФТ/НФТ, оценка нагрузки (Capacity Estimation) и Latency Budget.
*   **[Архитектура системы](docs/architecture.md)** — описание выбранного стиля (SBA), C4 диаграммы, API Design и модель данных.
*   **[Architectural Decision Records (ADR)](docs/adr/)** — реестр принятых архитектурных решений и их обоснования.

## 🏗 Ключевые архитектурные решения

*   **Стиль:** Service-Based Architecture (SBA) с элементами Event-Driven.
*   **Базы данных:** Polyglot Persistence (PostgreSQL для ACID, Elasticsearch для поиска, Redis для кэша и корзин).
*   **Надежность:** Паттерн Transactional Outbox для гарантии доставки событий и Idempotency Keys для защиты от дублей.
*   **Идентификаторы:** UUID v7 как основной формат ID для обеспечения K-сортируемости и производительности индексов.

## 🛠 Технологический стек

*   **Backend:** Go
*   **Databases:** PostgreSQL 18, Elasticsearch 8, Redis 7
*   **Messaging:** Apache Kafka
*   **API Gateway:** Kong
*   **Infrastructure:** Cloud-based (VM) + CDN для статики
