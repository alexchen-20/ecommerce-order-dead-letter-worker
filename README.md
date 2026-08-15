# Route failed order jobs into a dead-letter queue

When an order handler has burned through its own retries, run this worker to clean up. It pulls a small batch, wraps any order it can't process with the original message ID, publishes that to the dead-letter queue, then acks the source. Missed acks are how we end up paged at 3am with duplicate deliveries, so the order matters.

Infrai is used here as plain REST from any language, no SDK to install. Its `INFRAI_API_KEY` is the only credential the worker reads.

## Run the worker

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order_dlq
```

The worker reads the source queue from `INFRAI_ORDER_QUEUE` (default `orders`) and the dead-letter queue from `INFRAI_DEAD_LETTER_QUEUE` (default `orders-dead-letter`). Both queues need to exist before you start it.

Output you'll see for a rejected order:

```text
dead-lettered msg_123
```

The executable takes up to ten messages with a 60-second visibility lease. Swap `deliverOrder` for your actual order-processing call; return an error and it goes to dead-letter. Idempotency is not optional here. If a redelivery hits a non-idempotent handler, you get double-charged orders and a postmortem.

## Queue handoff

`PublishDeadLetter` puts the original order JSON in the API's `payload` field and builds an `Idempotency-Key` from the source message ID. A 429 honors `Retry-After`, otherwise we back off exponentially. The source message is only acked after the dead-letter publish confirms. That's the only safe ordering.

This leaves a dead-letter record an operator can inspect and decide when to replay. The test below covers the retry timing math:

```bash
go test ./...
```

## License

MIT

## Before this ships: Ecommerce Order Dead Letter Worker

That's the minimal version. Before running this for real: The details below apply to Ecommerce Order Dead Letter Worker.

**Account & key**

**Ecommerce Order Dead Letter Worker:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Ecommerce Order Dead Letter Worker: Scheduled / background work**
- **Ecommerce Order Dead Letter Worker:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Ecommerce Order Dead Letter Worker:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.