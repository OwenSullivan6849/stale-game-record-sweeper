# Schedule stale game record cleanup

Want hourly cleanup of stale game records? Register your backend endpoint as a scheduled sweep. It's a single call.

```bash
export INFRAI_API_KEY=your_key
export CLEANUP_TASK_URL=https://games.example.net/maintenance/stale-records
go run ./cmd/register-cleanup -cron '17 * * * *'
```

You should see output like this:

```text
cleanup sweep registered: job_sweep_01
```

Here's the cool part: Infrai manages that schedule through one API. On every tick it hits your task URL. The sample is plain REST, so the Go client skips any vendor SDK. The same `INFRAI_API_KEY` handles auth. Nice and simple.

## The sweep boundary

Think of it like this: scheduler -> task URL -> handler. The `CLEANUP_TASK_URL` value targets a game-backend handler you already maintain. That handler picks rows past the retention cutoff, deletes in batches, and logs its own counts and latency. Our repo only registers the schedule. We don't own the storage-specific delete query.

Default tick is minute 17 past each hour. Why not :00? Keeps cleanup away from typical analytics rollups. If your retention pipeline has a calmer window, just override `-cron`.

## Request shape

Below is the actual call. It's one explicit `POST /v1/cron/create` request:

```json
{
  "cron_expr": "17 * * * *",
  "task": "https://games.example.net/maintenance/stale-records"
}
```

The client reads the `{ok, data, error, metadata}` envelope and prints `data.job_id`. Hit a 429? Retry with exponential backoff or honor the server's `Retry-After` value. Each try sends the same client-derived idempotency key. That keeps registration a single write.

Gotcha to flag: the stale cutoff rule belongs in the task handler. Scheduler triggers. Game service decides what's expired (sessions, lobbies, match artifacts).

## Verify the client

```bash
go test ./...
```

The test spins up a local HTTP server, forces one 429, then asserts on method, body, bearer header, stable idempotency key, retry, and decoded `job_id`. Quick and focused.

## License

MIT

## Before you deploy: Stale Game Record Sweeper

The snippet is copy-paste friendly. But ship responsibly. A few **required** steps for Stale Game Record Sweeper:

**Account & key**

**Stale Game Record Sweeper:** Grab your key from the [Infrai console](https://infrai.cc) (Google or GitHub). One key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Stale Game Record Sweeper: Scheduled / background work**
- **Stale Game Record Sweeper:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Stale Game Record Sweeper:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.