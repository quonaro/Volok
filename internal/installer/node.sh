#!/usr/bin/env bash
# Volok node installer — installs Xray-core with VLESS+REALITY on this VPS
# and registers the direct share link back to Volok.
#
# Expected environment (injected by Volok when serving this script):
#   VOLOK_PUBLIC_URL  - https origin of the Volok server
#   VOLOK_TOKEN       - admin token (used only for the registration callback)
#
# Usage (run as root on the VPS):
#   bash -s -- [--name NAME] [--host HOST] [--port PORT] \
#              [--sni SNI] [--target HOST:PORT] [--fingerprint FP] [--flow FLOW]

set -euo pipefail

VOLOK_PUBLIC_URL=__VOLOK_PUBLIC_URL__
VOLOK_TOKEN=__VOLOK_TOKEN__

XRAY_VERSION="v26.3.27"
XRAY_BASE="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}"

NAME=""
HOST=""
PORT=""
SNI="yandex.ru"
TARGET="yandex.ru:443"
FINGERPRINT="chrome"
FLOW="xtls-rprx-vision"

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/usr/local/etc/xray"
META_DIR="/etc/volok-node"
META_FILE="${META_DIR}/node.json"
TMP_DIR=""

log_info() { printf '\e[32m[INFO]\e[0m  %s\n' "$*"; }
log_error() { printf '\e[31m[ERROR]\e[0m %s\n' "$*" >&2; }
die() { log_error "$1"; exit 1; }

url_encode() {
	local input="$1" out="" i ch
	for ((i = 0; i < ${#input}; i++)); do
		ch="${input:$i:1}"
		if [[ "$ch" =~ [A-Za-z0-9_.~-] ]]; then
			out+="$ch"
		else
			out+=$(printf '%%%02X' "'$ch")
		fi
	done
	printf '%s\n' "$out"
}

usage() {
	cat <<'EOF'
Usage: bash -s -- [OPTIONS]
  --name NAME              display name (default: hostname)
  --host HOST              public host/IP of this VPS (required or auto-detected)
  --port PORT              VLESS port (default: random 10000-65535)
  --sni SNI                REALITY SNI (default: yandex.ru)
  --target HOST:PORT       REALITY camouflage target (default: yandex.ru:443)
  --fingerprint FP         uTLS fingerprint (default: chrome)
  --flow FLOW              VLESS flow (default: xtls-rprx-vision)
EOF
}

parse_args() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
			--name) NAME="${2:-}"; shift 2 ;;
			--host) HOST="${2:-}"; shift 2 ;;
			--port) PORT="${2:-}"; shift 2 ;;
			--sni) SNI="${2:-}"; shift 2 ;;
			--target) TARGET="${2:-}"; shift 2 ;;
			--fingerprint) FINGERPRINT="${2:-}"; shift 2 ;;
			--flow) FLOW="${2:-}"; shift 2 ;;
			-h|--help) usage; exit 0 ;;
			*) die "unknown option: $1 (run with --help)" ;;
		esac
	done
}

validate_args() {
	[[ -n "$SNI" ]] || die "SNI is empty"
	[[ -n "$TARGET" ]] || die "REALITY target is empty"
	[[ -n "$FINGERPRINT" ]] || die "fingerprint is empty"
	[[ -z "$PORT" || "$PORT" =~ ^[0-9]+$ ]] || die "invalid port: $PORT"
	if [[ -n "$NAME" && ${#NAME} -gt 128 ]]; then
		die "name is too long (max 128 chars)"
	fi
}

detect_os() {
	[[ -f /etc/os-release ]] || die "cannot detect OS: /etc/os-release not found"
	# shellcheck source=/dev/null
	. /etc/os-release
	PKG_MGR=""
	case "${ID:-unknown}" in
		debian|ubuntu|linuxmint|pop|zorin|elementary) PKG_MGR="apt" ;;
		fedora|rhel|centos|rocky|almalinux|ol|amzn)
			if command -v dnf &>/dev/null; then PKG_MGR="dnf"; else PKG_MGR="yum"; fi ;;
		arch|manjaro|endeavouros|garuda) PKG_MGR="pacman" ;;
		alpine) PKG_MGR="apk" ;;
		*) die "unsupported OS: ${ID:-unknown}" ;;
	esac
}

detect_init() {
	if command -v systemctl &>/dev/null && [[ -d /etc/systemd/system ]]; then
		INIT_SYSTEM="systemd"
	elif command -v rc-update &>/dev/null; then
		INIT_SYSTEM="openrc"
	else
		die "no supported init system (systemd or openrc)"
	fi
}

detect_arch() {
	local machine
	machine="$(uname -m)"
	case "$machine" in
		x86_64|amd64) XRAY_ARCH="64" ;;
		aarch64|arm64) XRAY_ARCH="arm64-v8a" ;;
		armv7l|armv7) XRAY_ARCH="arm32-v7a" ;;
		*) die "unsupported architecture: $machine" ;;
	esac
}

