# Webhook notifications

paperless-gpt can POST a notification to an external HTTP endpoint (e.g. an
[n8n](https://n8n.io) webhook node) every time it finishes processing a
document. This lets you trigger downstream automations — email, Slack, archive
moves, indexing — without polling Paperless-NGX.

## Enabling

Set a single environment variable:

```bash
N8N_WEBHOOK_URL=https://n8n.example.com/webhook/paperless-gpt
```

If the variable is unset or empty, the feature is disabled and no requests are
sent.

## Events

Two event types are emitted, one per workflow. Each is delivered as a `POST`
to a distinct path appended to `N8N_WEBHOOK_URL`:

| Event          | URL                                  | When it fires                                  |
| -------------- | ------------------------------------ | ---------------------------------------------- |
| `auto_tagged`  | `${N8N_WEBHOOK_URL}/auto_tagged`     | After a document finishes auto-tagging (LLM)   |
| `ocr_textract` | `${N8N_WEBHOOK_URL}/ocr_textract`    | After a document finishes OCR via AWS Textract |

This lets you either:

- Point `N8N_WEBHOOK_URL` at a single n8n webhook node and ignore the suffix, or
- Register two separate webhook nodes (one per path) and handle each event
  differently.

## Payload

`Content-Type: application/json`

```json
{
  "schema": 1,
  "event": "auto_tagged",
  "document_id": 1234,
  "title": "Electric bill — March 2026",
  "tags": ["bills", "utilities"],
  "correspondent": "Stadtwerke",
  "document_type": "Invoice",
  "timestamp": "2026-04-12T18:42:11Z"
}
```

Fields:

- `schema` — payload version. Currently `1`. Bumped on breaking changes.
- `event` — `auto_tagged` or `ocr_textract`.
- `document_id` — Paperless-NGX document ID.
- `title`, `tags`, `correspondent`, `document_type` — the final values applied
  to the document. Optional fields are omitted when empty.
- `timestamp` — UTC time the notification was emitted.

## Delivery semantics

The sender is intentionally simple:

- **Fire-and-forget.** Webhook delivery runs in a background goroutine and
  never blocks document processing.
- **Retries.** Up to 3 attempts with exponential backoff (1s, 2s, 4s). Only
  network errors and `5xx` responses are retried; `4xx` responses are treated
  as permanent failures and not retried.
- **Timeout.** 15 seconds per HTTP attempt.
- **Log and drop.** If all retries fail, a warning is logged with the document
  ID and the last error. The event is not persisted anywhere and will not be
  retried later.
- **Circuit breaker.** After 5 consecutive failures the circuit opens for 60
  seconds; during that window new events are skipped (and logged at debug
  level) instead of hammering a broken endpoint. A single success closes the
  circuit.

Because there is no persistence, **delivery is best-effort**. If you need
guaranteed delivery, either put a durable queue in front of your consumer, or
drive the automation off Paperless-NGX's own state (e.g. a tag set by
paperless-gpt) instead of this webhook.

## Setting up the n8n side

1. Create a workflow with a **Webhook** trigger node.
2. Set the HTTP method to `POST` and the path to match the suffix you want
   (e.g. `paperless-gpt/auto_tagged`).
3. Copy the production URL from the node and set it as `N8N_WEBHOOK_URL` on
   paperless-gpt (strip the `/auto_tagged` suffix — paperless-gpt will add it).
4. Activate the workflow and process a document to verify the payload arrives.

## Troubleshooting

- **Nothing arrives.** Confirm `N8N_WEBHOOK_URL` is set in the paperless-gpt
  process environment and that logs don't show `webhook delivery gave up` or
  `webhook circuit opened`.
- **`4xx` errors.** paperless-gpt will not retry. Check the n8n webhook path,
  method, and any auth the node requires — this sender does not send auth
  headers.
- **Circuit stuck open.** It will auto-close 60s after the last failure, on
  the next successful send. Fix the receiver and wait.
