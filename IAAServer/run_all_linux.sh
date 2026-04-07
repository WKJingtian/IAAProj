#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$ROOT_DIR/linux_build"
RUNTIME_DIR="$ROOT_DIR/linux_runtime"
RELEASES_DIR="$RUNTIME_DIR/releases"
CONTROL_DIR="$RUNTIME_DIR/control"
LOGS_DIR="$RUNTIME_DIR/logs"
PID_DIR="$RUNTIME_DIR/pids"

SUPERVISOR_BIN="$CONTROL_DIR/svr_supervisor"
SUPERVISOR_CFG="$CONTROL_DIR/config.json"
SUPERVISOR_PID="$PID_DIR/svr_supervisor.pid"
SUPERVISOR_APP_LOG="$LOGS_DIR/svr_supervisor.log"
SUPERVISOR_STDOUT_LOG="$LOGS_DIR/svr_supervisor.stdout.log"

usage() {
    cat <<'EOF'
Usage:
  ./run_all_linux.sh import-release <release_name> [source_dir]
  ./run_all_linux.sh start <base_release> [source_dir]
  ./run_all_linux.sh deploy-game <release_name> [source_dir]
  ./run_all_linux.sh status
  ./run_all_linux.sh stop

Notes:
  source_dir defaults to ./linux_build
  start will auto-import the release if it does not already exist
  deploy-game performs svr_game cutover through svr_supervisor
EOF
}

json_get_string() {
    local file="$1"
    local key="$2"
    sed -nE 's/.*"'$key'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' "$file" | head -n 1
}

json_get_number() {
    local file="$1"
    local key="$2"
    sed -nE 's/.*"'$key'"[[:space:]]*:[[:space:]]*([0-9]+).*/\1/p' "$file" | head -n 1
}

ensure_runtime_dirs() {
    mkdir -p "$RELEASES_DIR" "$CONTROL_DIR" "$LOGS_DIR" "$PID_DIR"
}

release_dir() {
    local release_name="$1"
    echo "$RELEASES_DIR/$release_name"
}

default_source_dir() {
    echo "$BUILD_DIR"
}

supervisor_cfg_for_read() {
    if [[ -f "$SUPERVISOR_CFG" ]]; then
        echo "$SUPERVISOR_CFG"
        return
    fi
    if [[ -f "$BUILD_DIR/svr_supervisor/config.json" ]]; then
        echo "$BUILD_DIR/svr_supervisor/config.json"
        return
    fi
    echo ""
}

control_host() {
    local cfg
    cfg="$(supervisor_cfg_for_read)"
    if [[ -z "$cfg" ]]; then
        echo "127.0.0.1"
        return
    fi
    local value
    value="$(json_get_string "$cfg" "control_host")"
    if [[ -z "$value" ]]; then
        echo "127.0.0.1"
    else
        echo "$value"
    fi
}

control_port() {
    local cfg
    cfg="$(supervisor_cfg_for_read)"
    if [[ -z "$cfg" ]]; then
        echo "19090"
        return
    fi
    local value
    value="$(json_get_number "$cfg" "control_port")"
    if [[ -z "$value" ]]; then
        echo "19090"
    else
        echo "$value"
    fi
}

control_url() {
    echo "http://$(control_host):$(control_port)"
}

fetch_supervisor_status() {
    curl -fsS "$(control_url)/status"
}

extract_control_pid_from_status() {
    local body="$1"
    printf '%s\n' "$body" | sed -nE 's/.*"control_pid"[[:space:]]*:[[:space:]]*([0-9]+).*/\1/p' | head -n 1
}

is_supervisor_status_shape() {
    local body="$1"
    [[ "$body" == *'"state"'* && "$body" == *'"processes"'* ]]
}

is_valid_supervisor_status() {
    local body="$1"
    local expected_pid="${2:-}"

    is_supervisor_status_shape "$body" || return 1
    if [[ -z "$expected_pid" ]]; then
        return 0
    fi

    [[ "$body" == *'"control_pid"'* ]] || return 1
    local reported_pid
    reported_pid="$(extract_control_pid_from_status "$body")"
    [[ -n "$reported_pid" && "$reported_pid" == "$expected_pid" ]]
}

