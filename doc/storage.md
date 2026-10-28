# Disk budgets and project configuration

xgo uses Cobra for its command line and Viper for configuration. The runtime
module remains dependency-free. Go flags and program arguments retain their
existing passthrough behavior, including `test -args` and `run . <arguments>`.

Defaults on macOS and Linux:

| Setting | Default | Flag | Environment |
| --- | --- | --- | --- |
| Managed storage budget | 20 GiB | `--cache-max-size` | `XGO_CACHE_MAX_SIZE` |
| Free space reserve, on each managed volume | 10 GiB | `--cache-min-free` | `XGO_CACHE_MIN_FREE` |
| Abandoned session retention | 24 hours | `--temp-max-age` | `XGO_TEMP_MAX_AGE` |
| Guard polling interval | 2 seconds | `--disk-check-interval` | `XGO_DISK_CHECK_INTERVAL` |
| Cache/session root | `/tmp/xgo` | `--cache-dir` | `XGO_CACHE_DIR` |
| Explicit configuration file | nearest `.xgo.yaml` | `--config` | `XGO_CONFIG` |

Byte sizes accept integers and `KiB`, `MiB`, `GiB`, `KB`, `MB`, `GB`, or `B`.
The budget must be positive. An explicit zero reserve disables only the free
space reserve; the byte budget remains enforced. Poll intervals must be between
100 ms and one minute. Abandoned retention must be at least one minute.

Place a `.xgo.yaml` in the project:

```yaml
cache-max-size: 20GiB
cache-min-free: 10GiB
temp-max-age: 24h
disk-check-interval: 2s
# Optional. Relative paths resolve from the configuration file's directory.
# cache-dir: .xgo/cache
```

Precedence is explicit CLI flags, environment, project file, defaults.
`XGO_FLAGS` is still supported; explicit command arguments take precedence over
its values. Discovery begins at `--project-dir` (or the working directory),
walks upward to the nearest module boundary, and uses the closest `.xgo.yaml`.
An explicit `--config`/`XGO_CONFIG` can select Viper-supported YAML, TOML or JSON
files; relative explicit file names resolve from the working directory. An
explicit missing or malformed file, unknown key or invalid value is an error.

```sh
xgo test --cache-max-size 12GiB --cache-min-free 8GiB ./...
XGO_CACHE_MAX_SIZE=12GiB xgo test ./...
xgo cache status
xgo cache gc
xgo --config ./ci.yaml cache status
```

`cache status` is read-only and prints JSON with effective policy and usage.
`cache gc` reclaims abandoned sessions, then evicts unused managed directories
in last-use order as needed for the budget/reserve. It fails if protected work
prevents reclamation. Existing age/count-based cache collection remains active.

The budget counts the selected cache/session root, xgo-managed toolchains under
the selected `--xgo-home` (default `~/.xgo`), and namespaces beneath
`XGO_BUILD_CACHE_DIR` when set. It includes both normal and development caches.
It counts logical file sizes conservatively, without following symlinks; APFS
clones and sparse files can occupy less physical storage than the reported
usage. Explicit overlapping storage roots are rejected. Separate configured
root sets have separate budgets.

Before starting work, xgo checks the budget and each volume's free space. A
supervisor continues checking through toolchain setup, instrumentation, and
the Go build/test/run/exec command. Free-space checks run at least every 500 ms
(or the shorter configured interval), independently of slow usage scans.
Mutable cache sizes are sampled at most five seconds apart; immutable published
toolchain sizes are reused until their publication identity changes. Temporary
sessions are always measured live. Accounting snapshots amortize scans across
warm CLI invocations. Every invocation gets a leased session;
`TMPDIR`, `TMP`, `TEMP` and `GOTMPDIR` point there. Setup commands and child Go
commands inherit cache leases. A killed wrapper therefore does not make its
live child's storage eligible for deletion. Session cleanup also checks the
recorded process group in case a compiler outlives its parent.

When reclamation cannot satisfy the budget/reserve, xgo refuses new work or
stops its own process group and returns an error. It never terminates another
job to reclaim storage. Interrupting xgo also stops its supervised job. A
successful command stops any background descendants before session cleanup.
Interactive terminal input is forwarded to the job's process group.

This is a polling guard, not a filesystem quota: writes between checks/accounting refreshes, a slow
directory scan, or unrelated processes can temporarily overshoot the budget or
consume the reserve. Use filesystem/container quotas if a strict byte ceiling
is required. The reserve is a buffer for these in-flight writes. The guard
does not cover arbitrary build outputs outside managed directories, the native
Go cache used to compile xgo itself, or other programs' caches.

Legacy `/tmp`/macOS `go-build*`, `go-link*`, and project `.xgo/gen/tmp` directories
have no reliable lease and are left alone. Old xgo cache directories without a
pre-existing ownership lock are also preserved: age alone cannot establish
that an older xgo job is finished. These may require an explicit manual audit.
Only future managed sessions are automatically reclaimable. Symlink targets,
unrelated directory names and other users' storage are never reclaimed.

The free-space guard, process-group termination and inherited leases are
supported and tested on macOS and Linux. Other platforms retain cache policy
parsing/counting, but do not provide the same disk/process safety guarantees.
