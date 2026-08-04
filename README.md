# linkguard

*Read this in [English](#english) or [Español](#español).*

---

## English

### Problem

A publicly exposed URL shortener is an easy target for abuse: a client can
create thousands of URLs at once, or mass-scrape the redirect endpoint.

If the service runs with multiple replicas, limiting requests in local
(in-process) memory doesn't work — each replica keeps its own counter, so
the real, effective limit becomes meaningless. A client hitting a
load-balanced fleet of `N` replicas can burst up to `N` times the intended
rate simply by spreading requests across instances, and each replica has no
visibility into what the others have already counted.

**This is the problem linkguard solves:** providing a URL shortener with
rate limiting that stays correct and consistent across multiple replicas,
regardless of which instance handles a given request.

---

## Español

### Problema

Un acortador de URLs expuesto públicamente es un blanco fácil para el
abuso: un cliente puede crear miles de URLs de una sola vez, o hacer
scraping masivo del endpoint de redirección.

Si el servicio corre con múltiples réplicas, limitar las peticiones en
memoria local (dentro del proceso) no funciona — cada réplica mantiene su
propio contador, y el límite real y efectivo pierde todo sentido. Un
cliente que le pega a una flota de `N` réplicas detrás de un balanceador
puede llegar a ráfagas de hasta `N` veces la tasa prevista, simplemente
repartiendo sus peticiones entre las distintas instancias, ya que ninguna
réplica tiene visibilidad de lo que las demás ya contaron.

**Este es el problema que resuelve linkguard:** ofrecer un acortador de
URLs con limitación de tasa (rate limiting) que se mantenga correcta y
consistente entre múltiples réplicas, sin importar qué instancia atienda
una petición dada.

### Cómo lo resuelve

El límite de tasa (100 req/s por empresa, identificada por el header
`X-API-Key`) vive en Redis, no en memoria del proceso: todas las réplicas
comparten el mismo contador para una misma llave. El algoritmo es un
**Sliding Window Counter**, aplicado atómicamente vía un script Lua
(`EVAL`) en un único round-trip a Redis, para que dos réplicas nunca lean
un conteo obsoleto antes de que la otra lo incremente.

Arquitectura hexagonal ligera:

- `internal/domain` — puertos (interfaces) y entidades: `URLRepository`,
  `RateLimiter`, `WindowCounter`.
- `internal/usecase` — lógica de negocio pura (`CreateShortURL`, `Resolve`,
  el algoritmo del sliding window), testeable sin Postgres ni Redis reales.
- `internal/adapter` — implementaciones reales: `postgres/`, `redis/`,
  `http/`.

### Uso rápido

```bash
cp .env.example .env
docker compose up --build
```

```bash
curl -X POST http://localhost:8080/shorten \
  -H "X-API-Key: acme" -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/some/long/path"}'
# -> {"shortcode":"aZ3xQ9b","short_url":"http://localhost:8080/aZ3xQ9b", ...}

curl -i -H "X-API-Key: acme" http://localhost:8080/aZ3xQ9b
# -> 302 Found, Location: https://example.com/some/long/path
```

Documentación interactiva (Swagger UI) en `http://localhost:8080/docs`.

Tests: `make test` (equivalente a `go test -race -cover ./...`).

---

## English (quickstart)

The rate limit (100 req/s per company, identified by the `X-API-Key`
header) lives in Redis, not process memory: every replica shares the same
counter for a given key. The algorithm is a **Sliding Window Counter**,
applied atomically via a Lua script (`EVAL`) in a single Redis round trip,
so two replicas can never both read a stale count before either
increments it.

```bash
cp .env.example .env
docker compose up --build
```

```bash
curl -X POST http://localhost:8080/shorten \
  -H "X-API-Key: acme" -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/some/long/path"}'

curl -i -H "X-API-Key: acme" http://localhost:8080/aZ3xQ9b
```

Interactive docs (Swagger UI) at `http://localhost:8080/docs`. Tests:
`make test`.