is_supervisor_running() {
    if [[ ! -f "$SUPERVISOR_PID" ]]; then
        return 1
    fi

    local pid
    pid="$(cat "$SUPERVISOR_PID")"
    if [[ -z "$pid" ]]; then
        return 1
    fi
    if ! kill -0 "$pid" >/dev/null 2>&1; then
        rm -f "$SUPERVISOR_PID"
        return 1
    fi
    if ! command -v curl >/dev/null 2>&1; then
        return 0
    fi

    local status_body
    if ! status_body="$(fetch_supervisor_status 2>/dev/null)"; then
        return 1
    fi
    is_valid_supervisor_status "$status_body" "$pid"
}

install_supervisor_files() {
    local source_dir="$1"
    ensure_runtime_dirs

    if [[ ! -f "$source_dir/svr_supervisor/svr_supervisor" ]]; then
        echo "[ERROR] Missing supervisor binary in $source_dir/svr_supervisor"
        exit 1
    fi
    if [[ ! -f "$source_dir/svr_supervisor/config.json" ]]; then
        echo "[ERROR] Missing supervisor config in $source_dir/svr_supervisor"
        exit 1
    fi

    cp -f "$source_dir/svr_supervisor/svr_supervisor" "$SUPERVISOR_BIN"
    cp -f "$source_dir/svr_supervisor/config.json" "$SUPERVISOR_CFG"
    chmod +x "$SUPERVISOR_BIN"
}

start_supervisor() {
    local source_dir="$1"

    if is_supervisor_running; then
        echo "[SKIP] svr_supervisor is already running"
        return 0
    fi

    local existing_status
    if existing_status="$(fetch_supervisor_status 2>/dev/null)"; then
        if is_supervisor_status_shape "$existing_status"; then
            local existing_pid
            existing_pid="$(extract_control_pid_from_status "$existing_status")"
            if [[ -n "$existing_pid" ]]; then
                echo "[ERROR] Another svr_supervisor is already serving on $(control_url) with PID $existing_pid. Stop it before starting a new one."
            else
                echo "[ERROR] A legacy or unmanaged supervisor-like process is already serving on $(control_url). Stop it before starting a new one."
            fi
            exit 1
        fi
    fi

    install_supervisor_files "$source_dir"
    : > "$SUPERVISOR_APP_LOG"
    : > "$SUPERVISOR_STDOUT_LOG"

    (
        cd "$ROOT_DIR"
        APP_LOG_PATH="$SUPERVISOR_APP_LOG" \
        IAA_SUPERVISOR_CONFIG_PATH="$SUPERVISOR_CFG" \
        nohup "$SUPERVISOR_BIN" >>"$SUPERVISOR_STDOUT_LOG" 2>&1 &
        echo $! > "$SUPERVISOR_PID"
    )

    for _ in $(seq 1 30); do
        if is_supervisor_running; then
            echo "[OK] svr_supervisor started with PID $(cat "$SUPERVISOR_PID")"
            return 0
        fi
        sleep 1
    done

    echo "[ERROR] svr_supervisor failed to start. Check $SUPERVISOR_STDOUT_LOG"
    exit 1
}

request_control() {
    local method="$1"
    local path="$2"
    local body="${3:-}"
    local tmp
    tmp="$(mktemp)"

    local http_code
    if [[ -n "$body" ]]; then
        http_code="$(curl -sS -o "$tmp" -w "%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$body" "$(control_url)$path")"
    else
        http_code="$(curl -sS -o "$tmp" -w "%{http_code}" -X "$method" "$(control_url)$path")"
    fi

    if [[ "$http_code" -lt 200 || "$http_code" -ge 300 ]]; then
        echo "[ERROR] ${method} ${path} failed with status ${http_code}" >&2
        if [[ -s "$tmp" ]]; then
            cat "$tmp" >&2
            echo >&2
        fi
        rm -f "$tmp"
        return 1
    fi

    cat "$tmp"
    rm -f "$tmp"
}

