# Application Review Checklist

**Purpose**: Comprehensive checklist for reviewing production applications based on learnings from payment service optimization analysis.

**Last Updated**: 2025-11-23

---

## 1. Security & Compliance

### PCI/PII Data Protection
- [ ] **Sensitive data not logged** (credit cards, auth tokens, BRIC, account numbers, routing numbers)
- [ ] **Log redaction** implemented for any fields that might contain PCI data
- [ ] **Secret manager** used for credentials (not environment variables or config files)
- [ ] **Secrets not in git history** (check `.env`, `credentials.json`, `secrets/`)
- [ ] **TLS/HTTPS enforced** for all external communications
- [ ] **Input validation** on all user inputs (prevent injection attacks)
- [ ] **SQL injection prevention** (parameterized queries, not string concatenation)
- [ ] **XSS protection** in web interfaces
- [ ] **CSRF protection** for state-changing operations
- [ ] **Rate limiting** implemented per IP/user/merchant
- [ ] **Authentication** required for all sensitive endpoints
- [ ] **Authorization** checks before data access (prevent cross-merchant access)

### Production Hardening
- [ ] **HTTP server timeouts configured** (ReadTimeout, WriteTimeout, IdleTimeout)
  - Prevents Slowloris attacks
  - Prevents resource exhaustion
  - Recommended: ReadTimeout 10-15s, WriteTimeout 30-60s
