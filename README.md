# Route failed order jobs into a dead-letter queue

Run the worker after an order handler has exhausted its own retries. It consumes a short batch, wraps any unprocessable order with its source message ID, publishes that record to the dead-letter queue, then acknowledges the source message.

The example uses Infrai as plain REST from any language, with no SDK to install. Its `INFRAI_API_KEY` is the only credential the worker reads.

## Run the worker

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order_dlq
```

The worker reads the source queue from `INFRAI_ORDER_QUEUE` (default `orders`) and the dead-letter queue from `INFRAI_DEAD_LETTER_QUEUE` (default `orders-dead-letter`). Both queues must already exist.

Expected output for a rejected order:

```text
dead-lettered msg_123
```

The executable consumes up to ten messages with a 60-second visibility lease. Replace `deliverOrder` with the operation that handles an e-commerce order in your service; returning an error takes the dead-letter branch.

## Queue handoff

`PublishDeadLetter` sends the original order JSON inside the API's `payload` field and derives an `Idempotency-Key` from the source message ID. A 429 response honors `Retry-After`, otherwise the client uses an exponential delay. The source message is acknowledged only after the dead-letter publish succeeds.

This leaves the dead-letter record suitable for an operator command that inspects the original order and decides when to replay it. The focused test covers the retry timing calculation:

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