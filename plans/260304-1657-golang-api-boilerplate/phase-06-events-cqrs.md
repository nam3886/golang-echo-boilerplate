# Phase 6: Events & CQRS

**Priority:** P1 | **Effort:** L (4-8h) | **Status:** completed
**Depends on:** Phase 5
**Completed:** 2026-03-04

## Context

- [Framework Research](../reports/researcher-260304-1217-golang-boilerplate-research.md) — Watermill, CQRS
- [Architecture Patterns](../reports/researcher-260304-1437-golang-architecture-patterns.md) — Event-driven patterns

## Overview

Set up Watermill with RabbitMQ, implement CQRS EventBus, create event subscribers (audit trail, notification, cache invalidation), background cron jobs with distributed lock, and email notification adapter.

## Files to Create

```
internal/shared/events/bus.go              # Watermill EventBus wrapper
internal/shared/events/subscriber.go       # Watermill Router + subscriber setup
internal/shared/events/marshaler.go        # JSON marshaler for events
internal/shared/events/topics.go           # Topic constants
internal/shared/events/module.go           # Fx module for events

internal/modules/audit/subscriber.go       # Audit log event handler
internal/modules/audit/module.go           # Fx module

internal/modules/notification/sender.go    # NotificationSender interface
internal/modules/notification/email.go     # SMTP email adapter
internal/modules/notification/subscriber.go # Event handler → send notifications
internal/modules/notification/templates/   # Email templates
internal/modules/notification/module.go    # Fx module

internal/shared/cron/scheduler.go          # robfig/cron + Redis distributed lock
internal/shared/cron/module.go             # Fx module
```

## Implementation Steps

### 1. Watermill + RabbitMQ setup
```go
// internal/shared/events/bus.go
func NewPublisher(cfg *config.Config) (message.Publisher, error) {
    amqpCfg := amqp.NewDurableQueueConfig(cfg.RabbitURL)
    return amqp.NewPublisher(amqpCfg, watermill.NewSlogLogger(slog.Default()))
}

func NewSubscriber(cfg *config.Config) (message.Subscriber, error) {
    amqpCfg := amqp.NewDurableQueueConfig(cfg.RabbitURL)
    return amqp.NewSubscriber(amqpCfg, watermill.NewSlogLogger(slog.Default()))
}
```

### 2. Event bus wrapper
```go
// internal/shared/events/bus.go
type EventBus struct {
    publisher message.Publisher
    marshaler marshaler
}

func (b *EventBus) Publish(ctx context.Context, topic string, event any) error {
    payload, err := b.marshaler.Marshal(event)
    if err != nil { return fmt.Errorf("marshaling event: %w", err) }

    msg := message.NewMessage(uuid.NewString(), payload)
    // Propagate trace context into message metadata
    otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Metadata))
    msg.Metadata.Set("event_type", topic)

    return b.publisher.Publish(topic, msg)
}
```

### 3. Event definitions
```go
// internal/shared/events/topics.go
const (
    TopicUserCreated = "user.created"
    TopicUserUpdated = "user.updated"
    TopicUserDeleted = "user.deleted"
)

// Shared event structs
type UserCreatedEvent struct {
    UserID string    `json:"user_id"`
    Email  string    `json:"email"`
    Name   string    `json:"name"`
    Role   string    `json:"role"`
    At     time.Time `json:"at"`
}
```

### 4. Publish events from user module
```go
// internal/modules/user/app/create_user.go — updated
type CreateUserHandler struct {
    repo   domain.UserRepository
    hasher auth.PasswordHasher
    events *events.EventBus  // ← add
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) (*domain.User, error) {
    // ... create user in DB ...

    // Publish event AFTER successful DB write
    h.events.Publish(ctx, events.TopicUserCreated, events.UserCreatedEvent{
        UserID: string(user.ID()), Email: user.Email(),
        Name: user.Name(), Role: string(user.Role()),
        At: time.Now(),
    })
    return user, nil
}
```

### 5. Watermill Router (subscriber orchestration)
```go
// internal/shared/events/subscriber.go
func NewRouter(
    sub message.Subscriber,
    auditHandler *audit.Handler,
    notifHandler *notification.Handler,
) *message.Router {
    router, _ := message.NewRouter(message.RouterConfig{},
        watermill.NewSlogLogger(slog.Default()),
    )

    // Middleware: recovery, OTel context extraction, retry
    router.AddMiddleware(
        mw.Recoverer,
        mw.Retry{MaxRetries: 3, InitialInterval: time.Second}.Middleware,
        otelExtractMiddleware(), // Extract trace context from message metadata
    )

    // Audit subscriber — listens to ALL events
    router.AddHandler("audit.user_created", TopicUserCreated, sub,
        "audit.user_created", sub, // output topic (none needed, use same)
        auditHandler.HandleUserCreated,
    )
    // ... more audit handlers

    // Notification subscriber
    router.AddHandler("notify.user_created", TopicUserCreated, sub,
        "", nil, // no output
        notifHandler.HandleUserCreated,
    )

    return router
}
```

