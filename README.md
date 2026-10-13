# netcheck

A small concurrent TCP port and network health checker, written in Go.

Give it a list of `host:port` targets. It checks all of them at the same time and reports whether each one is UP or DOWN, along with the connection latency. Output is a table by default or JSON on request, and the exit code tells scripts whether anything failed.

## Why I built it

I wanted to learn Go by building a real tool instead of following a tutorial. Checking whether services are reachable is something I do all the time when troubleshooting, so it made a good first project.

## What it shows

- **Goroutines**: one per target, so 3 targets or 300 take about as long as the slowest one
- **Channels**: results are collected safely from the goroutines
- **sync.WaitGroup**: closes the results channel once every check has finished
- **net.DialTimeout**: the actual TCP reachability check
- **Exit codes**: 0 if all targets are UP, 1 if any target is DOWN
- **JSON output**: a separate output struct so latency prints as readable milliseconds

## Usage

Check the default targets:

```
go run main.go
```

Check your own list:

```
go run main.go -targets="google.com:443,github.com:443,localhost:9999"
```

Change the timeout for each check (default 2s):

```
go run main.go -timeout=500ms
```

Print JSON instead of a table:

```
go run main.go -json
```

Or build a binary:

```
go build -o netcheck .
./netcheck -targets="github.com:443"
```

## Example output

Table (latency values will vary):

```
TARGET               STATUS   LATENCY   ERROR
------               ------   -------   -----
github.com:443        UP         88ms
google.com:443        UP         88ms
localhost:9999        DOWN        2ms   dial tcp [::1]:9999: connect: connection refused
```

JSON:

```json
[
  {
    "target": "github.com:443",
    "up": true,
    "latency_ms": 56.51
  },
  {
    "target": "localhost:9999",
    "up": false,
    "latency_ms": 4.99,
    "error": "dial tcp [::1]:9999: connect: connection refused"
  }
]
```

## Exit codes

| Code | Meaning |
| ---- | ------- |
| 0 | Every target is reachable |
| 1 | At least one target is down, or JSON output failed |

This makes netcheck usable as a step in a shell script or CI pipeline, where the exit code decides pass or fail:

```
./netcheck -targets="db.internal:5432,cache.internal:6379" || echo "a dependency is down"
```

## Real world uses

- Confirm a deployment's dependencies (database, cache, APIs) are reachable
- Narrow down which host or port is the problem during an incident
- Gate a CI step on required services being up
- Feed JSON results into other scripts or monitoring

## Notes

- This checks that a TCP connection can be opened. It does not check that the service behind the port is healthy.
- Results were cross checked against `curl` and `nc`.