- [ ] **MaxHeaderBytes** set (recommend 1 MB) to prevent large header attacks
- [ ] **No debug endpoints** exposed in production
- [ ] **Error messages sanitized** (don't leak internal details)
- [ ] **Dependency vulnerabilities scanned** (go mod tidy, npm audit)

---

## 2. Performance & Scalability

### Database Performance
- [ ] **Connection pool configured** (MaxConns, MinConns, lifetimes)
- [ ] **Connection pool monitoring** (track utilization, acquire duration)
- [ ] **Query timeouts** configured (prevent blocking queries)
- [ ] **Indexes on high-frequency queries** verified with EXPLAIN ANALYZE
- [ ] **Composite indexes** for multi-column WHERE clauses
- [ ] **Partial indexes** where applicable (filtered indexes)
- [ ] **No N+1 query patterns** (use JOINs or IN clauses)
- [ ] **Pagination limits enforced** (max 100-1000 items per page)
- [ ] **Recursive queries have depth limits** (prevent DoS)
- [ ] **Slow query logging** enabled with threshold (e.g., > 1s)
- [ ] **Database query metrics** tracked (p50, p95, p99 latency)

### Caching Strategy
- [ ] **Frequently accessed data cached** (merchant credentials, payment methods, slugs)
- [ ] **Cache hit rate monitored** (target: 90%+)
- [ ] **Cache invalidation strategy** implemented (TTL + event-based)
- [ ] **Cache size limits** configured (prevent memory exhaustion)
- [ ] **Read-through caching** for database queries
- [ ] **Write-through invalidation** on updates
- [ ] **Static data pre-loaded** (response codes, reason codes, etc.)

### API Performance
- [ ] **Circuit breakers** on external API calls (prevent cascading failures)
- [ ] **Retry logic with exponential backoff** and jitter
- [ ] **Context-aware retries** (respect context cancellation)
- [ ] **Timeouts on all external calls** (EPX gateway, webhooks, etc.)
- [ ] **Concurrent processing** where appropriate (webhooks, batch operations)
- [ ] **Worker pools** with bounded concurrency (prevent goroutine explosion)
- [ ] **Bulkhead isolation** to limit blast radius
- [ ] **HTTP/2 enabled** for better multiplexing
- [ ] **Connection pooling** for external services
- [ ] **Keep-alive** connections reused

---

## 3. Memory & Resource Management

### Memory Optimization
- [ ] **Struct field alignment** checked (order fields by size: 8, 4, 2, 1 bytes)
- [ ] **Object pooling** for frequently allocated objects (sync.Pool)
- [ ] **Pre-allocation** of slices and maps with capacity hints
- [ ] **String building** uses strings.Builder (not concatenation)
- [ ] **Pointer vs value semantics** optimized (avoid heap escapes)
- [ ] **Memory profiling** conducted (pprof heap profiles)
- [ ] **No memory leaks** verified (goroutine leaks, unclosed connections)
- [ ] **Garbage collection tuned** if needed (GOGC environment variable)

### Resource Limits
- [ ] **Goroutine limits** enforced (semaphores, worker pools)
- [ ] **File descriptor limits** configured (ulimit)
- [ ] **Memory limits** set (cgroup limits, container limits)
- [ ] **CPU limits** appropriate for workload
- [ ] **Request body size limits** (prevent DoS)
- [ ] **Upload file size limits** enforced

---

## 4. Logging & Observability

### Logging Best Practices
- [ ] **Structured logging** (JSON with fields, not string formatting)
- [ ] **Log levels appropriate** (ERROR < 0.1%, WARN < 1%)
- [ ] **Request ID propagation** through all logs
- [ ] **Trace ID integration** (OpenTelemetry or similar)
- [ ] **No PCI/PII data in logs** (verified with log audits)
- [ ] **Async logging** for performance (buffered writes)
- [ ] **Log sampling** in high-throughput scenarios
- [ ] **Log rotation** configured (size and time-based)
- [ ] **Centralized logging** (ELK, CloudWatch, etc.)
- [ ] **Log retention policy** defined

### Tracing & Metrics
- [ ] **Distributed tracing** implemented (OpenTelemetry, Jaeger)
- [ ] **Key operations traced** (payment flows, external calls)
- [ ] **Metrics exposed** (Prometheus format)
- [ ] **Custom metrics** for business operations (payments/sec, failures/sec)
- [ ] **Database metrics** (query duration, connection pool stats)
- [ ] **External API metrics** (latency, error rates)
- [ ] **Alert thresholds defined** for critical metrics

---

## 5. Resilience & Reliability

### Error Handling
- [ ] **All errors checked** (no ignored errors in production code)
- [ ] **Error wrapping** with context (fmt.Errorf with %w)
- [ ] **Graceful degradation** for non-critical failures
- [ ] **Panic recovery** in HTTP handlers and goroutines
- [ ] **Transaction rollback** on errors
- [ ] **Idempotency** for critical operations (deduplication)
- [ ] **Dead letter queues** for failed asynchronous operations

### Health Checks
- [ ] **Liveness probes** (is service running?)
- [ ] **Readiness probes** (is service ready to accept traffic?)
- [ ] **Dependency health checks** (database, cache, external APIs)
- [ ] **Health check endpoints** don't perform expensive operations
- [ ] **Health check timeouts** configured (< 2 seconds)

### Graceful Shutdown
- [ ] **Signal handling** (SIGTERM, SIGINT)
- [ ] **In-flight requests complete** before shutdown
- [ ] **Connections drained** (database, external APIs)
- [ ] **Background jobs cancelled** (context cancellation)
- [ ] **Buffers flushed** (logs, metrics)
- [ ] **Shutdown timeout** configured (e.g., 30 seconds)

---

## 6. Testing

### Test Coverage
- [ ] **Unit tests** for business logic (target: 80%+ coverage)
- [ ] **Integration tests** for critical flows
- [ ] **Table-driven tests** for multiple scenarios
- [ ] **Error case testing** (not just happy paths)
- [ ] **Concurrent operation tests** (race detector enabled)
- [ ] **Timeout tests** (context cancellation)
- [ ] **Retry logic tests** (exponential backoff, max retries)

### Performance Testing
- [ ] **Load tests** conducted (target throughput validated)
- [ ] **Stress tests** (beyond normal capacity)
- [ ] **Endurance tests** (memory leaks, goroutine leaks)
- [ ] **Benchmarks** for critical paths (baseline established)
- [ ] **Database query benchmarks** (EXPLAIN ANALYZE)
- [ ] **Profile-guided optimization** (CPU, memory, block profiling)

---

## 7. Code Quality

### Architecture & Design
- [ ] **Dependency injection** used (testability)
- [ ] **Interface segregation** (clients depend on minimal interfaces)
- [ ] **Single responsibility** per service/module
- [ ] **Cyclomatic complexity** reasonable (< 10 per function)
- [ ] **No circular dependencies** between packages
- [ ] **Clear separation of concerns** (handlers, services, adapters)
- [ ] **Adapter pattern** for external dependencies

### Code Patterns
- [ ] **No conditional logic bloat** (consider Strategy pattern if > 200 lines)
- [ ] **Builder pattern** for complex object construction
- [ ] **Factory pattern** only when genuinely needed (not over-engineering)
- [ ] **Repository pattern** only if multiple data sources (don't wrap sqlc unnecessarily)
- [ ] **Consistent error handling** pattern
- [ ] **Consistent naming conventions**

### Documentation
- [ ] **README.md** with setup instructions
- [ ] **API documentation** (OpenAPI/Swagger)
- [ ] **Architecture diagrams** (dataflow, deployment)
- [ ] **CHANGELOG.md** maintained
- [ ] **Inline comments** for complex logic only (code should be self-documenting)
- [ ] **Package documentation** (godoc)

---

## 8. Deployment & Operations

### Configuration Management
- [ ] **Environment-based config** (dev, staging, prod)
- [ ] **12-factor app principles** followed
- [ ] **Secrets in secret manager** (not code or env vars)
- [ ] **Configuration validation** on startup
- [ ] **Feature flags** for gradual rollouts
- [ ] **No hardcoded values** (URLs, credentials, timeouts)

### CI/CD
- [ ] **Automated tests** in pipeline
- [ ] **Linting** (golangci-lint, staticcheck)
- [ ] **Security scanning** (dependency vulnerabilities)
- [ ] **Code formatting** enforced (gofmt, prettier)
- [ ] **Build reproducibility** (deterministic builds)
- [ ] **Container scanning** (Docker images)
- [ ] **Deployment automation** (zero-downtime deployments)

### Monitoring & Alerts
- [ ] **Service-level objectives (SLOs)** defined
- [ ] **Service-level indicators (SLIs)** tracked
- [ ] **Error budgets** established
- [ ] **On-call runbooks** documented
- [ ] **Alert fatigue prevention** (alert tuning, actionable alerts only)
- [ ] **Dashboards** for key metrics

---

## 9. Data Management

### Database Schema
- [ ] **Migrations versioned** (goose, migrate)
- [ ] **Rollback capability** for migrations
- [ ] **Schema changes backward compatible**
- [ ] **Foreign key constraints** where appropriate
- [ ] **NOT NULL constraints** on required fields
- [ ] **Default values** for columns
- [ ] **Soft delete** instead of hard delete (deleted_at)

### Data Integrity
- [ ] **Idempotency keys** for critical operations
- [ ] **Unique constraints** on natural keys
- [ ] **Optimistic locking** for concurrent updates (version columns)
- [ ] **Transaction boundaries** appropriate
- [ ] **Eventual consistency** acceptable or strong consistency required?
- [ ] **Data validation** in application layer (not just DB constraints)

---

## 10. Specific Application Concerns

### Payment Processing
- [ ] **Idempotent payment operations** (no duplicate charges)
- [ ] **Transaction state machine** validated
- [ ] **Partial captures** handled correctly
- [ ] **Refunds and voids** properly implemented
- [ ] **Settlement reconciliation** process
- [ ] **Chargeback handling** workflow

### Webhooks
- [ ] **Webhook signature verification** (HMAC)
- [ ] **Webhook retry logic** with exponential backoff
- [ ] **Webhook delivery concurrency** (parallel processing)
- [ ] **Webhook timeout** configured (5-10 seconds)
- [ ] **Dead letter queue** for failed webhooks
- [ ] **Webhook event deduplication**

### Subscription Billing
- [ ] **Recurring billing cron jobs** idempotent
- [ ] **Payment method verification** before billing
- [ ] **Grace periods** for failed payments
- [ ] **Dunning management** (retry failed payments)
- [ ] **Subscription state transitions** validated

---

## Review Checklist Usage

### Pre-Production Review
1. Review ALL items in this checklist
2. Mark items as complete or N/A
3. Document any exceptions with justification
4. Ensure P0 security items are 100% complete
5. Create tickets for incomplete P1/P2 items

### Quarterly Review
- Re-review this checklist for new features
- Update based on new learnings
- Check for technical debt accumulation
- Validate performance metrics still meet SLOs

### Post-Incident Review
- Check which checklist items were related to incident
- Update checklist if new patterns discovered
- Add new items based on root cause

---

## Priority Levels

**P0 (Critical - Must Fix Before Production)**
- Security vulnerabilities
- Data loss risks
- Service availability risks
- PCI compliance violations

**P1 (High - Should Fix Soon)**
- Performance issues at scale
- Poor observability
- Scalability bottlenecks
- Technical debt impeding development

**P2 (Medium - Plan to Fix)**
- Code quality improvements
- Documentation gaps
- Testing gaps
- Nice-to-have optimizations

**P3 (Low - Optional)**
- Minor refactoring
- Cosmetic improvements
- Experimental features

---

## References

This checklist was derived from analysis of:
- ARCHITECTURE_RECOMMENDATIONS.md
- CACHING_STRATEGY.md
- DATABASE_OPTIMIZATION.md
- LOGGING_TRACING_OPTIMIZATIONS.md
- MEMORY_OPTIMIZATIONS.md
- RESILIENCE_PATTERNS.md
- SECURITY_SCALING_ANALYSIS.md

**Version**: 1.0
**Maintainer**: Development Team
**Next Review**: Quarterly or post major release
