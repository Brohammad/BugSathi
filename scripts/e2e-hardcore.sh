#!/usr/bin/env bash
# Hardcore E2E suite for BugSathi local stack (API :8080, worker :8081).
#
# No `set -e`: this harness does its own PASS/FAIL accounting and must run every
# section to completion even when individual assertions fail.
set -uo pipefail

API="${API:-http://127.0.0.1:8080}"
WORKER="${WORKER:-http://127.0.0.1:8081}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="${TMPDIR:-/tmp}/bugsathi-e2e-$$"
mkdir -p "$TMP"
REPORT="$TMP/report.txt"
CURLLOG="$TMP/curl.err"
: >"$CURLLOG"
PASS=0
FAIL=0
SKIP=0

# Globals populated by req(); pre-declared so `set -u` stays happy.
CODE=""
BODY=""
RESP_HEADERS=""

log() { printf '%s\n' "$*" | tee -a "$REPORT"; }
ok() { PASS=$((PASS+1)); log "PASS  $*"; return 0; }
bad() { FAIL=$((FAIL+1)); log "FAIL  $*"; return 1; }
skip() { SKIP=$((SKIP+1)); log "SKIP  $*"; return 0; }
section() { log ""; log "=== $* ==="; }

assert_eq() {
  local got=$1 want=$2 msg=$3
  if [[ "$got" == "$want" ]]; then ok "$msg (got=$got)"; else bad "$msg (got=$got want=$want)"; fi
}

assert_code() {
  local code=$1 want=$2 msg=$3
  assert_eq "$code" "$want" "$msg"
}

# assert_code_any GOT MSG WANT...  — passes if GOT matches any acceptable code.
assert_code_any() {
  local got=$1 msg=$2; shift 2
  local w
  for w in "$@"; do
    if [[ "$got" == "$w" ]]; then ok "$msg (got=$got)"; return 0; fi
  done
  bad "$msg (got=$got want one of: $*)"
}

assert_nonempty() {
  local val=$1 msg=$2
  if [[ -n "$val" ]]; then ok "$msg"; else bad "$msg (empty)"; fi
}

# json_get DOTTED.PATH — reads stdin, prints value or nothing. Never fails.
json_get() {
  python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
v = d
for p in sys.argv[1].split("."):
    if isinstance(v, dict) and p in v:
        v = v[p]
    else:
        sys.exit(0)
if v is not None:
    print(v)
' "$1" 2>/dev/null
}

# json_unwrap_get ENVELOPE FIELD — reads stdin, unwraps {"envelope":{...}} if
# present, then prints FIELD. Never fails.
json_unwrap_get() {
  python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
env, field = sys.argv[1], sys.argv[2]
if isinstance(d, dict) and isinstance(d.get(env), dict):
    d = d[env]
if isinstance(d, dict):
    v = d.get(field, "")
    if v is not None:
        print(v)
' "$1" "$2" 2>/dev/null
}

# http_code [curl args...] — prints the status code, or 000 if curl failed.
http_code() {
  local c
  c=$(curl -sS -o /dev/null -w '%{http_code}' "$@" 2>>"$CURLLOG") || c="000"
  printf '%s' "$c"
}

