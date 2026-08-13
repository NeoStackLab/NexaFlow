# NexaFlow

开源、AI 原生、多租户的企业业务应用平台。NexaFlow 通过动态数据模型、业务记录、表单、工作流、权限、文件、知识库和 AI Agent 组合企业内部应用，可作为 CRM、订单管理、项目管理和审批系统的通用底座。

当前版本采用 Go + Next.js 的前后端分离架构，推荐使用 Docker Compose 安装和运行。

## 当前能力

- 六步首次安装向导，自动检查 PostgreSQL、Redis 和持久化目录
- 多租户企业空间、JWT 会话、角色权限和动态菜单
- 动态实体与字段定义，统一 JSONB 业务记录存储
- 通用 CRUD、表单构建器和 JSON Schema 校验
- 审批、条件、通知节点组成的可执行工作流
- 企业文件空间，本地、S3、Cloudflare R2 存储支持
- pgvector 企业知识库和权限受控的 AI Agent
- 纯白冷蓝 SaaS 管理后台和真实数据总览
- SaaS 套餐、用量计数、Stripe Checkout 和 Webhook
- 中英文界面，默认简体中文

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 前端 | Next.js 16.3、React 19、TypeScript、Tailwind CSS 4、TanStack Query |
| 后端 | Go 1.26.5、Gin、GORM、Zap |
| 数据库 | PostgreSQL 18、pgvector 0.8.6 |
| 缓存 | Redis 8.8 |
| 部署 | Docker Desktop、Docker Compose、多阶段非 root 镜像 |

## 项目结构

```text
NexaFlow/
├── backend/
│   ├── cmd/server/             # Go API 启动入口
│   ├── configs/                # 后端默认配置
│   └── internal/
│       ├── api/                # Gin 路由注册
│       ├── handler/            # HTTP 输入输出层
│       ├── middleware/         # 鉴权、租户和通用中间件
│       ├── model/              # 领域与持久化模型
│       ├── pkg/                # 数据库、缓存、日志等基础包
│       ├── repository/         # PostgreSQL/Redis 数据访问
│       └── service/            # 业务规则与事务边界
├── frontend/
│   ├── public/                 # 静态资源
│   └── src/
│       ├── app/                # Next.js App Router 页面和布局
│       │   ├── admin/          # 管理后台路由
│       │   ├── api/            # 前端健康检查 Route Handler
│       │   ├── install/        # 首次安装向导
│       │   ├── login/          # 登录
│       │   └── register/       # 员工注册
│       ├── components/         # 后台、表单、工作流和通用组件
│       └── lib/                # API 客户端、状态和业务类型
├── docker/compose.yaml         # 完整容器编排
├── docs/                       # 架构、API、数据库和部署文档
├── scripts/verify.ps1          # 后端和前端质量检查
├── .env.example                # 环境变量模板
└── README.md
```

后端依赖方向固定为：

```text
HTTP Request → Handler → Service → Repository → PostgreSQL / Redis
```

动态实体不会为每个实体创建独立 PostgreSQL 表。实体定义保存在 `entities`/`entity_fields`，业务数据统一存入 `dynamic_records.values` JSONB，并通过 `tenant_id` 和 `entity_id` 隔离。

## Windows 首次安装（推荐）

### 1. 准备环境

需要安装并启动：

- Docker Desktop（包含 Docker Compose）
- Git（仅克隆代码时需要）

在 PowerShell 中确认 Docker 已运行：

```powershell
docker desktop status
docker version
docker compose version
```

如果状态是 `starting`，等待 Docker Desktop 显示 `running` 后再继续。

### 2. 创建 `.env`

```powershell
Set-Location F:\spacex\NexaFlow
Copy-Item .env.example .env
```

生成兼容 Windows PowerShell 5.1/旧版 .NET 的 JWT 密钥：

```powershell
$bytes = New-Object byte[] 48
$rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create()
$rng.GetBytes($bytes)
$jwtSecret = [Convert]::ToBase64String($bytes)
$rng.Dispose()
(Get-Content .env) -replace '^JWT_SECRET=.*$', "JWT_SECRET=$jwtSecret" | Set-Content .env
```

不要使用 `[RandomNumberGenerator]::Fill()`，部分 Windows PowerShell/.NET 版本没有这个静态方法。

打开 `.env`，至少修改以下值：

