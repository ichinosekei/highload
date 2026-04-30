# Лог оптимизации

## Таблица прогресса

| Метрика         | NFR (ДЗ1)         | Iter 0 | Iter 1 | Iter 2 | Iter 3 |
|-----------------|-------------------|--------|--------|--------|--------|
| Latency p99     | Read <500ms, Write <1s |        |        |        |        |
| Max RPS         | Read ≥100, Write ≥30 |        |        |        |        |
| Error rate      | <1%               |        |        |        |        |
| CPU / RAM       | 70–90% / balanced |        |        |        |        |
| Bottleneck      | —                 |        |        |        |        |
| Что сделали     | —                 |        |        |        |        |
| NFR достигнут?  | —                 |        |        |        |        |

---

## Профиль трафика

**Mixed (read-heavy):** ~70% read (поиск/каталог/трекинг), ~30% write (заказы/оплата/статусы).

Write-планка: ≥30 RPS (create order + payment).
Read-планка: ≥100 RPS (search + track + list).

---

## Iteration 0: Baseline

### Как запустить

```bash
# 1. Поднять стек на VM
docker compose up -d

# 2. Прогрев (1–2 минуты)
k6 run --env PROFILE=smoke loadtest/stress.js

# 3. Stress-тест (baseline)
k6 run --env PROFILE=stress loadtest/stress.js

# 4. Метрики ресурсов на VM (в отдельном терминале)
docker stats --no-stream
htop
iostat -x 2
```

### RED метрики

- **Rate (Max RPS):** TODO после замера
- **Errors (Error rate):** TODO
- **Duration (Latency p50/p95/p99):** TODO

### USE метрики

- **Utilization (CPU, RAM):** `docker stats` output
- **Saturation (Disk I/O):** `iostat -x` output
- **Errors:** OOM kills, connection refused events

### Bottleneck Analysis

TODO после baseline замера.

Типичные кандидаты для Go + PostgreSQL на HDD:
1. **Disk I/O** — HDD значительно медленнее SSD для random I/O (pg WAL, checkpoints)
2. **Connection pool** — размер пула pgx может быть мал для целевой нагрузки
3. **N+1 queries** — проверить через `pg_stat_statements`
4. **Missing indexes** — `EXPLAIN ANALYZE` для горячих запросов

---

## Iteration 1: [TODO]

### Что нашли

TODO

### Что сделали

TODO

### Результаты

TODO

---

## Iteration 2: [TODO]

### Что нашли

TODO

### Что сделали

TODO

### Результаты

TODO

---

## Типичные направления оптимизации

### Код
- [ ] Проверить отсутствие N+1 запросов в репозиториях
- [ ] Настроить `GOMEMLIMIT` для Go-сервисов
- [ ] Увеличить connection pool (pgxpool MaxConns)
- [ ] Добавить graceful degradation при перегрузке

### Данные
- [ ] Анализ запросов через `EXPLAIN ANALYZE`
- [ ] Добавить недостающие индексы (проверить `idx_orders_status`, `idx_orders_user_id`)
- [ ] Настроить PostgreSQL: `shared_buffers`, `work_mem`, `effective_cache_size`
- [ ] Добавить Redis кэширование для горячих данных (меню, ресторанов)
- [ ] Тюнинг `checkpoint_completion_target`, `wal_buffers` для HDD

### Масштабирование
- [ ] Добавить nginx как reverse proxy + load balancer
- [ ] Запустить несколько инстансов Go-сервисов (`docker-compose.scaled.yml`)
- [ ] Read replicas для PostgreSQL (если CPU БД — узкое место)
