#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/redis-preflight.sh [options]

Checks an ARL deployment before switching the job queue to Redis Streams.

Options:
  --config PATH              ARL config file (default: config.yaml)
  --binary PATH              ARL binary (default: ./arl)
  --redis-url URL            Override redis.url; defaults to ARL_REDIS_URL or config
  --require-empty            Fail when any arl:jobs:* queue has pending work
  --rabbitmq-vhost VHOST     Check legacy RabbitMQ q.* queues through rabbitmqctl
  --allow-memory             Permit queue.driver=memory (development only)
  --skip-doctor              Skip the `arl doctor` invocation
  -h, --help                 Show this help

The Redis URL is never printed because it may contain credentials.
USAGE
}

config="${ARL_CONFIG:-config.yaml}"
binary="${ARL_BIN:-./arl}"
redis_url_arg=""
rabbitmq_vhost=""
require_empty=0
allow_memory=0
skip_doctor=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      config="${2:?--config requires a path}"
      shift 2
      ;;
    --binary)
      binary="${2:?--binary requires a path}"
      shift 2
      ;;
    --redis-url)
      redis_url_arg="${2:?--redis-url requires a URL}"
      shift 2
      ;;
    --rabbitmq-vhost)
      rabbitmq_vhost="${2:?--rabbitmq-vhost requires a vhost}"
      shift 2
      ;;
    --require-empty)
      require_empty=1
      shift
      ;;
    --allow-memory)
      allow_memory=1
      shift
      ;;
    --skip-doctor)
      skip_doctor=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

warn() {
  echo "WARN: $*" >&2
}

yaml_value() {
  local section="$1" key="$2" raw
  raw="$(
    awk -v section="$section" -v key="$key" '
      /^[[:space:]]*#/ { next }
      /^[^[:space:]][^:]*:/ {
        name = $1
        sub(/:.*/, "", name)
        in_section = (name == section)
        next
      }
      in_section {
        line = $0
        sub(/^[[:space:]]+/, "", line)
        name = line
        sub(/:.*/, "", name)
        if (name == key) {
          value = line
          sub(/^[^:]*:[[:space:]]*/, "", value)
          sub(/[[:space:]]+#.*$/, "", value)
          gsub(/\r/, "", value)
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
          print value
          exit
        }
      }
    ' "$config"
  )"
  raw="${raw%\"}"
  raw="${raw#\"}"
  raw="${raw%\'}"
  raw="${raw#\'}"
  printf '%s\n' "$raw"
}

[[ -f "$config" ]] || fail "config not found: $config"
[[ -x "$binary" ]] || fail "ARL binary not executable: $binary (run make build first)"

if grep -Eq '^[[:space:]]*rabbitmq:' "$config"; then
  fail "legacy rabbitmq block still exists in $config"
fi

driver="${ARL_QUEUE_DRIVER:-$(yaml_value queue driver)}"
[[ -n "$driver" ]] || driver="memory"

case "$driver" in
  redis) ;;
  memory)
    if [[ "$allow_memory" -ne 1 ]]; then
      fail "queue.driver=memory; use --allow-memory only for local development"
    fi
    warn "queue.driver=memory; Redis checks will be skipped"
    ;;
  *)
    fail "unsupported queue.driver=$driver; expected redis or memory"
    ;;
esac

