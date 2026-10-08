# Traffic storage v1

The control database contains only rules, groups, policies and risk events. History
is disposable: `history.directory` defaults to `traffic-history-v1` beside the control
file. Retention is fixed at 6 hours of minutes, 7 days of ten-minute buckets and
30 days of hourly buckets, subject to the 2GiB budget and 2GiB disk reserve.
`monitor.history_interval` is 60 (enabled) or 0 (disabled).

Each minute is collected from packet increments, independently of live-counter
resets and IP eviction. A minute transaction publishes six pre-aggregated dimensions,
a unique batch ledger and coverage together. Retry batches retain their ID. Capture
and retry buffers are bounded to 32MiB each; overflow is recorded as a data gap.
A clean shutdown flushes closed minutes; the current partial minute is marked as a
process gap on restart. Ten-minute and hourly coverage are committed atomically with
their rolled metrics. Retention never normally deletes a source before its target
coverage exists. Capacity eviction may shorten history and is visible in status.

Files are `v1-<resolution>-<shard-start>.sqlite`. Minute shards span one hour;
ten-minute/hourly shards span one UTC day. Open-file cache limit: eight databases,
two connections each. Four history queries may execute at once, with a three-second
SQL deadline. Control database has a separate four-connection pool. Deletion waits
for readers, closes pooled connections and removes the main file, WAL and SHM.

Queries use actual coverage, coarsest available buckets compatible with requested
precision, and fine data for uncovered recent periods. No segment is counted twice.
Absent coverage is a gap; committed empty coverage is zero traffic. Bucket size is
rounded up to a multiple of the selected tier and bounded to 720 buckets. Returned
`meta.start/end` describe the aligned actual interval, `bucket` the actual precision,
`freshness` the latest sampled time, and `gaps` unavailable or incomplete intervals.
Top lists aggregate complete dimension totals before sorting and limiting. Atomic
per-shard totals accelerate complete shards; only partial edges scan time buckets.

## HTTP contract (breaking)

All routes remain authenticated. Standard `{code,message,data}` envelope remains.

- `GET /api/traffic?page=0&page_size=25&sort=bytes_out_per_sec&order=desc&remote_ip=&local_ip=`:
  `data={items,total,snapshot_id}`. Zero-based pages, maximum 100 rows. No port or
  protocol details in list items. Unsupported sort/order is a 400.
- `GET /api/traffic/overview`: global counts/rates, protocol totals, snapshot ID.
- `GET /api/traffic/ip/<ip>`: one live IP with protocol and top-eight port detail.
- `GET /api/traffic/ports`: independent five-second port snapshot, even when
  the policy engine is disabled.
- Existing `/api/traffic/history`, `/history/top`, `/history/ports`,
  `/history/ports/top`, `/history/protocols`: `data={items,meta}`. Unix-second
  start/end, bucket, optional IP/protocol/port. Port -1 means all ports; 0 is valid.
  Maximum 30-day span, 100 Top entries. Port curves select ten series plus `other/-1`.
- `GET /api/traffic/storage`: sizes (including WAL), budget, reserve, available
  range, shard watermarks, pause/error/failure status and bounded gap/event records.

Same historical requests coalesce and cache for 30 seconds (128 entries maximum).
Cancellation propagates to the owner SQL request; waiting requests may cancel
independently. Live snapshots refresh every five seconds. UI polls live data every
15 seconds, history and storage every 60 seconds, cancels superseded requests and
retains prior successful data on background errors. History precision/gaps are shown.

## Upgrade and rollback

1. Build/tag the release through the existing Gitea workflow. Linux race tests run
   inside the build before an image can be pushed. Never upload an image from Mac.
2. Record container image, host networking, capabilities, restart policy and mounts.
   Stop the old instance before final backup. Export control data with
   `python3 scripts/migrate_control.py --export /etc/netbouncer/netbouncer.db`.
   Stream the export, config and container metadata to a private off-host directory.
3. Restore the export to a new local file and verify table values, SHA-256 and
   `PRAGMA integrity_check`. Do not copy or VACUUM the 2GiB legacy SQLite database.
4. After verifying the off-host backup, restore it on the server as
   `/etc/netbouncer/control-v1.sqlite`; set `database.database` to that path,
   `database.dsn` to empty, and `database.log_level` to `warn`. Set the history path
   explicitly to `/etc/netbouncer/traffic-history-v1` and its budget/reserve to 2147483648.
5. Recreate only netbouncer with its original host network, NET_ADMIN/NET_RAW,
   config mount and restart policy, adding `--log-opt max-size=10m --log-opt max-file=3`.
   Removing the stopped old container reclaims its unmanaged JSON log. Delete the old
   traffic DB/WAL/SHM and legacy traffic backup after control-data verification.
6. Confirm unauthenticated requests are still 401, OIDC discovery/login work, the
   firewall has all preserved rules, minute coverage advances and storage has free room.
   Run `./netbouncer benchmark` inside the new container: it creates and removes an
   isolated synthetic fixture (10,000 IPs, 9,000 rules, >7M history rows), tests real
   authenticated loopback HTTP routes at 20 concurrent readers with concurrent sampling,
   and reports cold/p95 latency with acceptance budgets.
7. Observe memory, disk growth, writer errors, rollup coverage and rotated logs for
   24 hours. Notify only on a meaningful change or a required action.

Rollback recreates the original image with original config and the verified compact
control backup. Old traffic history is intentionally not recovered. Retain the
original image and off-host backup until the observation window passes.

SQL batch contents and literal queries are never logged. Capture buffers/current
shards exceeding the capacity reserve cause sampling pause rather than disk exhaustion;
real-time monitoring and control data remain operational.