### 6. Audit trail subscriber
```go
// internal/modules/audit/subscriber.go
type Handler struct {
    queries *sqlcgen.Queries
}

func (h *Handler) HandleUserCreated(msg *message.Message) ([]*message.Message, error) {
    var event events.UserCreatedEvent
    json.Unmarshal(msg.Payload, &event)

    ctx := extractContext(msg) // OTel context from metadata
    h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
        EntityType: "user",
        EntityID:   uuid.MustParse(event.UserID),
        Action:     "created",
        ActorID:    actorFromContext(ctx),
        Changes:    toJSON(event),
    })
    return nil, nil
}
```

### 7. Notification system
```go
// internal/modules/notification/sender.go
type Sender interface {
    Send(ctx context.Context, to string, subject string, body string) error
}

// internal/modules/notification/email.go
type SMTPSender struct {
    host string
    port int
    from string
}
func (s *SMTPSender) Send(ctx context.Context, to, subject, body string) error {
    // net/smtp with html/template rendered body
}

// internal/modules/notification/subscriber.go
type Handler struct {
    sender Sender
    tmpl   *template.Template
}
func (h *Handler) HandleUserCreated(msg *message.Message) ([]*message.Message, error) {
    var event events.UserCreatedEvent
    json.Unmarshal(msg.Payload, &event)
    body := h.tmpl.Execute("welcome.html", event)
    return nil, h.sender.Send(ctx, event.Email, "Welcome!", body)
}
```

### 8. Cron scheduler with distributed lock
```go
// internal/shared/cron/scheduler.go
type Scheduler struct {
    cron *cron.Cron
    rdb  *redis.Client
}

func NewScheduler(rdb *redis.Client) *Scheduler {
    c := cron.New(cron.WithSeconds())
    return &Scheduler{cron: c, rdb: rdb}
}

func (s *Scheduler) AddJob(spec, name string, fn func(ctx context.Context) error) {
    s.cron.AddFunc(spec, func() {
        ctx := context.Background()
        // Acquire distributed lock
        lock, err := s.rdb.SetNX(ctx, "cron:"+name, "locked", 5*time.Minute).Result()
        if err != nil || !lock { return } // Another instance has the lock
        defer s.rdb.Del(ctx, "cron:"+name)
        if err := fn(ctx); err != nil {
            slog.Error("cron job failed", "job", name, "err", err)
        }
    })
}

// Register in Fx lifecycle: OnStart → s.cron.Start(), OnStop → s.cron.Stop()
```

### 9. Fx module wiring
```go
// internal/shared/events/module.go
var Module = fx.Module("events",
    fx.Provide(NewPublisher),
    fx.Provide(NewSubscriber),
    fx.Provide(NewEventBus),
    fx.Provide(NewRouter),
    fx.Invoke(startRouter), // Fx lifecycle: OnStart → router.Run(ctx)
)
```

## Todo

- [x] Watermill Publisher + Subscriber (RabbitMQ AMQP)
- [x] EventBus wrapper with OTel trace propagation
- [x] Event type definitions (UserCreated, UserUpdated, UserDeleted)
- [x] Watermill Router with middleware (recovery, retry, OTel extract)
- [x] Publish events from user command handlers (after DB commit)
- [x] Audit trail subscriber → audit_logs table
- [x] Notification sender interface + SMTP adapter
- [x] Email templates (html/template)
- [x] Notification subscriber (welcome email on user created)
- [x] Cron scheduler (robfig/cron v3) + Redis distributed lock
- [x] Example cron job: cleanup expired refresh tokens
- [x] Fx modules for events, audit, notification, cron
- [x] Fx lifecycle: Router start/stop, Cron start/stop
- [x] Verify: create user → audit log written + email sent (MailHog)
- [x] Verify: cron job runs on schedule, only one instance executes

## Success Criteria

- Create user → audit_logs row created with correct entity_type/action/changes
- Create user → welcome email in MailHog inbox
- Cron cleanup job runs on schedule
- Distributed lock prevents duplicate cron execution
- OTel trace propagated: HTTP request → event publish → subscriber traces linked
- Watermill retry: failed handler retries 3 times before dead-lettering

## Risk Assessment

- **Publish after commit:** If app crashes between DB commit and event publish, event is lost. Acceptable for V1. Watermill outbox pattern solves this later.
- **RabbitMQ connection:** Must handle reconnection gracefully. Watermill handles this internally.

## Next Steps

→ Phase 7: DevOps & Testing (Docker, CI/CD, testcontainers, SigNoz)