if [[ "$driver" == "redis" ]]; then
  redis_url="${redis_url_arg:-${ARL_REDIS_URL:-$(yaml_value redis url)}}"
  [[ -n "$redis_url" ]] || fail "redis.url is empty; set redis.url, ARL_REDIS_URL, or --redis-url"
  command -v redis-cli >/dev/null 2>&1 || fail "redis-cli is required (apt install redis-tools)"

  redis_cmd() {
    redis-cli -u "$redis_url" --raw "$@" 2>/dev/null
  }

  echo "[1/6] Redis connectivity"
  pong="$(redis_cmd PING || true)"
  [[ "$pong" == "PONG" ]] || fail "Redis PING failed; check redis.url, credentials, TLS, and network access"
  echo "PASS: Redis PING"

  echo "[2/6] Redis version and persistence"
  server_info="$(redis_cmd INFO server || true)"
  redis_version="$(awk -F: '/^redis_version:/{gsub(/\r/, "", $2); print $2}' <<<"$server_info")"
  [[ -n "$redis_version" ]] || fail "unable to read redis_version from INFO server"
  if [[ "$(printf '%s\n%s\n' "7.0" "$redis_version" | sort -V | head -n1)" != "7.0" ]]; then
    fail "Redis $redis_version is too old; require >= 7.0"
  fi

  appendonly="$(redis_cmd CONFIG GET appendonly | tail -n1 || true)"
  appendfsync="$(redis_cmd CONFIG GET appendfsync | tail -n1 || true)"
  maxmemory_policy="$(redis_cmd CONFIG GET maxmemory-policy | tail -n1 || true)"
  [[ -n "$appendonly" ]] || fail "unable to read Redis appendonly; ACL may block CONFIG"
  [[ -n "$appendfsync" ]] || fail "unable to read Redis appendfsync; ACL may block CONFIG"
  [[ -n "$maxmemory_policy" ]] || fail "unable to read Redis maxmemory-policy; ACL may block CONFIG"
  [[ "$appendonly" == "yes" ]] || fail "Redis appendonly is not enabled"
  case "$appendfsync" in
    everysec|always) ;;
    *) fail "Redis appendfsync must be everysec or always (got $appendfsync)" ;;
  esac
  [[ "$maxmemory_policy" == "noeviction" ]] || fail "Redis maxmemory-policy must be noeviction (got $maxmemory_policy)"
  echo "PASS: Redis $redis_version, appendonly=yes, appendfsync=$appendfsync, maxmemory-policy=noeviction"

  echo "[3/6] Redis write permission"
  probe_key="arl:preflight:$(date +%s)-$$"
  [[ "$(redis_cmd SET "$probe_key" 1 EX 30 || true)" == "OK" ]] || fail "Redis write probe failed; check ACL/write permissions and maxmemory"
  redis_cmd DEL "$probe_key" >/dev/null 2>&1 || true
  echo "PASS: Redis write/delete"

  echo "[4/6] Queue depth"
  queues=(
    q.discovery.network
    q.discovery.dns
    q.web.fetch
    q.web.scan
    q.poc.http
    q.poc.service
    q.monitor
    q.github
    q.maintenance
    q.icp
  )
  total=0
  nonempty=()
  for queue in "${queues[@]}"; do
    stream="arl:jobs:$queue"
    main="$(redis_cmd XLEN "$stream" || true)"
    retry="$(redis_cmd ZCARD "$stream:retry" || true)"
    dlq="$(redis_cmd XLEN "$stream:dlq" || true)"
    [[ "$main" =~ ^[0-9]+$ && "$retry" =~ ^[0-9]+$ && "$dlq" =~ ^[0-9]+$ ]] || fail "unable to read queue depth for $queue"
    count=$((main + retry + dlq))
    total=$((total + count))
    if [[ "$count" -gt 0 ]]; then
      nonempty+=("$queue main=$main retry=$retry dlq=$dlq")
    fi
  done
  echo "INFO: pending jobs across all queues: $total"
  for line in "${nonempty[@]}"; do
    echo "INFO: $line"
  done
  if [[ "$require_empty" -eq 1 && "$total" -ne 0 ]]; then
    fail "queue is not empty and --require-empty was requested"
  fi
  echo "PASS: queue depth inspected"

  if [[ -n "$rabbitmq_vhost" ]]; then
    command -v rabbitmqctl >/dev/null 2>&1 || fail "rabbitmqctl is required for --rabbitmq-vhost checks"
    if ! rabbit_out="$(rabbitmqctl -q -p "$rabbitmq_vhost" list_queues name messages_ready messages_unacknowledged 2>/dev/null)"; then
      fail "rabbitmqctl could not inspect vhost $rabbitmq_vhost"
    fi
    [[ -n "$rabbit_out" ]] || warn "no RabbitMQ queues found for vhost $rabbitmq_vhost"
    bad="$(awk '$1 ~ /^q\./ && (($2 + 0) > 0 || ($3 + 0) > 0) {print}' <<<"$rabbit_out")"
    if [[ -n "$bad" ]]; then
      fail "legacy RabbitMQ queues still contain messages:$'\n'$bad"
    fi
    echo "PASS: legacy RabbitMQ q.* queues are empty"
  fi
else
  echo "[1/6] Redis checks skipped for memory driver"
  echo "[2/6] Redis checks skipped for memory driver"
  echo "[3/6] Redis checks skipped for memory driver"
  echo "[4/6] Redis checks skipped for memory driver"
fi

echo "[5/6] ARL doctor"
if [[ "$skip_doctor" -ne 1 ]]; then
  doctor_out="$("$binary" doctor --config "$config" 2>&1)" || {
    printf '%s\n' "$doctor_out" >&2
    fail "arl doctor reported problems"
  }
  printf '%s\n' "$doctor_out"
  grep -q 'mongo: OK' <<<"$doctor_out" || fail "arl doctor did not report mongo: OK"
  grep -q 'queue: OK' <<<"$doctor_out" || fail "arl doctor did not report queue: OK"
  echo "PASS: arl doctor"
else
  warn "arl doctor skipped by --skip-doctor"
fi

echo "[6/6] Migration guard"
if grep -Eq '^[[:space:]]*rabbitmq:' "$config"; then
  fail "rabbitmq config reappeared during preflight"
fi
echo "PASS: no legacy rabbitmq config"
echo "preflight: OK"
