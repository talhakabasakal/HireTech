#!/usr/bin/env bash
# Start the HireTech backend and web application from the repository root.
#
# Usage:
#   ./dev.sh           Backend + Next.js web app
#   ./dev.sh web       Backend + Next.js web app
#   ./dev.sh desktop   Backend + Next.js + Electron desktop app
#   ./dev.sh stop      Stop processes started by this script

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$PROJECT_ROOT/hiretech_backend"
FRONTEND_ROOT="$PROJECT_ROOT/hiretech_frontend"
MODE="$(printf '%s' "${1:-web}" | tr '[:upper:]' '[:lower:]')"
PIDS=()
BACKEND_CONTAINER_NAME="hiretech-backend-dev"
BACKEND_IMAGE="hiretech-backend-dev:local"

# Optional local-only deployment secrets/configuration. The file is ignored by
# git and is never copied into the frontend or Docker image.
LOCAL_ENV_FILE="${HIRETECH_ENV_FILE:-$PROJECT_ROOT/.env.local}"
if [[ -f "$LOCAL_ENV_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$LOCAL_ENV_FILE"
    set +a
fi

# Mail delivery. Local development uses the bundled Mailpit inbox; when a
# real provider is supplied, forward it into the backend container unchanged.
if [[ -n "${HIRETECH_SMTP_HOST:-}" ]]; then
    EMAIL_PROVIDER="smtp"
    SMTP_HOST="$HIRETECH_SMTP_HOST"
    SMTP_PORT="${HIRETECH_SMTP_PORT:-587}"
    SMTP_TLS_MODE="${HIRETECH_SMTP_TLS_MODE:-starttls}"
    SMTP_USERNAME="${HIRETECH_SMTP_USERNAME:-}"
    SMTP_PASSWORD="${HIRETECH_SMTP_PASSWORD:-}"
    if [[ -n "${HIRETECH_EMAIL_FROM:-}" ]]; then
        EMAIL_FROM="$HIRETECH_EMAIL_FROM"
    elif [[ -n "$SMTP_USERNAME" ]]; then
        EMAIL_FROM="HireTech <noreply@hiretech.com>"
    else
        EMAIL_FROM="HireTech <noreply@hiretech.com>"
    fi
else
    EMAIL_PROVIDER="smtp"
    SMTP_HOST="mailpit"
    SMTP_PORT="1025"
    SMTP_TLS_MODE="none"
    SMTP_USERNAME=""
    SMTP_PASSWORD=""
    EMAIL_FROM="HireTech <noreply@hiretech.com>"
fi

# Keep the project database separate from a PostgreSQL installation already
# running on the host. This value is consumed by both Compose and the Go API.
export DB_PORT="${HIRETECH_DB_PORT:-55432}"
export HIRETECH_DESKTOP_PORT="${HIRETECH_DESKTOP_PORT:-3210}"
export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:3000,http://127.0.0.1:3000,http://localhost:${HIRETECH_DESKTOP_PORT},http://127.0.0.1:${HIRETECH_DESKTOP_PORT}}"

# The integrated runner uses the real API. Mock mode is an explicit opt-in for
# isolated UI demos and is never selected implicitly by this script.
export NEXT_PUBLIC_DATA_MODE="${NEXT_PUBLIC_DATA_MODE:-api}"
export NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL:-http://127.0.0.1:8080}"
export NEXT_PUBLIC_APP_URL="${NEXT_PUBLIC_APP_URL:-http://localhost:3000}"
export HIRETECH_RENDERER_URL="${HIRETECH_RENDERER_URL:-http://localhost:3000}"

log() { printf '[dev] %s\n' "$*"; }

check_desktop_prerequisites() {
    [[ "$MODE" == "desktop" && "$(uname -s)" == "Linux" ]] || return 0

    local sandbox_helper="$FRONTEND_ROOT/node_modules/electron/dist/chrome-sandbox"
    local sandbox_uid=""
    local sandbox_mode=""

    if [[ -f "$sandbox_helper" ]]; then
        sandbox_uid="$(stat -c '%u' "$sandbox_helper" 2>/dev/null || true)"
        sandbox_mode="$(stat -c '%a' "$sandbox_helper" 2>/dev/null || true)"
    fi

    if [[ "$sandbox_uid" != "0" || "$sandbox_mode" != "4755" ]]; then
        log "Electron Linux sandbox helper yapılandırılmamış."
        log "Şu komutları bir kez çalıştırın:"
        log "sudo chown root:root $sandbox_helper"
        log "sudo chmod 4755 $sandbox_helper"
        return 1
    fi
}

terminate_process_tree() {
    local parent_pid="$1"
    local child_pid

    if command -v pgrep >/dev/null 2>&1; then
        while read -r child_pid; do
            [[ -n "$child_pid" ]] && terminate_process_tree "$child_pid"
        done < <(pgrep -P "$parent_pid" 2>/dev/null || true)
    fi

    if kill -0 "$parent_pid" 2>/dev/null; then
        kill "$parent_pid" 2>/dev/null || true
    fi
}

cleanup() {
    local exit_code=$?
    trap - EXIT INT TERM
    for pid in "${PIDS[@]:-}"; do
        terminate_process_tree "$pid"
    done
    wait 2>/dev/null || true
    exit "$exit_code"
}

wait_for_started_processes() {
    local pid
    local state

    while true; do
        for pid in "${PIDS[@]}"; do
            state="$(ps -o stat= -p "$pid" 2>/dev/null | tr -d '[:space:]' || true)"
            if [[ -z "$state" || "$state" == Z* ]]; then
                wait "$pid"
                return $?
            fi
        done
        sleep 1
    done
}

stop_existing() {
    # Only stop processes listening on the project ports. Do not kill unrelated
    # Node/Go processes owned by the user.
    for port in 3000 8080; do
        local pids=""
        if command -v lsof >/dev/null 2>&1; then
            pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
        fi
        # lsof can miss listeners in some namespace/container setups even
        # though the command itself is installed. Fall back when it returns
        # no PID instead of assuming that the port is free.
        if [[ -z "$pids" ]] && command -v fuser >/dev/null 2>&1; then
            pids="$(fuser -n tcp "$port" 2>&1 | awk -F: '{print $2}' || true)"
        fi

        if [[ -n "$pids" ]]; then
            log "Port $port kullanımda; eski süreç(ler) durduruluyor..."
            while read -r pid; do
                [[ -n "$pid" ]] && kill "$pid" 2>/dev/null || true
            done <<< "$pids"
        fi

        # A process may need a short moment to release its socket. Starting
        # Next.js immediately after kill otherwise causes a false EADDRINUSE.
        for _ in $(seq 1 50); do
            if ! (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1; then
                break
            fi
            sleep 0.1
        done
        if (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1; then
            log "Port $port hâlâ kullanımda; mevcut süreç otomatik olarak durdurulamadı."
            return 1
        fi
    done
}

configure_docker() {
    if docker info >/dev/null 2>&1; then
        return 0
    fi
    if [[ -S /var/run/docker.sock ]] && DOCKER_HOST="unix:///var/run/docker.sock" docker info >/dev/null 2>&1; then
        export DOCKER_HOST="unix:///var/run/docker.sock"
        log "Docker Desktop context kullanılamıyor; çalışan yerel Docker Engine kullanılıyor."
        return 0
    fi
    return 1
}

wait_for_port() {
    local port="$1"
    local label="$2"
    local attempts="${3:-60}"
    local pid="${4:-}"
    for _ in $(seq 1 "$attempts"); do
        if (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1; then
            # Do not mistake an older process on the same port for the process
            # started by this script. Give the child time to report EADDRINUSE.
            if [[ -n "$pid" ]]; then
                sleep 2
                if ! kill -0 "$pid" 2>/dev/null; then
                    log "$label süreci port açıldıktan hemen sonra sonlandı."
                    return 1
                fi
            fi
            log "$label hazır: http://localhost:$port"
            return 0
        fi
        if [[ -n "$pid" ]] && ! kill -0 "$pid" 2>/dev/null; then
            log "$label süreci beklenmedik şekilde sonlandı."
            return 1
        fi
        sleep 1
    done
    log "$label $port portunda başlatılamadı."
    return 1
}

start_backend() {
    if [[ "${DOCKER_HOST:-}" == "unix:///var/run/docker.sock" ]]; then
        start_backend_in_docker
        return
    fi
    [[ -x "$BACKEND_ROOT/dev.sh" ]] || chmod +x "$BACKEND_ROOT/dev.sh"
    log "Backend başlatılıyor..."
    (cd "$BACKEND_ROOT" && exec ./dev.sh) &
    PIDS+=("$!")
}

start_backend_in_docker() {
    log "Backend Docker Compose ağı içinde başlatılıyor..."

    (cd "$BACKEND_ROOT" && docker compose -f deployments/docker-compose.yml up -d --wait)
    (cd "$BACKEND_ROOT" && ./scripts/migrate.sh up)

    if ! docker image inspect "$BACKEND_IMAGE" >/dev/null 2>&1; then
        log "Backend Docker imajı oluşturuluyor..."
        (cd "$BACKEND_ROOT" && docker build -t "$BACKEND_IMAGE" -f deployments/Dockerfile .)
    fi

    docker rm -f "$BACKEND_CONTAINER_NAME" >/dev/null 2>&1 || true
    docker run --rm --name "$BACKEND_CONTAINER_NAME" \
        --network deployments_default \
        -p 8080:8080 \
        -e DB_HOST=postgres \
        -e DB_PORT=5432 \
        -e REDIS_HOST=redis \
        -e REDIS_PORT=6379 \
        -e KAFKA_BROKERS=kafka:29092 \
        -e KAFKA_ENABLED=true \
        -e CORS_ALLOWED_ORIGINS="$CORS_ALLOWED_ORIGINS" \
        -e EMAIL_PROVIDER="$EMAIL_PROVIDER" \
        -e SMTP_HOST="$SMTP_HOST" \
        -e SMTP_PORT="$SMTP_PORT" \
        -e SMTP_TLS_MODE="$SMTP_TLS_MODE" \
        -e SMTP_USERNAME="$SMTP_USERNAME" \
        -e SMTP_PASSWORD="$SMTP_PASSWORD" \
        -e EMAIL_FROM="$EMAIL_FROM" \
        "$BACKEND_IMAGE" /server &
    PIDS+=("$!")
}

start_web() {
    log "Next.js web uygulaması başlatılıyor..."
    (cd "$FRONTEND_ROOT" && exec npm run dev:web) &
    PIDS+=("$!")
}

start_desktop() {
    log "Electron masaüstü uygulaması başlatılıyor..."
    (cd "$FRONTEND_ROOT" && exec npm run dev:desktop) &
    PIDS+=("$!")
}

    case "$MODE" in
    web|desktop)
        trap cleanup EXIT INT TERM
        stop_existing
        check_desktop_prerequisites
        if ! configure_docker; then
            log "Docker daemon erişilebilir değil. Docker Desktop/Engine'i başlatın."
            exit 1
        fi
        start_backend
        start_web
        # Check the web process first so a stale/occupied 3000 is reported
        # immediately. The first Go build can take longer than Next.js startup.
        wait_for_port 3000 "Web uygulaması" 60 "${PIDS[1]}"
        wait_for_port 8080 "Backend" 180 "${PIDS[0]}"
        if [[ "$MODE" == "desktop" ]]; then
            start_desktop
            log "Geliştirme ortamı hazır. Kapatmak için Ctrl+C kullanın."
        else
            log "Web geliştirme ortamı hazır. Kapatmak için Ctrl+C kullanın."
        fi
        # If Electron, Next.js, or the backend exits, stop the remaining
        # processes too instead of leaving stale listeners behind.
        wait_for_started_processes
        ;;
    stop)
        stop_existing
        log "3000 ve 8080 portlarındaki geliştirme süreçleri durduruldu."
        ;;
    *)
        printf 'Kullanım: %s [web|desktop|stop]\n' "$0" >&2
        exit 2
        ;;
esac