install_deps() {
	log_info "installing dependencies..."
	case "$PKG_MGR" in
		apt)
			export DEBIAN_FRONTEND=noninteractive
			apt-get update -y
			apt-get install -y curl jq coreutils unzip tar ca-certificates
			;;
		dnf|yum)
			$PKG_MGR install -y curl jq coreutils unzip tar ca-certificates
			;;
		pacman)
			# Avoid partial sync upgrades; refresh index without upgrading the system.
			pacman -Sy --noconfirm curl jq coreutils unzip tar ca-certificates
			;;
		apk)
			apk add --no-cache bash curl jq coreutils unzip tar ca-certificates
			;;
	esac
}

install_xray() {
	local zip="$1"
	local url="${XRAY_BASE}/${zip}"
	local expected="$2"
	local tmp
	tmp="$(mktemp -d "${TMP_DIR}/xray.XXXXXX")"
	curl -fsSL "$url" -o "$tmp/xray.zip"
	local actual
	actual="$(sha256sum "$tmp/xray.zip" | awk '{print $1}')"
	if [[ "$actual" != "$expected" ]]; then
		die "checksum mismatch for $zip (expected $expected, got $actual)"
	fi
	unzip -q -o "$tmp/xray.zip" -d "$tmp/unpacked"
	install -Dm 755 "$tmp/unpacked/xray" "$INSTALL_DIR/xray"
	rm -rf "$tmp"
	log_info "Xray-core ${XRAY_VERSION} installed"
}

generate_identity() {
	if [[ -f "$META_FILE" ]]; then
		log_info "reusing existing node identity"
		return 0
	fi

	mkdir -p "$META_DIR"
	UUID="$("$INSTALL_DIR/xray" uuid)"
	KEY_OUT="$("$INSTALL_DIR/xray" x25519)"
	PRIVATE_KEY="$(echo "$KEY_OUT" | awk -F': ' '/^PrivateKey:/ {print $2}')"
	PUBLIC_KEY="$(echo "$KEY_OUT" | awk -F': ' '/^Password/ {print $2}')"
	[[ -n "$PRIVATE_KEY" && -n "$PUBLIC_KEY" ]] || die "could not parse x25519 keys"
	SHORT_ID="$(od -An -tx1 -N8 /dev/urandom | tr -d ' \n')"
	[[ -z "$PORT" ]] && PORT="$(shuf -i 10000-65535 -n 1)"
	[[ -z "$HOST" ]] && HOST="$(detect_public_ip)"
	[[ -z "$NAME" ]] && NAME="$(hostname 2>/dev/null || echo vps)"

	cat > "$META_FILE" <<EOF
{
  "node_id": "$(random_hex 16)",
  "port": $PORT,
  "host": "$HOST",
  "name": "$NAME",
  "installed_version": "${XRAY_VERSION}"
}
EOF
	chmod 600 "$META_FILE"
	log_info "node identity created"
}

load_identity() {
	[[ -f "$META_FILE" ]] || die "node metadata missing"
	NODE_ID="$(jq -r '.node_id' "$META_FILE")"
	PORT="$(jq -r '.port' "$META_FILE")"
	HOST="$(jq -r '.host' "$META_FILE")"
	NAME="$(jq -r '.name' "$META_FILE")"
	# Read credentials from the existing Xray config for a managed node.
	if [[ -f "$CONFIG_DIR/config.json" ]]; then
		UUID="$(jq -r '.inbounds[0].settings.clients[0].id' "$CONFIG_DIR/config.json")"
		PUBLIC_KEY="$(jq -r '.inbounds[0].streamSettings.realitySettings.publicKey' "$CONFIG_DIR/config.json")"
		PRIVATE_KEY="$(jq -r '.inbounds[0].streamSettings.realitySettings.privateKey' "$CONFIG_DIR/config.json")"
		SHORT_ID="$(jq -r '.inbounds[0].streamSettings.realitySettings.shortIds[0]' "$CONFIG_DIR/config.json")"
	fi
}

random_hex() {
	od -An -tx1 -N"$1" /dev/urandom | tr -d ' \n'
}

detect_public_ip() {
	curl -4 -fsSL https://icanhazip.com 2>/dev/null || curl -4 -fsSL https://api.ipify.org 2>/dev/null || die "cannot detect public IP; pass --host"
}

generate_config() {
	mkdir -p "$CONFIG_DIR"
	jq -n \
		--arg uuid "$UUID" \
		--arg flow "$FLOW" \
		--argjson port "$PORT" \
		--arg target "$TARGET" \
		--arg sni "$SNI" \
		--arg private_key "$PRIVATE_KEY" \
		--arg public_key "$PUBLIC_KEY" \
		--arg short_id "$SHORT_ID" \
		--arg fingerprint "$FINGERPRINT" \
		'{
			log: { loglevel: "warning" },
			inbounds: [
				{
					port: $port,
					protocol: "vless",
					settings: {
						clients: [{ id: $uuid, flow: $flow }],
						decryption: "none"
					},
					streamSettings: {
						network: "tcp",
						security: "reality",
						realitySettings: {
							target: $target,
							serverNames: [$sni],
							privateKey: $private_key,
							publicKey: $public_key,
							shortIds: [$short_id],
							fingerprint: $fingerprint
						}
					}
				}
			],
			outbounds: [{ protocol: "freedom", tag: "direct" }]
		}' > "$CONFIG_DIR/config.json"
	log_info "xray config written"
}

