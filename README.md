# logpulse

一个用 Go 实现的命令行日志分析工具,面向运维/后端场景。读取本地日志文件或 stdin 管道,输出统计报告,支持通用 `[LEVEL]` 格式与 nginx combined 格式,提供级别统计、Top N 高频 IP/接口、时间与级别过滤、连续 ERROR 异常告警,以及终端表格与结构化 JSON 两种输出。

## 功能概览

- **双格式解析**:通用 `[ERROR]`/`[WARN]`/`[INFO]`/`[DEBUG]` 标记;nginx combined 提取客户端 IP 与请求路径
- **级别统计**:总行数 + 各级别计数,未识别计入 `unknown`
- **Top N**:nginx 模式下高频 IP 与高频接口路径排序(默认 Top 10)
- **过滤**:`--from`/`--to` 时间范围(RFC3339 或 `YYYY-MM-DD` 简写)、`--level` 逗号分隔多选
- **异常检测**:连续 N 次 ERROR 触发告警,输出起止行号与持续次数
- **输出**:`--format-output table|json`,JSON 报告字段含 `summary`/`level_counts`/`top_ips`/`top_paths`/`alerts`

## 环境要求

- Go 1.22+(本仓库在 Go 1.25 上开发)
- 或 Docker(用于容器化运行)

## 本地构建

```bash
# 在仓库根目录执行
go build -o logpulse .

# 运行(二进制在当前目录)
./logpulse --help
```

### 一键构建脚本 scripts/build.sh

脚本封装「本地编译二进制 + 构建 Docker 镜像」全流程,推荐使用。

```bash
# 基础用法:构建二进制与 logpulse:latest 镜像
bash scripts/build.sh

# 指定镜像标签
bash scripts/build.sh --tag v1.0.0

# 构建并推送到镜像仓库
bash scripts/build.sh --tag v1.0.0 --push

# 查看帮助
bash scripts/build.sh --help
```

- 前置依赖:`go` 与 `docker` 命令必须存在,缺失时脚本报错退出
- 构建产物:`bin/logpulse`(本地二进制)+ `logpulse:<tag>`(Docker 镜像)
- 推送失败仅警告,不中断流程

## 使用示例

### 1. 基础行数与级别统计(通用格式)

```bash
./logpulse analyze access.log
```

输出:
```
文件 access.log 总行数: 5
级别统计:
  ERROR   2
  WARN    1
  INFO    1
  DEBUG   0
  unknown 1
告警: (无)
```

### 2. nginx 格式 + Top N

```bash
./logpulse analyze access.log --format nginx -n 5
```

输出高频 IP Top 5 与高频路径 Top 5。

### 3. 时间与级别过滤

```bash
# 仅看 2026-07-01 当天的 ERROR 与 WARN
./logpulse analyze access.log --from 2026-07-01 --to 2026-07-02 --level ERROR,WARN
```

### 4. 连续 ERROR 告警(自定义阈值)

```bash
./logpulse analyze access.log --alert-threshold 3
```

连续 3 次 ERROR 即触发告警(默认阈值为 5)。

### 5. JSON 结构化报告

```bash
./logpulse analyze access.log --format-output=json
```

输出带缩进的合法 JSON,可被 `jq` 解析:
```bash
./logpulse analyze access.log --format-output=json | jq '.summary, .alerts'
```

### 6. 从 stdin 读取(管道)

```bash
cat access.log | ./logpulse analyze /dev/stdin
```

## 参数说明

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--format` | `generic` | 日志格式:`generic` 或 `nginx` |
| `-n, --n` | `10` | Top N 输出条数(仅 nginx 模式生效) |
| `--from` | (无) | 起始时间,支持 RFC3339 或 `YYYY-MM-DD` |
| `--to` | (无) | 结束时间,`YYYY-MM-DD` 简写时含当天 23:59:59 |
| `--level` | (无) | 级别过滤,逗号分隔,如 `ERROR,WARN` |
| `--format-output` | `table` | 输出格式:`table` 或 `json` |
| `--alert-threshold` | `5` | 连续 ERROR 告警阈值(仅 generic 模式生效) |

## Docker 使用

### 构建镜像

```bash
docker build -t logpulse .
```

### 运行容器

```bash
# 通过 stdin 传入日志
cat access.log | docker run --rm -i logpulse analyze /dev/stdin

