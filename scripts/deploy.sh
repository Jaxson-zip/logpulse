#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.yml"
LOGS_DIR="${PROJECT_ROOT}/logs"
REPORT_FILE="${LOGS_DIR}/report.json"
IMAGE_REF="logpulse:latest"
# 可选固定镜像:将环境变量 IMAGE_ID 设为 `docker image inspect logpulse:latest --format '{{.Id}}'` 的输出,
# start 时会校验实际镜像 ID 与之一致,防止可变标签 :latest 被替换后静默部署不一致的镜像。
IMAGE_ID="${IMAGE_ID:-}"

log_step() { echo "[STEP] $1 开始"; }
log_ok()   { echo "[OK] $1"; }
log_info() { echo "[INFO] $1"; }
log_warn() { echo "[WARN] $1" >&2; }
log_fail() { echo "[FAIL] $1" >&2; }

usage() {
  cat <<EOF
用法: deploy.sh <子命令>

子命令:
  start    启动 logpulse 容器(docker compose up -d)并打印服务状态
  stop     停止并移除容器(docker compose down)
  restart  先 stop 再 start
  logs     实时查看容器日志(docker compose logs -f --tail=100)
  status   查看容器状态(docker compose ps)与 report.json 最近更新时间

示例:
  deploy.sh start
  deploy.sh status
  deploy.sh logs

备注:
  首次运行前请先执行 scripts/build.sh 构建 logpulse:latest 镜像,
  并在 ./logs 目录下放置待分析的日志文件(如 access.log)。

环境变量:
  IMAGE_ID  可选。设为镜像 ID(sha256:...),start 时校验实际镜像一致,
            防止可变标签 :latest 被替换后部署不一致镜像。
EOF
}

# 统一封装 docker compose 调用,固定使用项目根目录的编排文件,与当前工作目录无关。
dc() {
  docker compose -f "${COMPOSE_FILE}" "$@"
}

# 确保宿主机 ./logs 目录存在并以受限权限创建,避免 docker 自动以 root 身份创建导致权限错乱;
# 随后将属主对齐到容器内 app 用户,解决非 root 容器向宿主所属 bind mount 写 report.json 的 UID 不匹配问题。
ensure_logs_dir() {
  if [ ! -d "${LOGS_DIR}" ]; then
    mkdir -p -m 0750 "${LOGS_DIR}"
    log_info "已创建日志目录: ${LOGS_DIR} (mode 0750)"
  fi
  # 查询容器内 app 用户的 UID(覆盖 entrypoint 为 id,USER app 仍生效),据此 chown 日志目录。
  local app_uid
  app_uid=$(docker run --rm --entrypoint id "${IMAGE_REF}" -u 2>/dev/null || echo "")
  if [ -n "${app_uid}" ]; then
    if chown -R "${app_uid}" "${LOGS_DIR}" 2>/dev/null; then
      log_info "日志目录已对齐容器 app 用户 (uid=${app_uid})"
    else
      log_warn "无法 chown ${LOGS_DIR} 到 uid=${app_uid}(当前用户非 root 或系统不支持 chown);若容器写 report.json 失败请以 root 手动 chown"
    fi
  fi
}

# 校验镜像已构建并打印其 ID;若设置了 IMAGE_ID 则校验摘要一致,防止可变标签被替换。
check_image() {
  if ! docker image inspect "${IMAGE_REF}" >/dev/null 2>&1; then
    log_fail "镜像检查"
    echo "未找到镜像 ${IMAGE_REF},请先执行 scripts/build.sh 构建镜像" >&2
    exit 1
  fi
  local id
  id=$(docker image inspect "${IMAGE_REF}" --format '{{.Id}}' 2>/dev/null || echo "")
  log_info "镜像 ${IMAGE_REF} ID: ${id}"
  if [ -n "${IMAGE_ID}" ]; then
    if [ "${id}" != "${IMAGE_ID}" ]; then
      log_fail "镜像摘要校验"
      echo "镜像 ID 不匹配:期望 ${IMAGE_ID},实际 ${id}" >&2
      exit 1
    fi
    log_ok "镜像摘要校验通过 ${id}"
  else
    log_ok "镜像检查 ${IMAGE_REF}(未设置 IMAGE_ID,跳过摘要校验)"
  fi
}

# 打印 report.json 的最近修改时间;兼容 GNU 与 BSD 版本的 stat。
print_report_mtime() {
  if [ ! -f "${REPORT_FILE}" ]; then
    log_warn "尚未生成 report.json(容器可能仍在首次分析,或日志文件 ./logs/access.log 不存在)"
    return 0
  fi
  local mtime
  mtime=$(stat -c %y "${REPORT_FILE}" 2>/dev/null || stat -f %Sm "${REPORT_FILE}" 2>/dev/null || echo "未知")
  log_info "report.json 最近更新时间: ${mtime}"
  log_info "report.json 路径: ${REPORT_FILE}"
}

cmd_start() {
  log_step "启动 logpulse 服务"
  check_image
  ensure_logs_dir
  dc up -d
  log_ok "启动 logpulse 服务"
  echo ""
  dc ps
  echo ""
  log_info "提示: 实时查看日志请执行 deploy.sh logs"
}

cmd_stop() {
  log_step "停止 logpulse 服务"
  dc down
  log_ok "停止 logpulse 服务"
}

cmd_restart() {
  log_step "重启 logpulse 服务(先 stop 再 start)"
  cmd_stop
  cmd_start
  log_ok "重启 logpulse 服务"
}

cmd_logs() {
  dc logs -f --tail=100
}

cmd_status() {
  log_step "查看 logpulse 服务状态"
  dc ps
  echo ""
  print_report_mtime
}

main() {
  local sub="${1:-}"
  case "${sub}" in
    start)   cmd_start ;;
    stop)    cmd_stop ;;
    restart) cmd_restart ;;
    logs)    cmd_logs ;;
    status)  cmd_status ;;
    -h|--help|help)
      usage
      exit 0
      ;;
    "")
      usage
      exit 1
      ;;
    *)
      log_fail "未知子命令: ${sub}"
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
