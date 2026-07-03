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
├── Dockerfile
├── .dockerignore
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
