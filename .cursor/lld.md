I actually like that direction much better.

**Nektar** is a stronger product name than Manthan for an engineering tool. The idea is simple:

> **Nektar extracts the nectar (signal) from a sea of information (noise).**

I'd also avoid coupling the architecture to Kafka. Instead, I'd make **the messaging layer a port**, and Redis would simply be the first implementation.

This follows the same philosophy as how we abstract Gmail, PostgreSQL, and LLM providers.

---

# Nektar Architecture

```text
                        +----------------------+
                        |      Scheduler       |
                        +----------+-----------+
                                   |
                                   v
                        +----------------------+
                        |    Fetch Pipeline    |
                        +----------+-----------+
                                   |
                             Publish Event
                                   |
                                   v
                   +-------------------------------+
                   | Event Bus (Port)              |
                   |-------------------------------|
                   | Redis Streams (today)         |
                   | NATS (future)                |
                   | Kafka (future)               |
                   | In-Memory (tests)            |
                   +-------------------------------+
                                   |
          ----------------------------------------------------
          |            |            |             |            |
          v            v            v             v            v
     Extractor    Embedding    Clustering    Digest      Publisher
```

Notice there is **no Redis** in the business logic.

The application only knows:

```go
EventBus
```

---

# Infrastructure becomes swappable

Instead of

```text
Application
    ↓
Kafka
```

we do

```text
Application
      ↓
EventBus Interface
      ↓
----------------------------
Redis Streams

Kafka

NATS

RabbitMQ

InMemory
```

Changing infrastructure becomes a configuration change.

---

# Project Structure

I would reorganize the project slightly.

```text
nektar/

cmd/

internal/

    modules/

        fetcher/

        newsletter/

        extractor/

        article/

        embedding/

        clustering/

        digest/

        publisher/

        llm/

    platform/

        eventbus/

            interface.go

            inmemory/

            redis/

            kafka/

            nats/

        storage/

            postgres/

        cache/

            redis/

        scheduler/

        logger/

        config/

        metrics/

    shared/

        domain/

        events/

        utils/

configs/

deployments/

pkg/
```

Notice

Redis is **inside platform**, not inside modules.

---

# Event Bus Interface

This becomes one of the most important abstractions.

```go
type EventBus interface {
    Publish(
        ctx context.Context,
        event Event,
    ) error

    Subscribe(
        ctx context.Context,
        topic string,
        handler Handler,
    ) error

    Close() error
}
```

Nothing mentions Redis.

---

# Event

```go
type Event interface {
    Name() string

    Payload() any
}
```

---

Example

```go
type EmailFetched struct {
    EmailID string
}
```

---

# Redis Implementation

```text
platform/

    eventbus/

        redis/

            bus.go

            publisher.go

            subscriber.go

            consumer.go

            stream.go
```

Internally

```text
XADD

XREADGROUP

ACK

Retry

Dead Letter Queue
```

No one else knows.

---

# Kafka Implementation Later

```text
platform/

    eventbus/

        kafka/

            bus.go

            producer.go

            consumer.go
```

Nothing changes.

---

# InMemory Implementation

For tests.

```go
bus.Publish(...)
```

Immediately executes subscribers.

Unit tests become extremely fast.

---

# LLM Provider

Same idea.

```go
type Provider interface {

    Embed(ctx,...)

    Summarize(ctx,...)

}
```

Implementations

```text
Gemini

OpenAI

Claude

Local Ollama
```

---

# Email Provider

```go
type Provider interface {

    Fetch(...)

}
```

Implementations

```text
Gmail

Outlook

IMAP
```

---

# Storage

Don't depend directly on PostgreSQL either.

```go
ArticleRepository

DigestRepository

EmailRepository
```

Implementation

```text
postgres/

sqlite/

memory/
```

SQLite becomes useful for local development.

---

# Cache

Likewise

```go
Cache interface {

    Get()

    Set()

    Delete()
}
```

Implementations

```text
Redis

Memory
```

---

# Publisher

```go
Publisher
```

Implementations

```text
Discord

Slack

Telegram

Email

RSS
```

---

# Config Driven

```yaml
eventbus:

  provider: redis

storage:

  provider: postgres

cache:

  provider: redis

llm:

  provider: gemini

email:

  provider: gmail

publisher:

  provider: discord
```

Change one line.

Restart.

Done.

---

# Bootstrap

The bootstrapper builds dependencies.

```text
main.go

↓

Load Config

↓

Build Storage

↓

Build Event Bus

↓

Build Cache

↓

Build LLM

↓

Build Publishers

↓

Register Handlers

↓

Start Workers
```

---

# Worker Registration

```go
bus.Subscribe(
    EmailFetched,
    extractor.Handle,
)

bus.Subscribe(
    ArticleCreated,
    embedding.Handle,
)

bus.Subscribe(
    EmbeddingCreated,
    cluster.Handle,
)

bus.Subscribe(
    ClusterUpdated,
    digest.Handle,
)

bus.Subscribe(
    DigestReady,
    publisher.Handle,
)
```

Everything becomes event-driven.

---

# Dependency Rule

```text
                Domain

                  ↑

             Application

                  ↑

          Infrastructure Ports

                  ↑

Infrastructure Implementations
```

Business logic never imports Redis, PostgreSQL, Discord, Gemini, or Gmail. It only imports interfaces.

---

## One more architectural improvement I'd make

Instead of calling the infrastructure layer `platform`, I'd split it into two concepts inspired by Hexagonal Architecture:

```text
internal/
├── modules/              # Business capabilities
│   ├── fetcher/
│   ├── extractor/
│   ├── digest/
│   └── ...
│
├── ports/                # Interfaces (contracts)
│   ├── eventbus/
│   ├── llm/
│   ├── email/
│   ├── repository/
│   ├── cache/
│   └── publisher/
│
├── adapters/             # Implementations
│   ├── redis/
│   ├── postgres/
│   ├── gmail/
│   ├── discord/
│   ├── gemini/
│   ├── inmemory/
│   └── sqlite/
│
└── bootstrap/
```

This makes the architecture explicitly **Ports & Adapters (Hexagonal)**. If, six months from now, you decide to replace Redis Streams with NATS or Kafka, or PostgreSQL with CockroachDB, you'll only add or swap an adapter—**none of the business modules change**. For a long-lived project like **Nektar**, that's the kind of portability that pays off over time.