# req METHOD URL [json_body] [extra curl args...]
# Sets globals CODE, BODY, RESP_HEADERS. Must NOT be called in a command
# substitution or the globals stay trapped in the subshell.
req() {
  local method=$1 url=$2; shift 2
  local body=""
  local -a extra=()
  if [[ $# -gt 0 && "$1" != -* ]]; then body=$1; shift; fi
  if [[ $# -gt 0 ]]; then extra=("$@"); fi

  local hdr="$TMP/hdr" out="$TMP/out" bodyfile="$TMP/req-body"
  local -a args=(-sS -D "$hdr" -o "$out" -w '%{http_code}' -X "$method" "$url")
  if [[ -n "$body" ]]; then
    # Bodies go through a file so large payloads cannot blow past ARG_MAX.
    printf '%s' "$body" >"$bodyfile"
    args+=(-H 'Content-Type: application/json' --data-binary "@$bodyfile")
  fi
  if [[ -n "${ACCESS:-}" ]]; then
    args+=(-H "Authorization: Bearer $ACCESS")
  fi
  if [[ ${#extra[@]} -gt 0 ]]; then args+=("${extra[@]}"); fi

  CODE=$(curl "${args[@]}" 2>>"$CURLLOG") || CODE="000"
  BODY=$(cat "$out" 2>/dev/null || true)
  RESP_HEADERS=$(cat "$hdr" 2>/dev/null || true)
}

wait_http() {
  local url=$1 want=${2:-200} label=$3
  local i c="000"
  for ((i = 1; i <= 60; i++)); do
    c=$(http_code "$url")
    if [[ "$c" == "$want" ]]; then ok "$label ready ($c)"; return 0; fi
    sleep 1
  done
  bad "$label not ready (last=$c want=$want)"
  return 1
}

STAMP="$(date +%s)-$$"
EMAIL_A="e2e-owner-$STAMP@example.com"
EMAIL_B="e2e-member-$STAMP@example.com"
PASSWD='password123'

section "0. Health / readiness"
READY=1
wait_http "$API/healthz" 200 "API healthz" || READY=0
wait_http "$API/readyz" 200 "API readyz" || READY=0
wait_http "$WORKER/healthz" 200 "Worker healthz" || READY=0
wait_http "$WORKER/readyz" 200 "Worker readyz" || READY=0
if [[ "$READY" != "1" ]]; then
  section "SUMMARY"
  log "ABORT: API/worker not reachable — refusing to continue."
  log "Start the stack first, e.g.:"
  log "  make up && make migrate && make run-api & make run-worker &"
  log "PASS=$PASS FAIL=$FAIL SKIP=$SKIP"
  log "Full log: $REPORT"
  log "curl stderr: $CURLLOG"
  exit 2
fi

H=$(curl -sSI "$API/healthz" 2>>"$CURLLOG" || true)
echo "$H" | grep -qi 'X-Content-Type-Options: nosniff' && ok "security header nosniff" || bad "missing nosniff"
echo "$H" | grep -qi 'X-Frame-Options: DENY' && ok "security header frame deny" || bad "missing frame deny"

MET=$(curl -sS "$API/metrics" 2>>"$CURLLOG" || true)
echo "$MET" | grep -q 'bugsathi_http_requests_total' && ok "API metrics scrape" || bad "API metrics missing"
METW=$(curl -sS "$WORKER/metrics" 2>>"$CURLLOG" || true)
echo "$METW" | grep -q 'bugsathi_pipeline_jobs_total\|bugsathi_http_requests_total' && ok "Worker metrics scrape" || bad "Worker metrics missing"

section "1. Auth — register / login / me / refresh / logout"
ACCESS=""
req POST "$API/v1/auth/register" "{\"email\":\"$EMAIL_A\",\"password\":\"$PASSWD\",\"name\":\"Owner E2E\"}"
assert_code "$CODE" "201" "register owner"
ACCESS=$(printf '%s' "$BODY" | json_get tokens.access_token)
REFRESH=$(printf '%s' "$BODY" | json_get tokens.refresh_token)
OWNER_ID=$(printf '%s' "$BODY" | json_get user.id)
assert_nonempty "$ACCESS" "owner access token"

req GET "$API/v1/auth/me"
assert_code "$CODE" "200" "auth me"
assert_eq "$(printf '%s' "$BODY" | json_get user.email)" "$EMAIL_A" "me email"

req POST "$API/v1/auth/login" "{\"email\":\"$EMAIL_A\",\"password\":\"$PASSWD\"}"
assert_code "$CODE" "200" "login"
ACCESS=$(printf '%s' "$BODY" | json_get tokens.access_token)
REFRESH=$(printf '%s' "$BODY" | json_get tokens.refresh_token)

req POST "$API/v1/auth/refresh" "{\"refresh_token\":\"$REFRESH\"}"
assert_code "$CODE" "200" "refresh"
NEW_REFRESH=$(printf '%s' "$BODY" | json_get tokens.refresh_token)
ACCESS=$(printf '%s' "$BODY" | json_get tokens.access_token)

# old refresh should fail after rotation
req POST "$API/v1/auth/refresh" "{\"refresh_token\":\"$REFRESH\"}"
assert_code_any "$CODE" "rotated refresh rejected" 401 400 403
REFRESH=$NEW_REFRESH

req POST "$API/v1/auth/login" "{\"email\":\"$EMAIL_A\",\"password\":\"wrong-password\"}"
assert_code_any "$CODE" "bad password rejected" 401 400

req POST "$API/v1/auth/register" "{\"email\":\"$EMAIL_B\",\"password\":\"$PASSWD\",\"name\":\"Member E2E\"}"
assert_code "$CODE" "201" "register member"
MEMBER_ACCESS=$(printf '%s' "$BODY" | json_get tokens.access_token)
MEMBER_ID=$(printf '%s' "$BODY" | json_get user.id)

section "2. Auth rate limit (auth bucket)"
# Burst auth with wrong password from same IP — expect 429 eventually
HIT_429=0
for i in $(seq 1 40); do
  c=$(http_code -X POST "$API/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"rate-$i@x.com\",\"password\":\"nope\"}")
  if [[ "$c" == "429" ]]; then HIT_429=1; break; fi
done
[[ "$HIT_429" == "1" ]] && ok "auth rate limit returns 429" || bad "auth rate limit never 429 (check RATE_LIMIT / AUTH_RATE_LIMIT)"

# cool down briefly
sleep 2

# re-login owner after rate limit storm (bucket refills over RATE_LIMIT_WINDOW)
for i in $(seq 1 40); do
  req POST "$API/v1/auth/login" "{\"email\":\"$EMAIL_A\",\"password\":\"$PASSWD\"}"
  if [[ "$CODE" == "200" ]]; then break; fi
  sleep 2
done
assert_code "$CODE" "200" "owner re-login after rate limit window"
ACCESS=$(printf '%s' "$BODY" | json_get tokens.access_token)

section "3. Projects CRUD + members"
req POST "$API/v1/projects" '{"name":"E2E Project"}'
assert_code "$CODE" "201" "create project"
PID=$(printf '%s' "$BODY" | json_get project.id)
assert_nonempty "$PID" "project id $PID"

req GET "$API/v1/projects"
assert_code "$CODE" "200" "list projects"

req GET "$API/v1/projects/$PID"
assert_code "$CODE" "200" "get project"

req PATCH "$API/v1/projects/$PID" '{"name":"E2E Project Renamed"}'
assert_code "$CODE" "200" "update project"

req POST "$API/v1/projects/$PID/members" "{\"user_id\":\"$MEMBER_ID\",\"role\":\"member\"}"
assert_code_any "$CODE" "add member" 201 200

req GET "$API/v1/projects/$PID/members"
assert_code "$CODE" "200" "list members"

# member cannot delete project
SAVE_ACCESS=$ACCESS
ACCESS=$MEMBER_ACCESS
req DELETE "$API/v1/projects/$PID"
assert_code "$CODE" "403" "member cannot delete project"
ACCESS=$SAVE_ACCESS

section "4. Upload → complete → pipeline → report"
VIDEO="$TMP/bug.webm"
FFLOG="$TMP/ffmpeg.log"
if command -v ffmpeg >/dev/null 2>&1; then
  # WebM takes Opus or Vorbis; builds vary in which encoder they ship.
  AENC=""
  for cand in libopus libvorbis; do
    if ffmpeg -hide_banner -encoders 2>/dev/null | grep -q " $cand "; then AENC=$cand; break; fi
  done
  if [[ -z "$AENC" ]]; then
    bad "no webm audio encoder (libopus/libvorbis) in this ffmpeg build"
  elif ffmpeg -y -f lavfi -i color=c=blue:s=320x240:d=2 -f lavfi -i sine=f=440:d=2 \
    -c:v libvpx -c:a "$AENC" -shortest "$VIDEO" >"$FFLOG" 2>&1; then
    ok "generated sample webm (audio=$AENC)"
  else
    bad "ffmpeg generate failed — see $FFLOG"
  fi
else
  bad "ffmpeg missing — cannot generate video"
fi

req POST "$API/v1/projects/$PID/recordings" \
  '{"content_type":"video/webm","filename":"bug.webm","metadata":{"browser":"chrome","os":"darwin"}}'
assert_code "$CODE" "201" "create recording"
RID=$(printf '%s' "$BODY" | json_get recording.id)
UPLOAD_URL=$(printf '%s' "$BODY" | json_get upload_url)
if [[ -n "$RID" && -n "$UPLOAD_URL" ]]; then ok "recording $RID + upload_url"; else bad "missing recording/upload_url"; fi

# complete without upload → 404 object missing
req POST "$API/v1/projects/$PID/recordings/$RID/complete"
assert_code "$CODE" "404" "complete without object → 404"

if [[ -f "$VIDEO" ]]; then
  PUT_CODE=$(http_code -X PUT -H 'Content-Type: video/webm' --data-binary @"$VIDEO" "$UPLOAD_URL")
  assert_code_any "$PUT_CODE" "MinIO PUT upload" 200 204
fi

req POST "$API/v1/projects/$PID/recordings/$RID/complete"
assert_code "$CODE" "200" "complete upload"
assert_eq "$(printf '%s' "$BODY" | json_unwrap_get recording status)" "UPLOADED" "status UPLOADED"

# idempotent complete
req POST "$API/v1/projects/$PID/recordings/$RID/complete"
assert_code "$CODE" "200" "idempotent complete"

section "5. Wait for READY report (media+AI)"
REPORT_ID=""
ST=""
RBODY=""
RCODE=""
for i in $(seq 1 90); do
  req GET "$API/v1/projects/$PID/recordings/$RID"
  ST=$(printf '%s' "$BODY" | json_unwrap_get recording status)
  req GET "$API/v1/projects/$PID/recordings/$RID/report"
  RBODY=$BODY
  RCODE=$CODE
  if [[ "$RCODE" == "200" ]]; then
    RST=$(printf '%s' "$RBODY" | json_unwrap_get report status)
    if [[ "$RST" == "READY" ]]; then
      REPORT_ID=$(printf '%s' "$RBODY" | json_unwrap_get report id)
      ok "pipeline READY in ~$((i * 2))s (recording=$ST)"
      break
    fi
  fi
  if [[ "$ST" == "FAILED" ]]; then
    bad "recording FAILED before report ready"
    break
  fi
  sleep 2
done
if [[ -z "$REPORT_ID" ]]; then
  bad "timed out waiting for READY report (last recording status=$ST code=$RCODE body=${RBODY:0:300})"
fi

if [[ -n "$REPORT_ID" ]]; then
  req GET "$API/v1/projects/$PID/reports/$REPORT_ID"
  assert_code "$CODE" "200" "get report by id"
  TITLE=$(printf '%s' "$BODY" | json_unwrap_get report title)
  assert_nonempty "$TITLE" "report has title"

  req GET "$API/v1/projects/$PID/reports"
  assert_code "$CODE" "200" "list reports"

  # cache warm
  req GET "$API/v1/projects/$PID/reports/$REPORT_ID"
  assert_code "$CODE" "200" "cached report get"
fi

section "6. Sharing"
if [[ -n "$REPORT_ID" ]]; then
  req POST "$API/v1/projects/$PID/reports/$REPORT_ID/shares" '{}'
  assert_code_any "$CODE" "create share" 201 200
  TOKEN=$(printf '%s' "$BODY" | json_unwrap_get share token)
  SHARE_ID=$(printf '%s' "$BODY" | json_unwrap_get share id)
  assert_nonempty "$TOKEN" "share token"

  SAVE=$ACCESS; ACCESS=""
  req GET "$API/s/$TOKEN"
  assert_code "$CODE" "200" "public share get unauthenticated"
  ACCESS=$SAVE

  req GET "$API/v1/projects/$PID/reports/$REPORT_ID/shares"
  assert_code "$CODE" "200" "list shares"

  if [[ -n "$SHARE_ID" ]]; then
    req DELETE "$API/v1/projects/$PID/shares/$SHARE_ID"
    assert_code_any "$CODE" "revoke share" 204 200
    SAVE=$ACCESS; ACCESS=""
    req GET "$API/s/$TOKEN"
    assert_code_any "$CODE" "revoked share blocked" 404 410 403
    ACCESS=$SAVE
  fi
else
  skip "sharing (no report)"
fi

section "7. Collab comments + SSE"
if [[ -n "$REPORT_ID" ]]; then
  req POST "$API/v1/projects/$PID/reports/$REPORT_ID/comments" '{"body":"e2e comment hardcore"}'
  assert_code_any "$CODE" "create comment" 201 200

  req GET "$API/v1/projects/$PID/reports/$REPORT_ID/comments"
  assert_code "$CODE" "200" "list comments"
  echo "$BODY" | grep -q 'e2e comment hardcore' && ok "comment body present" || bad "comment missing"

  # SSE: read a few events with timeout
  SSE_OUT="$TMP/sse.txt"
  curl -sS -N -H "Authorization: Bearer $ACCESS" \
    --max-time 3 \
    "$API/v1/projects/$PID/reports/$REPORT_ID/events" >"$SSE_OUT" 2>>"$CURLLOG" || true
  if [[ -s "$SSE_OUT" ]]; then
    ok "SSE events endpoint streams"
  else
    # empty stream within 3s can still be OK if the connection was accepted
    c=$(http_code -H "Authorization: Bearer $ACCESS" --max-time 2 \
      "$API/v1/projects/$PID/reports/$REPORT_ID/events")
    assert_code "$c" "200" "SSE endpoint accepts"
  fi
else
  skip "collab (no report)"
fi

section "8. Body limit"
BIG=$(python3 -c 'print("{\"name\":\""+"A"*1200000+"\"}")')
req POST "$API/v1/projects" "$BIG"
assert_code_any "$CODE" "oversized body rejected" 413 400

section "9. Reprocess (owner) + member forbidden"
if [[ -n "$RID" ]]; then
  ACCESS=$MEMBER_ACCESS
  req POST "$API/v1/projects/$PID/recordings/$RID/reprocess"
  assert_code "$CODE" "403" "member cannot reprocess"
  ACCESS=$SAVE_ACCESS

  # ensure owner token is fresh (rate limit bucket may still be draining)
  for i in $(seq 1 20); do
    req POST "$API/v1/auth/login" "{\"email\":\"$EMAIL_A\",\"password\":\"$PASSWD\"}"
    [[ "$CODE" == "200" ]] && break
    sleep 2
  done
  ACCESS=$(printf '%s' "$BODY" | json_get tokens.access_token)

  req POST "$API/v1/projects/$PID/recordings/$RID/reprocess"
  assert_code "$CODE" "202" "owner reprocess accepted"
fi

section "10. Unauthorized / not found negatives"
SAVE=$ACCESS; ACCESS=""
req GET "$API/v1/projects"
assert_code "$CODE" "401" "unauth projects"
ACCESS=$SAVE
req GET "$API/v1/projects/00000000-0000-0000-0000-000000000000"
assert_code_any "$CODE" "missing project" 404 403

section "11. Cleanup delete project"
req DELETE "$API/v1/projects/$PID"
assert_code_any "$CODE" "delete project" 204 200

section "SUMMARY"
log "PASS=$PASS FAIL=$FAIL SKIP=$SKIP"
log "Full log: $REPORT"
log "curl stderr: $CURLLOG"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
