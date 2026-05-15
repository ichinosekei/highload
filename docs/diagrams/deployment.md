# Deployment Diagram — FoodDelivery Platform

Каноническая deployment-диаграмма в формате Draw.io. Показывает физическое размещение сервисов по зонам доступности, сетевые зоны, направления связей и используемые протоколы/порты.

## Визуализация

![Deployment Diagram](./deployment.png)

> **Примечание:** Для редактирования используйте файл [deployment.drawio](./deployment.drawio) в приложении diagrams.net (draw.io).

## Сетевая топология

| Зона | CIDR | Назначение |
| :--- | :--- | :--- |
| **Regional Layer** | - | L7 ALB (Application Load Balancer) |
| **Public Zone (DMZ)** | `10.128.0.0/16` | Traefik Ingress, NAT Gateways |
| **Private App Zone** | `10.128.1.0/24`, `10.129.1.0/24`, `10.130.1.0/24` | Микросервисы (Catalog, Order, etc.) |
| **Private Data Zone** | `10.128.2.0/24`, `10.129.2.0/24`, `10.130.2.0/24` | Хранилища (PG, Redis, NATS, Meili) |

## Адресация

| Компонент | FQDN / Адрес | Порт | Протокол | Комментарий |
| :--- | :--- | :--- | :--- | :--- |
| **ALB (External)** | `api.fooddelivery.ru` | `:443` | HTTPS | Публичный вход (Static IP) |
| **Traefik** | `traefik.svc.cluster.local` | `:80` | HTTP | Ingress внутри DMZ |
| **Catalog** | `catalog.svc.cluster.local` | `:8080` | HTTP/gRPC | Поиск и меню |
| **Order** | `order.svc.cluster.local` | `:8082` | HTTP/gRPC | Оформление заказов |
| **Payment** | `payment.svc.cluster.local` | `:8083` | HTTP/gRPC | Обработка платежей |
| **PostgreSQL** | `postgres.db.internal` | `:5432` | SQL | Master (AZ-a) |
| **Redis** | `redis.infra.internal` | `:6379` | RESP | Кэш / Корзины |
| **NATS** | `nats.infra.internal` | `:4222` | NATS | Event Bus (JetStream) |
| **Meilisearch** | `meilisearch.internal` | `:7700` | HTTP | Поисковый индекс |

> **Примечание:** Эфемерные IP подов K8s и autoscaling-инстансов не указываются — они управляются оркестратором.

## Обоснование топологии (Why 3 AZ?)
Выбор 3-зонной архитектуры обусловлен следующими факторами:
1.  **Quorum & Consensus**: Для корректной работы кворумных систем (NATS JetStream, Redis Cluster) требуется нечетное количество узлов (минимум 3). Размещение их в разных AZ исключает ситуацию "split-brain" при отказе одной зоны.
2.  **High Availability (99.95%)**: Использование 3 AZ позволяет системе выдерживать полный отказ одного дата-центра без потери доступности для записи (в отличие от 2 AZ, где отказ зоны с Master-узлом требует ручного или сложного автоматического вмешательства с риском потери кворума).
3.  **Cloud Native Best Practices**: Yandex Cloud предоставляет нативную поддержку распределения ресурсов по 3 зонам (ru-central1-a/b/c), что позволяет реализовать HA-схему с минимальными накладными расходами на сетевую связность.
