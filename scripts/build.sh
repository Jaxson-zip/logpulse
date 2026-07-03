#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="logpulse"
IMAGE_TAG="latest"
SHOULD_PUSH="false"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

log_step() {
  echo "[STEP] $1 开始"
}

log_ok() {
  echo "[OK] $1"
}

log_info() {
  echo "[INFO] $1"
}

log_warn() {
  echo "[WARN] $1" >&2
}

log_fail() {
  echo "[FAIL] $1" >&2
}

usage() {
  cat <<EOF
用法: build.sh [--tag <镜像标签>] [--push]

选项:
  --tag <镜像标签>   指定构建的镜像标签,默认 latest
  --push             构建完成后推送到镜像仓库

示例:
  build.sh
  build.sh --tag v1.0.0
  build.sh --tag v1.0.0 --push
EOF
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --tag)
        if [ $# -lt 2 ]; then
          log_fail "参数解析"
          echo "--tag 需要一个参数值" >&2
          exit 1
        fi
        IMAGE_TAG="$2"
        shift 2
        ;;
      --push)
        SHOULD_PUSH="true"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        log_fail "参数解析"
        echo "未知参数: $1" >&2
        usage >&2
        exit 1
        ;;
    esac
  done
}

check_dependencies() {
  log_step "检查前置依赖"
  local missing=""
  if ! command -v go >/dev/null 2>&1; then
    missing="go"
  fi
  if ! command -v docker >/dev/null 2>&1; then
    if [ -n "${missing}" ]; then
      missing="${missing} docker"
    else
      missing="docker"
    fi
  fi
  if [ -n "${missing}" ]; then
    log_fail "检查前置依赖"
    echo "缺少必要依赖: ${missing}" >&2
    echo "请先安装上述命令后再运行本脚本" >&2
    exit 1
  fi
  log_info "go:     $(go version 2>/dev/null || echo '版本未知')"
  log_info "docker: $(docker --version 2>/dev/null || echo '版本未知')"
  log_ok "检查前置依赖"
}

do_build_binary() {
  mkdir -p "${PROJECT_ROOT}/bin" || return 1
  (cd "${PROJECT_ROOT}" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/logpulse .) || return 1
}

do_build_image() {
  (cd "${PROJECT_ROOT}" && docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .) || return 1
}

run_step() {
  local step_name="$1"
  local step_func="$2"
  log_step "${step_name}"
  if "${step_func}"; then
    log_ok "${step_name}"
  else
    log_fail "${step_name}"
    exit 1
  fi
}

print_image_size() {
  log_step "打印镜像大小"
  local image_info
  image_info=$(docker images "${IMAGE_NAME}:${IMAGE_TAG}" --format "{{.Repository}}:{{.Tag}} {{.Size}}" 2>/dev/null) || image_info=""
  if [ -n "${image_info}" ]; then
    log_info "${image_info}"
    log_ok "打印镜像大小"
  else
    log_warn "未获取到镜像 ${IMAGE_NAME}:${IMAGE_TAG} 的大小信息"
  fi
}

push_image() {
  log_step "推送镜像 ${IMAGE_NAME}:${IMAGE_TAG}"
  if docker push "${IMAGE_NAME}:${IMAGE_TAG}"; then
    log_ok "推送镜像 ${IMAGE_NAME}:${IMAGE_TAG}"
  else
    log_warn "推送镜像 ${IMAGE_NAME}:${IMAGE_TAG} 失败,请检查仓库登录状态或网络连接(仅警告,不中断)"
  fi
}

main() {
  parse_args "$@"
  check_dependencies
  run_step "本地编译二进制" do_build_binary
  run_step "构建 Docker 镜像 ${IMAGE_NAME}:${IMAGE_TAG}" do_build_image
  print_image_size
  if [ "${SHOULD_PUSH}" = "true" ]; then
    push_image
  fi
  echo ""
  log_info "构建流程全部完成: ${IMAGE_NAME}:${IMAGE_TAG}"
}

main "$@"
