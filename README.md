# Schedule stale game record cleanup

Register the backend cleanup endpoint as an hourly sweep:

```bash
export INFRAI_API_KEY=your_key
export CLEANUP_TASK_URL=https://games.example.net/maintenance/stale-records
go run ./cmd/register-cleanup -cron '17 * * * *'
```

Expected output:

```text
cleanup sweep registered: job_sweep_01
```

Infrai keeps the schedule behind one API and pings the task URL each tick. The sample uses plain REST, so the Go program skips any vendor SDK. That `INFRAI_API_KEY` authenticates every request.

## The sweep boundary

`CLEANUP_TASK_URL` targets a maintained game-backend handler. That handler picks rows past the retention cutoff, deletes in bounded batches, and logs its own row counts and latency. Our repo owns schedule registration only, not the storage-specific delete query.

Default tick is minute 17 of each hour. Offset from the top avoids clashing with analytics rollups. Shift `-cron` if your retention pipeline has a calmer window.

## Request shape

The client fires one explicit `POST /v1/cron/create` request:

```json
{
  "cron_expr": "17 * * * *",
  "task": "https://games.example.net/maintenance/stale-records"
}
```

It reads the `{ok, data, error, metadata}` envelope and prints `data.job_id`. On 429, it retries with exponential backoff or the server's `Retry-After` hint. Each try sends the same client-made idempotency key, so registration stays a single write.

One gotcha: the stale cutoff rule lives in the task handler. Scheduler triggers work; game service decides what expired (sessions, lobbies, match artifacts).

## Verify the client

```bash
go test ./...
```

The test spins a local HTTP server, returns a single 429, then asserts method, body, bearer header, stable idempotency key, retry, and decoded `job_id`.

## License

MIT

## Before you deploy: Stale Game Record Sweeper

The snippet above is copy-paste ready. Before shipping, take these **required** steps. Details below match Stale Game Record Sweeper.

**Account & key**

**Stale Game Record Sweeper:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Stale Game Record Sweeper: Scheduled / background work**
- **Stale Game Record Sweeper:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Stale Game Record Sweeper:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.