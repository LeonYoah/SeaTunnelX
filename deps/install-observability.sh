#!/usr/bin/env bash
set -euo pipefail

#
# install-observability.sh
#
# 安装并初始化/启动 Prometheus / Alertmanager / Grafana 三件套。
#
# 行为约定：
# - 优先使用离线包（deps/downloads 下的三个 tar.gz），缺失则走在线下载。
# - 在线下载：Prometheus/Alertmanager 优先走 GitHub 代理，Grafana 直接连官方源（代理不支持）。
#

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
DOWNLOAD_DIR="$BASE_DIR/downloads"

mkdir -p "$DOWNLOAD_DIR"

PROMETHEUS_VERSION="${PROMETHEUS_VERSION:-3.9.1}"
ALERTMANAGER_VERSION="${ALERTMANAGER_VERSION:-0.31.1}"
GRAFANA_VERSION="${GRAFANA_VERSION:-12.3.3}"

# 目前仅考虑 Linux amd64/arm64，其他架构直接失败
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    PROM_OSARCH="linux-amd64"
    ALERT_OSARCH="linux-amd64"
    GRAFANA_OSARCH="linux-amd64"
    ;;
  aarch64|arm64)
    PROM_OSARCH="linux-arm64"
    ALERT_OSARCH="linux-arm64"
    GRAFANA_OSARCH="linux-arm64"
    ;;
  *)
    echo "Unsupported architecture: $ARCH (only x86_64/amd64 and aarch64/arm64 are supported)" >&2
    exit 1
    ;;
esac

PROM_ARCHIVE="prometheus-${PROMETHEUS_VERSION}.${PROM_OSARCH}.tar.gz"
ALERT_ARCHIVE="alertmanager-${ALERTMANAGER_VERSION}.${ALERT_OSARCH}.tar.gz"

# Grafana 使用企业版华为云镜像包
GRAFANA_ENTERPRISE_BUILD_ID="21957728731"
case "$GRAFANA_OSARCH" in
  linux-amd64)
    GRAFANA_ARCHIVE="grafana-enterprise_${GRAFANA_VERSION}_${GRAFANA_ENTERPRISE_BUILD_ID}_linux_amd64.tar.gz"
    ;;
  linux-arm64)
    GRAFANA_ARCHIVE="grafana-enterprise_${GRAFANA_VERSION}_${GRAFANA_ENTERPRISE_BUILD_ID}_linux_arm64.tar.gz"
    ;;
  *)
    echo "Unsupported grafana arch: $GRAFANA_OSARCH" >&2
    exit 1
    ;;
esac

PROM_URL="https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/${PROM_ARCHIVE}"
ALERT_URL="https://github.com/prometheus/alertmanager/releases/download/v${ALERTMANAGER_VERSION}/${ALERT_ARCHIVE}"
# Grafana 使用华为云镜像源（结构与官方保持一致）
GRAFANA_URL="https://repo.huaweicloud.com/grafana/${GRAFANA_VERSION}/${GRAFANA_ARCHIVE}"

PROXY_PREFIX="https://hk.gh-proxy.org/"

log() {
  echo "[install-observability] $*"
}

# 用于 GitHub 源：优先代理，失败回退官方（curl）
download_with_proxy() {
  local url="$1"
  local out="$2"

  local proxy_url="${PROXY_PREFIX}${url}"

  log "Downloading (proxy first): $proxy_url"
  if curl -fSL "$proxy_url" -o "$out"; then
    log "Downloaded via proxy"
    return 0
  fi

  log "Proxy failed, falling back to origin: $url"
  curl -fSL "$url" -o "$out"
  log "Downloaded from origin"
}

# 直接下载（Grafana 等非 GitHub 源，代理不支持，curl）
download_direct() {
  local url="$1"
  local out="$2"
  log "Downloading: $url"
  curl -fSL "$url" -o "$out"
  log "Downloaded: $url"
}

have_offline_bundles() {
  [[ -f "$DOWNLOAD_DIR/$PROM_ARCHIVE" ]] &&
    [[ -f "$DOWNLOAD_DIR/$ALERT_ARCHIVE" ]] &&
    [[ -f "$DOWNLOAD_DIR/$GRAFANA_ARCHIVE" ]]
}

