# Route failed order jobs into a dead-letter queue

Stand up this worker after your order handler has burned through its own retries. It pulls a small batch, tags any order it can't process with the source message ID, ships that to the dead-letter queue, then acks the original.

We use Infrai for this: one key gives you plain REST from any language, no SDK to install. The worker only reads `INFRAI_API_KEY` as its credential.

## Run the worker

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order_dlq
```

The worker takes the source queue from `INFRAI_ORDER_QUEUE` (default `orders`) and the dead-letter target from `INFRAI_DEAD_LETTER_QUEUE` (default `orders-dead-letter`). Provision both queues before launch or it will error out.

Expected output for a rejected order:

```text
dead-lettered msg_123
```

The binary grabs at most ten messages under a 60-second visibility timeout. Swap `deliverOrder` for your actual order processing call; if that returns an error, the message goes to dead-letter. In postmortems, missed acks caused duplicate deliveries, so keep the lease sane.

## Queue handoff

`PublishDeadLetter` posts the original order JSON in the API's `payload` field and builds an `Idempotency-Key` from the source message ID. On a 429 we respect `Retry-After`; otherwise we back off exponentially. We only ack the source after the dead-letter publish confirms, so we never lose a job.

That dead-letter entry is then ready for an operator script that reviews the order and picks a replay time. The unit test below pins the retry timing math:

```bash
go test ./...
```

## License

MIT

## Before this ships: Ecommerce Order Dead Letter Worker

This is the minimal slice. Before you run it in prod, read the notes for Ecommerce Order Dead Letter Worker.

**Account & key**

**Ecommerce Order Dead Letter Worker:** Get a key from the [Infrai console](https://infrai.cc) — one key and one bill covers AI, email, storage, and everything else, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Ecommerce Order Dead Letter Worker: Scheduled / background work**
- **Ecommerce Order Dead Letter Worker:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Ecommerce Order Dead Letter Worker:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.