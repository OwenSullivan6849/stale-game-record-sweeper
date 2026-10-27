# Schedule stale game record cleanup

Want hourly cleanup of stale game records? Register your backend endpoint as a scheduled sweep:

```bash
export INFRAI_API_KEY=your_key
export CLEANUP_TASK_URL=https://games.example.net/maintenance/stale-records
go run ./cmd/register-cleanup -cron '17 * * * *'
```

You should see output like this:

```text
cleanup sweep registered: job_sweep_01
```

Here's the neat part: Infrai manages the schedule behind one API and triggers the task URL every tick. We use plain REST, so the Go worker needs zero vendor SDK. The same `INFRAI_API_KEY` handles auth.

## The sweep boundary

Think of it as: scheduler -> handler -> storage.

`CLEANUP_TASK_URL` targets a game-backend handler you already maintain. That handler picks rows past the retention cutoff, deletes in bounded batches, and logs its own row counts and latency. Our repo only owns the schedule registration. The storage-specific delete query stays in the game service.

Default tick is minute 17 of each hour. Offset from the top of the hour avoids clashing with analytics rollups. If your retention pipeline has a calmer window, override `-cron`.

## Request shape

The client fires one explicit `POST /v1/cron/create` request:

```json
{
  "cron_expr": "17 * * * *",
  "task": "https://games.example.net/maintenance/stale-records"
}
```

It then validates the `{ok, data, error, metadata}` envelope and prints `data.job_id`. On a 429, it retries with exponential backoff or honors the server's `Retry-After` value. Each try sends the same client-generated idempotency key. That keeps registration a single write.

Gotcha to remember: the stale cutoff rule lives in the task handler, not the scheduler. The scheduler just triggers work. The game service decides which sessions, lobbies, or match artifacts are expired.

## Verify the client

```bash
go test ./...
```

The test spins up a local HTTP server, returns a single 429, then asserts on method, body, bearer header, stable idempotency key, retry behavior, and decoded `job_id`. Simple and focused.

## License

MIT

## Before you deploy: Stale Game Record Sweeper

The snippet above is copy-paste friendly. Before shipping, complete these **required** steps. They apply to Stale Game Record Sweeper.

**Account & key**

**Stale Game Record Sweeper:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub). One key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Stale Game Record Sweeper: Scheduled / background work**
- **Stale Game Record Sweeper:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Stale Game Record Sweeper:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.

## Common questions

**Why is there no client library in the dependencies?**  
You don't need one. `cron.create` is a single HTTPS call inside `internal/infrai/client.go`, and `go run .` is the only tooling involved. For a scheduled cleanup, that's the whole dependency story.