# 分析容器外文件:挂载目录
docker run --rm -v "$PWD":/data logpulse analyze /data/access.log --format nginx -n 5
```

### 镜像说明

- 多阶段构建:`golang:1.25-alpine` 编译 → `alpine:3.21` 运行
- 构建期禁用 CGO(`CGO_ENABLED=0`),生成纯静态二进制
- `-trimpath -ldflags="-s -w"` 去除路径与调试信息,缩减体积
- 运行阶段以非 root 用户 `app` 执行
- 镜像体积约 15~18MB(< 20MB)

### 容器编排 docker-compose.yml

`docker-compose.yml` 定义 `logpulse` 服务,挂载宿主机 `./logs` 目录到容器 `/data`,通过 shell 循环每 60 秒全量分析 `/data/access.log`,将 JSON 报告原子写入 `/data/report.json`(先写临时文件再 `mv` 替换,分析失败时保留上一份有效报告),错误信息写入 `/data/analyze.err`。

```bash
# 前台启动(查看实时输出,Ctrl+C 停止)
docker compose up

# 后台启动
docker compose up -d

# 查看状态与停止
docker compose ps
docker compose down
```

服务配置要点:

| 项 | 值 / 说明 |
|---|---|
| image | `logpulse:latest`(需先执行 `scripts/build.sh` 构建) |
| volumes | `./logs:/data`,日志输入与报告输出共用该目录 |
| command | `sh -c` 循环,每 60s 触发一次 `logpulse analyze`(批处理工具需读到 EOF 才输出,故用循环而非 `tail -F` 管道) |
| restart | `unless-stopped` |
| 健康检查 | 每 30s 执行一次,验证 `report.json` 存在且在近 120s 内更新过(`-mmin -2`,相对 60s 周期留边界余量);超时 10s、重试 3 次、启动宽限 60s |
| 资源限制 | `mem_limit: 256m`、`cpus: 1.0`、`pids_limit: 100` |
| 容器加固 | `read_only: true`(rootfs 只读,写仅落 /data 与 /tmp tmpfs)、`cap_drop: [ALL]`、`no-new-privileges` |
| labels | `app=logpulse`、`version=1.0.0` |

使用前需在 `./logs` 下放置待分析的日志文件(如 `access.log`),报告会输出到 `./logs/report.json`。

## 部署运维

`scripts/deploy.sh` 封装 `docker compose` 的启停、日志、状态查看等运维操作。

```bash
# 启动服务(docker compose up -d)并打印状态
bash scripts/deploy.sh start

# 停止并移除容器(docker compose down)
bash scripts/deploy.sh stop

# 重启:先 stop 再 start
bash scripts/deploy.sh restart

# 实时查看日志(docker compose logs -f --tail=100)
bash scripts/deploy.sh logs

# 查看容器状态与 report.json 最近更新时间
bash scripts/deploy.sh status

# 查看用法说明
bash scripts/deploy.sh --help
```

子命令说明:

| 子命令 | 行为 |
|---|---|
| `start` | 校验镜像存在 → 确保 `./logs` 目录存在并按容器 `app` 用户对齐属主 → `docker compose up -d` → 打印 `docker compose ps` |
| `stop` | `docker compose down`,停止并移除容器与网络 |
| `restart` | 先 `stop` 再 `start` |
| `logs` | `docker compose logs -f --tail=100`,实时跟踪日志 |
| `status` | `docker compose ps` 并打印 `report.json` 最近更新时间 |
| (无参数) | 打印 usage 用法,退出码 1 |

- 首次运行前请先执行 `scripts/build.sh` 构建镜像
- 可选环境变量 `IMAGE_ID`:设为镜像 ID(`docker image inspect logpulse:latest --format '{{.Id}}'` 的输出),`start` 时校验实际镜像一致,防止可变标签 `:latest` 被替换后部署不一致镜像

## 项目结构

```
logpulse/
├── main.go                     # 入口
├── cmd/
│   ├── root.go                 # 根命令
│   └── analyze.go              # analyze 子命令与各模式分发
├── internal/
│   ├── parse/                  # 日志解析(级别、nginx)
│   ├── stat/                   # Top N 排序
│   ├── filter/                 # 时间/级别过滤
│   ├── alert/                  # 连续 ERROR 检测
│   └── report/                 # JSON 报告结构化
├── scripts/
│   ├── build.sh                # 一键构建(编译二进制 + 构建镜像)
│   └── deploy.sh               # 部署运维(启停/日志/状态)
├── docker-compose.yml          # 容器编排(挂载日志、周期分析、健康检查)
├── Dockerfile                  # 多阶段镜像构建
├── .dockerignore               # 构建忽略清单
├── .gitignore                  # Git 忽略清单
├── .gitattributes              # 行尾归一化(*.sh 强制 LF)
├── go.mod
├── go.sum
└── README.md
```

## 测试

```bash
go test ./...
go test -cover ./...
```

整体测试覆盖率 >= 90%。

## License

MIT