create_service() {
	case "$INIT_SYSTEM" in
		systemd)
			cat > /etc/systemd/system/xray.service <<EOF
[Unit]
Description=Xray Service
After=network.target nss-lookup.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/xray run -config $CONFIG_DIR/config.json
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
			systemctl daemon-reload
			systemctl enable xray.service
			;;
		openrc)
			cat > /etc/init.d/xray <<'EOF'
#!/sbin/openrc-run
description="Xray Service"
command="/usr/local/bin/xray"
command_args="run -config /usr/local/etc/xray/config.json"
command_background="yes"
pidfile="/run/${RC_SVCNAME}.pid"
depend() { need net; after firewall; }
EOF
			chmod +x /etc/init.d/xray
			rc-update add xray default
			;;
	esac
}

start_service() {
	case "$INIT_SYSTEM" in
		systemd)
			systemctl restart xray.service
			systemctl is-active --quiet xray.service || die "xray failed to start"
			;;
		openrc)
			rc-service xray restart
			rc-service xray status || die "xray failed to start"
			;;
	esac
	log_info "xray is running"
}

check_port() {
	if (echo > "/dev/tcp/127.0.0.1/$PORT") 2>/dev/null; then
		log_info "port $PORT is listening locally"
	else
		die "port $PORT is not listening"
	fi
}

build_link() {
	local ip="$1"
	local params
	params="security=reality&encryption=none&fp=${FINGERPRINT}&pbk=${PUBLIC_KEY}&sid=${SHORT_ID}&flow=${FLOW}&type=tcp&headerType=none&sni=${SNI}"
	echo "vless://${UUID}@${ip}:${PORT}?${params}#$(url_encode "$NAME")"
}

register_node() {
	[[ -n "${VOLOK_PUBLIC_URL:-}" && "${VOLOK_PUBLIC_URL}" != "__VOLOK_PUBLIC_URL__" ]] || die "Volok public URL is not configured"
	[[ -n "${VOLOK_TOKEN:-}" && "${VOLOK_TOKEN}" != "__VOLOK_TOKEN__" ]] || die "Volok token is not configured"

	local link
	link="$(build_link "$HOST")"
	local payload
	payload="$(jq -n --arg name "$NAME" --arg url "$link" '{name: $name, url: $url}')"

	local attempt rc
	for attempt in 1 2 3; do
		rc=0
		# TLS certificate verification stays enabled for https origins.
		curl -fsS --max-time 20 \
			-X PUT \
			-H "Authorization: Bearer ${VOLOK_TOKEN}" \
			-H "Content-Type: application/json" \
			--data "$payload" \
			"${VOLOK_PUBLIC_URL}/nodes/${NODE_ID}" || rc=$?
		if [[ $rc -eq 0 ]]; then
			log_info "node registered: ${NODE_ID}"
			return 0
		fi
		if [[ $rc -ge 400 && $rc -lt 500 ]] && [[ $rc -ne 408 && $rc -ne 429 ]]; then
			die "registration rejected (HTTP $rc)"
		fi
		log_info "registration attempt $attempt failed (exit $rc), retrying..."
		sleep 3
	done
	die "registration failed after 3 attempts; the node itself is installed and running"
}

main() {
	parse_args "$@"
	validate_args
	[[ $EUID -eq 0 ]] || die "run as root on the VPS"

	detect_os
	detect_init
	detect_arch

	TMP_DIR="$(mktemp -d /tmp/volok.XXXXXX)"
	trap 'rm -rf "$TMP_DIR"' EXIT

	install_deps
	install_xray "Xray-linux-${XRAY_ARCH}.zip" "${SHA256_BY_ARCH[$XRAY_ARCH]}"

	generate_identity
	load_identity
	generate_config

	"$INSTALL_DIR/xray" run -test -config "$CONFIG_DIR/config.json" || die "xray config test failed"

	create_service
	start_service
	check_port

	register_node

	log_info "done. Xray is running on port ${PORT}"
	log_info "share link: $(build_link "$HOST")"
}

declare -A SHA256_BY_ARCH=(
	["64"]="23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae"
	["arm64-v8a"]="4d30283ae614e3057f730f67cd088a42be6fdf91f8639d82cb69e48cde80413c"
	["arm32-v7a"]="c7265ae13c63ca0241a037df4ef960ad37938c8a67d984cc08834b2cfdf5654b"
)

main "$@"
