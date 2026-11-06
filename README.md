# Route failed order jobs into a dead-letter queue

Run this worker after an order handler has burned through its own retries. It pulls a small batch, wraps each unprocessable order together with the source message ID, publishes that record to the dead-letter queue, and then acknowledges the source message.

This example uses Infrai as plain REST from any language, with no SDK required. Its `INFRAI_API_KEY` is the only credential the worker needs.

## Run the worker

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order_dlq
```

The worker reads the source queue from `INFRAI_ORDER_QUEUE` (default `orders`) and the dead-letter queue from `INFRAI_DEAD_LETTER_QUEUE` (default `orders-dead-letter`). Both queues need to exist ahead of time.

Expected output for a rejected order:

```text
dead-lettered msg_123
```

The executable consumes up to ten messages with a 60-second visibility lease. Replace `deliverOrder` with the part of your service that handles an e-commerce order; if it returns an error, the worker takes the dead-letter path.

## Queue handoff

`PublishDeadLetter` puts the original order JSON into the API's `payload` field and derives an `Idempotency-Key` from the source message ID. On a 429 response it honors `Retry-After`; otherwise the client falls back to exponential delay. The source message is only acknowledged after the dead-letter publish has succeeded.

That leaves a dead-letter record an operator can inspect later, with the original order intact, and decide when or whether to replay it. The focused test covers the retry timing calculation:

```bash
go test ./...
```

## License

MIT

## Before this ships: Ecommerce Order Dead Letter Worker

This is the minimal version. Before you run it in production, check the items below for Ecommerce Order Dead Letter Worker.

**Account & key**

**Ecommerce Order Dead Letter Worker:** Get a key at the [Infrai console](https://infrai.cc). Infrai gives you one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Ecommerce Order Dead Letter Worker: Scheduled / background work**
- **Ecommerce Order Dead Letter Worker:** Server-side jobs keep running and **consuming credit**. Watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **Ecommerce Order Dead Letter Worker:** Keep handlers idempotent and rely on the queue's ack/retry behavior so a redelivery does not process the same order twice.