post_json() {
    local path="$1"
    local body="$2"
    request_control "POST" "$path" "$body"
}

post_empty() {
    local path="$1"
    request_control "POST" "$path"
}

show_status() {
    if ! is_supervisor_running; then
        echo "[STOPPED] svr_supervisor is not running"
        return 0
    fi
    fetch_supervisor_status
    echo
}

import_release() {
    local release_name="$1"
    local source_dir="${2:-$(default_source_dir)}"
    local target_dir

    ensure_runtime_dirs
    target_dir="$(release_dir "$release_name")"

    if [[ -d "$target_dir" ]]; then
        echo "[ERROR] Release already exists: $target_dir"
        exit 1
    fi

    for service in svr_gateway svr_login svr_game; do
        if [[ ! -d "$source_dir/$service" ]]; then
            echo "[ERROR] Missing $service in source dir $source_dir"
            exit 1
        fi
    done

    mkdir -p "$target_dir"
    cp -R "$source_dir/svr_gateway" "$target_dir/"
    cp -R "$source_dir/svr_login" "$target_dir/"
    cp -R "$source_dir/svr_game" "$target_dir/"
    chmod +x "$target_dir/svr_gateway/svr_gateway"
    chmod +x "$target_dir/svr_login/svr_login"
    chmod +x "$target_dir/svr_game/svr_game"
    echo "[OK] Imported release $release_name into $target_dir"
}

ensure_release_present() {
    local release_name="$1"
    local source_dir="${2:-$(default_source_dir)}"
    if [[ -d "$(release_dir "$release_name")" ]]; then
        return 0
    fi
    import_release "$release_name" "$source_dir"
}

start_stack() {
    local base_release="$1"
    local source_dir="${2:-$(default_source_dir)}"

    ensure_release_present "$base_release" "$source_dir"
    start_supervisor "$source_dir"

    post_json "/bootstrap" "{\"base_release\":\"$base_release\"}" >/dev/null
    post_empty "/start" >/dev/null
    echo "[OK] Stack started with base release $base_release"
    show_status
}

deploy_game() {
    local release_name="$1"
    local source_dir="${2:-$(default_source_dir)}"

    ensure_release_present "$release_name" "$source_dir"
    if ! is_supervisor_running; then
        echo "[ERROR] svr_supervisor is not running. Start the stack first."
        exit 1
    fi

    post_json "/deploy/game" "{\"release\":\"$release_name\"}" >/dev/null
    echo "[OK] svr_game cutover triggered with release $release_name"
    show_status
}

stop_stack() {
    if is_supervisor_running; then
        post_empty "/stop" >/dev/null || true
    fi

    if [[ -f "$SUPERVISOR_PID" ]]; then
        local pid
        pid="$(cat "$SUPERVISOR_PID")"
        if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
            kill "$pid" >/dev/null 2>&1 || true
            sleep 1
            if kill -0 "$pid" >/dev/null 2>&1; then
                kill -9 "$pid" >/dev/null 2>&1 || true
            fi
        fi
        rm -f "$SUPERVISOR_PID"
    fi

    echo "[OK] Stack stopped"
}

main() {
    local command="${1:-}"
    case "$command" in
        import-release)
            if [[ $# -lt 2 ]]; then
                usage
                exit 1
            fi
            import_release "$2" "${3:-$(default_source_dir)}"
            ;;
        start)
            if [[ $# -lt 2 ]]; then
                usage
                exit 1
            fi
            start_stack "$2" "${3:-$(default_source_dir)}"
            ;;
        deploy-game)
            if [[ $# -lt 2 ]]; then
                usage
                exit 1
            fi
            deploy_game "$2" "${3:-$(default_source_dir)}"
            ;;
        status)
            show_status
            ;;
        stop)
            stop_stack
            ;;
        help|-h|--help|"")
            usage
            ;;
        *)
            echo "[ERROR] Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

main "$@"