install_component_from_tar() {
  local archive="$1"     # 文件名（不含路径）
  local target_name="$2" # 安装后在 BASE_DIR 下的目录名，例如 prometheus/alertmanager/grafana
  local prefix="$3"      # 解压出来的目录前缀；为空时自动探测

  local tar_path="$DOWNLOAD_DIR/$archive"

  if [[ ! -f "$tar_path" ]]; then
    echo "missing archive: $tar_path" >&2
    exit 1
  fi

  local target_path="$BASE_DIR/$target_name"

  # 清理旧的目标目录
  rm -rf "$target_path"

  # 解压到 BASE_DIR
  log "Extracting $archive to $BASE_DIR"
  tar -xf "$tar_path" -C "$BASE_DIR"

  local extracted_dir=""

  if [[ -n "$prefix" ]]; then
    extracted_dir="$BASE_DIR/$prefix"
    if [[ ! -d "$extracted_dir" ]]; then
      echo "expected directory not found after extract: $extracted_dir" >&2
      exit 1
    fi
  else
    # 自动探测解压出来的目录（用于 Grafana enterprise 这类包）
    local base_prefix="${target_name%%-*}-" # grafana- / prometheus- 等
    for d in "$BASE_DIR"/${base_prefix}*; do
      if [[ -d "$d" ]]; then
        extracted_dir="$d"
        # 若目录名中包含版本号则优先选用
        if [[ "$d" == *"$GRAFANA_VERSION"* ]]; then
          extracted_dir="$d"
          break
        fi
      fi
    done
    if [[ -z "$extracted_dir" ]]; then
      echo "failed to auto-detect extracted directory for $archive" >&2
      exit 1
    fi
  fi

  # 如果解压目录名已经等于目标目录名，则无需再移动
  if [[ "$extracted_dir" == "$target_path" ]]; then
    log "Extracted directory already at target: $target_path"
    return 0
  fi

  mv "$extracted_dir" "$target_path"
}

main() {
  log "Architecture detected: $ARCH ($PROM_OSARCH)"
  log "Versions: prometheus=$PROMETHEUS_VERSION, alertmanager=$ALERTMANAGER_VERSION, grafana=$GRAFANA_VERSION"
  log "Download directory: $DOWNLOAD_DIR"

  if have_offline_bundles; then
    log "Found offline archives in downloads/:"
    log "  - $PROM_ARCHIVE"
    log "  - $ALERT_ARCHIVE"
    log "  - $GRAFANA_ARCHIVE"
  else
    log "Offline archives not complete, starting online download..."
    download_with_proxy "$PROM_URL" "$DOWNLOAD_DIR/$PROM_ARCHIVE"
    download_with_proxy "$ALERT_URL" "$DOWNLOAD_DIR/$ALERT_ARCHIVE"
    download_direct "$GRAFANA_URL" "$DOWNLOAD_DIR/$GRAFANA_ARCHIVE"
  fi

  # 安装到带版本号的目录
  install_component_from_tar "$PROM_ARCHIVE" "prometheus-${PROMETHEUS_VERSION}" "prometheus-${PROMETHEUS_VERSION}.${PROM_OSARCH}"
  install_component_from_tar "$ALERT_ARCHIVE" "alertmanager-${ALERTMANAGER_VERSION}" "alertmanager-${ALERTMANAGER_VERSION}.${ALERT_OSARCH}"
  # Grafana Enterprise 包解压目录名可能与文件名不完全一致，使用自动探测前缀 grafana-
  install_component_from_tar "$GRAFANA_ARCHIVE" "grafana-${GRAFANA_VERSION}" ""

  log "Binaries installed under:"
  log "  - $BASE_DIR/prometheus-${PROMETHEUS_VERSION}"
  log "  - $BASE_DIR/alertmanager-${ALERTMANAGER_VERSION}"
  log "  - $BASE_DIR/grafana-${GRAFANA_VERSION}"
  log ""

  log "Running init-observability-defaults.sh..."
  "$BASE_DIR/init-observability-defaults.sh"

  log "Starting observability stack via start-observability.sh..."
  "$BASE_DIR/start-observability.sh"
}

main "$@"

