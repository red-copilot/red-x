#!/usr/bin/env bash
set -euo pipefail

base_url="${ARL_BASE_URL:-http://127.0.0.1:5003}/api/v1"
username="${E2E_USERNAME:?set E2E_USERNAME}"
password="${E2E_PASSWORD:?set E2E_PASSWORD}"
domain="${EASM_LIVE_DOMAIN:?set EASM_LIVE_DOMAIN to an authorized domain}"
scan="${EASM_LIVE_SCAN:-0}"
cookie_jar="$(mktemp)"
login_body="$(mktemp)"
trap 'rm -f "$cookie_jar" "$login_body"' EXIT
chmod 600 "$cookie_jar" "$login_body"

jq -n --arg username "$username" --arg password "$password" '{username:$username,password:$password}' >"$login_body"
login="$(curl -fsS -c "$cookie_jar" -H 'Content-Type: application/json' --data-binary @"$login_body" "$base_url/auth/login")"
csrf="$(jq -er '.csrfToken' <<<"$login")"

call() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS -b "$cookie_jar" -H "X-CSRF-Token: $csrf" -H 'Content-Type: application/json' -X "$method" --data-binary "$body" "$base_url$path"
  else
    curl -fsS -b "$cookie_jar" -H "X-CSRF-Token: $csrf" -X "$method" "$base_url$path"
  fi
}

echo "[1/6] health"
call GET /overview | jq -e '.health.chinaz != "unavailable" and .health.icpQuery == "ok" and .health.geoip == "ok" and .health.neo4j == "ok"' >/dev/null

echo "[2/6] domain -> organization"
lookup="$(call POST /domain-lookups "$(jq -n --arg domain "$domain" '{domain:$domain}')")"
org_id="$(jq -er '.organization.id' <<<"$lookup")"
echo "organization=$org_id confidence=$(jq -r '.confidence' <<<"$lookup")"

echo "[3/6] organization -> filed domains"
job="$(call POST "/organizations/$org_id/icp-refresh" '{}')"
job_id="$(jq -er '.id' <<<"$job")"
for _ in $(seq 1 60); do
  status="$(call GET "/domain-lookups/$job_id" | jq -r '.status')"
  [[ "$status" == "done" ]] && break
  [[ "$status" == "failed" ]] && { echo "ICP job failed" >&2; exit 1; }
  sleep 2
done
[[ "${status:-}" == "done" ]] || { echo "ICP job timed out" >&2; exit 1; }

echo "[4/6] review candidates"
domains_json="$(call GET "/organizations/$org_id/domains?status=candidate")"
candidates="$(jq -r '[.items[].rootDomain]' <<<"$domains_json")"
if [[ "$(jq 'length' <<<"$candidates")" -gt 0 ]]; then
  call POST "/organizations/$org_id/domains/confirm" "$(jq -n --argjson domains "$candidates" '{domains:$domains}')" >/dev/null
fi

if [[ "$scan" != "1" ]]; then
  echo "[5/6] scan skipped; set EASM_LIVE_SCAN=1 to run a bounded authorized scan"
  echo "[6/6] export"
  export_json="$(call POST "/organizations/$org_id/exports" '{"format":"xlsx"}')"
  echo "artifact=$(jq -r '.file' <<<"$export_json")"
  exit 0
fi

echo "[5/6] bounded scan"
confirmed="$(jq -r '[.items[] | select(.status == "confirmed") | .rootDomain] | .[0]' <<<"$(call GET "/organizations/$org_id/domains")")"
[[ -n "$confirmed" && "$confirmed" != "null" ]] || { echo "no confirmed domain to scan" >&2; exit 1; }
scan_body="$(jq -n --arg domain "$confirmed" '{rootDomains:[$domain],options:{portScan:true,portScanType:"top100",serviceDetection:true,siteIdentify:true,sslCert:true,nucleiScan:false}}')"
scan_res="$(call POST "/organizations/$org_id/scans" "$scan_body")"
task_id="$(jq -er '.items[0].id' <<<"$scan_res")"
for _ in $(seq 1 180); do
  status="$(call GET "/tasks/$task_id" | jq -r '.task.status')"
  case "$status" in succeeded|partial|failed|canceled) break;; esac
  sleep 5
done
echo "task=$task_id status=${status:-unknown}"

echo "[6/6] summary, findings, and export"
call GET "/organizations/$org_id/summary" | jq -e '.confirmedDomains >= 1' >/dev/null
call GET "/organizations/$org_id/findings?state=current&pageSize=1" >/dev/null
export_json="$(call POST "/organizations/$org_id/exports" '{"format":"xlsx"}')"
echo "artifact=$(jq -r '.file' <<<"$export_json")"