```dotenv
POSTGRES_PASSWORD=一个强数据库密码
REDIS_PASSWORD=一个强Redis密码
JWT_SECRET=上一步自动生成的随机值
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

AI、Stripe、S3/R2 均为可选能力，暂时不用可以保持空值。

### 3. 首次构建和启动

```powershell
docker compose --env-file .env -f docker\compose.yaml up --build -d
```

首次运行会下载并构建镜像，后续日常启动不会重复安装这些内容。

检查状态：

```powershell
docker compose --env-file .env -f docker\compose.yaml ps
```

等待以下四个服务全部显示 `healthy`：

- `nexaflow-postgres-1`
- `nexaflow-redis-1`
- `nexaflow-backend-1`
- `nexaflow-frontend-1`

### 4. 完成浏览器安装向导

打开：

```text
http://localhost:3000
```

未安装时根地址自动进入 `/install`。按顺序完成：

1. 阅读欢迎信息
2. 检查 PostgreSQL、Redis、环境变量和持久化目录
3. 确认可选 AI、支付和存储能力
4. 创建首个超级管理员
5. 设置企业名称、行业、默认语言和时区
6. 完成安装并进入管理后台

安装成功后，数据库和上传文件保存在 Docker named volumes 中。关闭 Docker Desktop或重启电脑不会丢失数据，也不需要重新安装。

## 日常启动和停止

Docker Desktop 已关闭时：

```powershell
Set-Location F:\spacex\NexaFlow
docker desktop start
docker desktop status
docker compose --env-file .env -f docker\compose.yaml up -d
```

Docker Desktop 已经运行时，只需：

```powershell
Set-Location F:\spacex\NexaFlow
docker compose --env-file .env -f docker\compose.yaml up -d
```

日常启动不要加 `--build`。登录地址：

```text
http://localhost:3000/login
```

后台地址：

```text
http://localhost:3000/admin
```

停止应用但保留全部数据：

```powershell
docker compose --env-file .env -f docker\compose.yaml stop
```

停止并移除容器，但保留 named volumes 数据：

```powershell
docker compose --env-file .env -f docker\compose.yaml down
```

不要执行 `docker compose down -v`，`-v` 会删除 PostgreSQL、Redis 和 NexaFlow 持久化卷。

## 更新代码后的重新部署

拉取或修改代码后，需要重新构建应用镜像：

```powershell
Set-Location F:\spacex\NexaFlow
docker compose --env-file .env -f docker\compose.yaml up --build -d
docker compose --env-file .env -f docker\compose.yaml ps
```

仅前端有变化时可以执行：

```powershell
docker compose --env-file .env -f docker\compose.yaml build frontend
docker compose --env-file .env -f docker\compose.yaml up -d --no-deps frontend
```

浏览器仍显示旧界面时按 `Ctrl + F5` 强制刷新。

## 地址和健康检查

| 用途 | 地址 |
| --- | --- |
| Web 根地址 | <http://localhost:3000> |
| 首次安装 | <http://localhost:3000/install> |
| 登录 | <http://localhost:3000/login> |
| 管理后台 | <http://localhost:3000/admin> |
| API 存活检查 | <http://localhost:8080/health/live> |
| API 就绪检查 | <http://localhost:8080/health/ready> |
| 版本化 API 健康检查 | <http://localhost:8080/api/v1/health> |

查看日志：

```powershell
docker compose --env-file .env -f docker\compose.yaml logs -f backend frontend
```

## 常见问题

### 端口已被占用

本地 PostgreSQL 或 Redis 服务可能占用 `5432`/`6379`。先确认端口占用，再停止冲突服务或修改 Compose 端口。Windows PostgreSQL 18 的服务名常见为：

```powershell
Stop-Service postgresql-x64-18
```

执行 `Stop-Service` 可能需要管理员 PowerShell。不要在没有冲突时每次都停止本机服务。

### Redis 显示 unhealthy

```powershell
docker compose --env-file .env -f docker\compose.yaml logs redis
docker compose --env-file .env -f docker\compose.yaml up -d --force-recreate redis
```

### 打开首页却不是后台

当前版本根地址会根据安装状态跳转：未安装进入 `/install`，已安装进入 `/login`。若浏览器缓存了旧页面，按 `Ctrl + F5`，或直接访问 `/login`、`/admin`。

### 查看容器状态

```powershell
docker desktop status
docker compose --env-file .env -f docker\compose.yaml ps
```

## 本地开发

本地工具链：Go 1.26.5、Node.js 22、pnpm 11、PostgreSQL 18、Redis 8。

后端：

```powershell
Set-Location backend
go mod download
go test ./...
go run ./cmd/server
```

前端（另一个 PowerShell）：

```powershell
Set-Location frontend
corepack enable
corepack pnpm install --frozen-lockfile
corepack pnpm lint
corepack pnpm typecheck
corepack pnpm dev
```

完整质量检查：

```powershell
Set-Location F:\spacex\NexaFlow
.\scripts\verify.ps1
```

## 生产部署

`docker/compose.yaml` 默认面向单机安装和本地验证。生产环境至少需要：

1. 使用独立随机的 PostgreSQL、Redis 和 JWT 密钥，并通过 Secret Manager/Docker Secrets 注入。
2. 在反向代理或负载均衡器终止 HTTPS，只对外开放 Web 服务端口。
3. 将 `CORS_ALLOWED_ORIGINS` 设置为准确的 HTTPS 前端域名。
4. 构建前端镜像时将 `NEXT_PUBLIC_API_URL` 设置为公网 HTTPS API 地址。
5. 不要向公网暴露 PostgreSQL `5432` 和 Redis `6379`。
6. PostgreSQL 必须支持 pgvector，并建立定期备份和恢复演练。
7. 对对象存储启用版本管理；使用本地存储时备份 `nexaflow_data`。
8. Redis 开启认证、持久化、内存限制和适当的淘汰策略。
9. 将 JSON 日志、指标和告警接入外部可观测平台。
10. 发布前执行后端测试、Go vet、前端 Lint、类型检查和生产构建。

生产环境变量示例：

```dotenv
APP_ENV=production
NEXT_PUBLIC_API_URL=https://api.example.com/api/v1
CORS_ALLOWED_ORIGINS=https://app.example.com
JWT_SECRET=<至少32字符的随机密钥>
STORAGE_PROVIDER=s3
STORAGE_ENDPOINT=<对象存储地址>
STORAGE_BUCKET=<存储桶>
```

Named volumes 不是备份。生产恢复必须依赖经过验证的 PostgreSQL 备份和对象存储版本。

更多说明：

- [安装设计](docs/installation.md)
- [API 文档](docs/api.md)
- [系统架构](docs/architecture.md)
- [数据库设计](docs/database.md)
- [开发指南](docs/development.md)
- [部署指南](docs/deployment.md)
- [安全边界](docs/security.md)
- [插件扩展](docs/plugin.md)

## License

Apache License 2.0，参见 [LICENSE](LICENSE)。
