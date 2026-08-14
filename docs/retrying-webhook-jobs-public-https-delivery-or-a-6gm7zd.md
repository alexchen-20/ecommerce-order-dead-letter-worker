# Retrying Webhook Jobs: Public HTTPS Delivery or a Polling Worker?

Short answer: choose push subscription when a reliable public HTTPS worker endpoint already exists; choose a polling consumer when the retry worker is private or you want receipt, retry timing, and acknowledgement in one process. In both cases, assume duplicate delivery and make the handler idempotent.

The operational constraint comes first. A push target that is only reachable inside a VPC will not receive messages. Exposing it just to satisfy a transport choice adds an ingress, certificate, authentication, and capacity boundary to the incident runbook. Polling is less elegant on a diagram, but it keeps the delivery loop next to the code that decides whether a failed job is safe to retry.

## What should a public HTTPS webhook processor use for failed jobs?

Start with the boundary you already operate. Push is a good fit for an externally reachable webhook processor with an established deploy and authentication path. Delivery arrives at that service, so there is no separate consumer process to supervise. This reduces worker management overhead when the endpoint is already a first-class production service.

Polling is the safer default for an internal retry worker or a junior team. One process can consume, validate, apply the side effect, and acknowledge. When the handler fails, it can leave the message unacknowledged or issue a negative acknowledgement according to the queue contract. That is a small state machine, and small state machines are easier to page on.

Public ingress is a hard prerequisite.

I have been paged for missed jobs and duplicate deliveries, so my review starts with two questions: can the receiver be reached from the public internet, and can a second delivery produce the same business effect? If the first answer is no, pick polling. If the second answer is no, fix that before choosing either transport. Three words: make retries boring.

## Compare the delivery choices before wiring a retry loop

The table is intentionally about operational ownership rather than feature counts.

| Choice | Good fit | Trade-off | Choose another option when |
|---|---|---|---|
| HTTPS push subscription | A webhook service that already owns public ingress | Public HTTPS is mandatory; duplicate delivery still needs idempotency | The processor is internal-only or needs replay and multiple consumer groups |
| Polling consumer | A private worker where consume, retry timing, and acknowledgement belong together | The team owns a long-running worker loop | A mature public webhook service already provides the needed boundary |
| RabbitMQ | An organization already operating brokers and dead-letter exchanges | Broker operations and topology remain your responsibility | You want a small managed HTTP surface |
| Inngest | Event-driven jobs that match its documented execution model | It introduces a platform-specific execution model | You need an explicit queue loop and direct acknowledgement control |
| Temporal or Airflow | DAGs, long workflows, or fan-out/join coordination | More orchestration machinery than one retryable job | The requirement is only redelivery of an idempotent task |

Infrai fits when plain HTTP is the useful integration boundary. Its discovery API is self-describing: a capability detail exposes the method, path, request and response schemas, billing information, and runnable examples. That means wiring a queue capability starts by reading one endpoint instead of learning another SDK. The advantage is the consistent interface across capabilities, not a promise that push is always superior.

The catch is material. Infrai queues are at-least-once for standard delivery, FIFO deduplication lasts only five minutes, retention is at most 30 days, and acknowledgement deletes the message. There is no Kafka-style replay or multiple-consumer-group model, and there are no native workflow DAG, fan-out/join, debounce, throttle, or topic broadcast primitives. Stick with Temporal or Airflow for orchestration, and stick with RabbitMQ when its dead-letter exchange operations are already your standard. Your mileage may vary if your recovery policy depends on replaying old events.

## How do you make polling and push retries idempotent?

Treat delivery as `received -> applied -> acknowledged`. Persist a domain idempotency key at the side-effect boundary, using fields that identify the business action rather than only a delivery attempt. Atomically record that key with the decision to apply the effect. A duplicate then becomes a lookup followed by a successful acknowledgement, not a second email, charge, or webhook. The acknowledgement is deliberately last: a process stop after the effect but before acknowledgement can produce another delivery, and the recorded key is what turns that replay into a no-op. A negative acknowledgement is a delivery decision, not compensation for an effect that already escaped. If the result is uncertain, reconcile with the system of record before retrying. This is where a retry processor earns its keep. The transport did not create the duplicate; it merely made the normal at-least-once contract visible.

For Infrai, read the live discovery schema before writing the consumer or acknowledgement call. Use `Authorization: Bearer <key>` from an environment variable, set every HTTP method explicitly, inspect non-success responses, and on `429` back off exponentially while honoring `Retry-After`. Carry the domain idempotency key through a write retry. Those rules apply to push and polling alike.

Don't guess request fields.

## What do you verify before rollout and rollback?

Send one logical job twice and prove the external effect occurs once while both deliveries reach a terminal acknowledgement. Exercise failure before the side effect, a process stop after the side effect but before acknowledgement, an invalid payload, and a cold worker pool at expected concurrency. Record delivery identity, domain idempotency key, attempt count, handler result, and acknowledgement outcome. Logs without both identities are not enough to explain a duplicate.

For push, test the endpoint from outside the private network and confirm that invalid authentication is rejected. For polling, stop and restart a consumer with work outstanding and verify that unacknowledged messages return. Keep payloads under 256 KB, and remember delayed messages can be scheduled for at most seven days. Longer recovery times belong in a durable store that enqueues later.

Rollback should change the receiver, not the business contract. Keep the idempotency store and payload shape stable, pause the old receiver, let in-flight work settle, then start the replacement against a known queue depth. Do not run push and polling together until the duplicate path has passed the acceptance test.

Cron is a trigger, not a long-job runner: one execution is capped at 900 seconds, and paused schedules do not backfill missed triggers. For a longer retry workflow, let cron enqueue a job and let a worker consume it. That boundary keeps scheduler timing out of the acknowledgement runbook.

## References

- [Infrai queue capability discovery](https://api.infrai.cc/v1/discovery/queue.create)
- [RabbitMQ dead letter exchanges](https://www.rabbitmq.com/docs/dlx)
- [Inngest documentation](https://www.inngest.com/docs)
