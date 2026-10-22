# OpenChore 开发环境部署与运维手册

本文档全面整理 OpenChore 项目在本地环境下的**开发准备、启动运行、多端联调、数据库运维、进程守护以及常见问题排查**，便于日常高效开发与部署维护。

---

## 目录
1. [技术架构概览](#一技术架构概览)
2. [环境准备与依赖安装](#二环境准备与依赖安装)
3. [配置初始化（.env 与 config.yaml）](#三配置初始化env-与-configyaml)
4. [本地运行与开发模式](#四本地运行与开发模式)
5. [多端联调与域名访问（Vite 域名放行）](#五多端联调与域名访问vite-域名放行)
6. [数据库运维与迁移](#六数据库运维与迁移)
7. [生产与常驻运维（PM2 + Nginx 无 Docker 部署）](#七生产与常驻运维pm2--nginx-无-docker-部署)
8. [质量检测与打包编译](#八质量检测与打包编译)
9. [高频问题排查与避坑指南 (FAQ)](#九高频问题排查与避坑指南-faq)

---

## 一、技术架构概览

OpenChore 是面向家庭的自托管家务管理 PWA，采用前后端分离架构：

```
[手机/平板 PWA / 浏览器]
       │
       ▼ (Vite: 5173 或 Nginx: 8088)
┌──────────────┐         反代 /api/, /uploads/, /tts/
│  前端 React  │ ───────────────────────────────────► ┌──────────────┐
│ (TypeScript) │                                      │   Go 后端    │ (监听 8080)
└──────────────┘                                      └───────┬──────┘
                                                              │
                                                      ┌───────┴───────┐
                                                      ▼               ▼
                                                   SQLite        PostgreSQL
                                                (openchore.db)  (局域网独立实例)
```

- **后端**：Go 1.25+（内置 `chi/v5` 路由，`CGO_ENABLED=0` 纯 Go 编译）。
- **前端**：React 18 + TypeScript + Vite + Vanilla CSS / CSS Modules，支持国际化（默认中文 `zh`）。
- **数据层**：
  - **SQLite**（默认）：单文件嵌入式数据库，纯 Go 驱动 `modernc.org/sqlite`，WAL 模式。
  - **PostgreSQL**：高性能独立数据库，支持局域网集中存储与多实例协作。
- **端口默认规划**：
  - `8080`：Go 后端 API 及文件上传服务端口。
  - `5173`：前端 Vite 开发热重载服务器端口。
  - `8088`：Nginx 部署托管端口（若使用反代）。

---

## 二、环境准备与依赖安装

### 1. 基础环境要求
- **Go**：1.25+（可使用 `go version` 检查）
- **Node.js**：22+ 及 npm（可使用 `node -v`、`npm -v` 检查）
- **Git**

### 2. 国内网络加速配置（避免依赖拉取超时）
在终端中执行：
```bash
# 1. 配置 Go 代理（国内七牛云镜像源）
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GO111MODULE=on

# 2. 配置 npm 淘宝镜像源
npm config set registry https://registry.npmmirror.com
```

### 3. 安装项目依赖
在项目根目录下执行：
```bash
# 一键安装后端 Go 模块与前端 npm 依赖
make install

# 或者分别手动安装：
go mod download
cd web && npm install && cd ..
```

---

## 三、配置初始化（.env 与 config.yaml）

项目运行依赖两个核心配置文件：

### 1. 运行时环境变量 `.env`
用于控制服务端口、数据库选型以及可选的 AI 服务地址。复制示例配置并调整：
```bash
cp .env.example .env
```

核心配置参数说明：
```env
# 服务端口
PORT=8080
TZ=Asia/Shanghai

# 数据库选项 A：使用内置 SQLite 单文件数据库（默认推荐，零运维）
DB_TYPE=sqlite
DB_PATH=openchore.db

# 数据库选项 B：使用独立 PostgreSQL 数据库
# DB_TYPE=postgres
# POSTGRES_HOST=192.168.199.11
# POSTGRES_PORT=5432
# POSTGRES_USER=admin
# POSTGRES_PASSWORD=your_password
# POSTGRES_DATABASE=openchore
# POSTGRES_SSLMODE=disable

# 可选 AI 模型能力（OpenAI 兼容端点，留空则关闭 AI）
# AI_BASE_URL=http://localhost:8081/v1
# AI_MODEL=gemma-4-e4b
# TTS_BASE_URL=http://localhost:8880/v1
```

### 2. 初始数据模板 `config/config.yaml`
**特别注意：`config/config.yaml` 仅在数据库完全为空（首次启动）时被导入**。一旦数据库初始化完成，所有的家庭成员、家务、奖励和规则都以数据库中的数据为准，后续可在前端“后台管理”界面直接维护。

```bash
cp config/config.example.yaml config/config.yaml
```
可在该文件中预设家庭名称、默认管理员（Alex/Jamie 及 PIN 码）、孩子列表与常用家务模板。

---

## 四、本地运行与开发模式

根据开发阶段与数据需求，支持以下三种运行模式：

### 模式 A：日常开发模式（保留现有数据）
日常开发前后端时，应**保留现有数据库数据**，避免重复录入：

```bash
# 终端 1：启动 Go 后端（热重载或直接运行）
make api
# 或：go run cmd/server/main.go

# 终端 2：启动前端 Vite 开发服务器（允许所有局域网设备接入）
npm --prefix web run dev -- --host 0.0.0.0
```
- 后端运行在：`http://localhost:8080`
- 前端运行在：`http://localhost:5173`（局域网为 `http://<本机IP>:5173`）
- Vite 会自动将 `/api/`、`/uploads/`、`/tts/` 的请求转发至后端的 `8080` 端口。

### 模式 B：全新重置模式（`make dev`）
> [!WARNING]
> `make dev` 会**主动删除本地的 `openchore.db` 文件**并从 `config/config.yaml` 重新导入种子数据！仅在需要快速重置测试状态或初始化测试数据库时使用。

```bash
make dev
```

### 模式 C：带本地 AI 侧车服务的开发模式
如果需要测试拍照打卡视觉校验与 TTS 语音朗读：
```bash
make dev-ai
```
该命令会自动通过 Docker 启动本地的 llama.cpp 与 Kokoro 容器，并设置环境变量注入后端。

---

## 五、多端联调与域名访问（Vite 域名放行）

### 1. 局域网访问与真机调试
启动前端时必须添加 `--host 0.0.0.0`，以便手机、平板在同一 Wi-Fi 下访问：
```bash
npm --prefix web run dev -- --host 0.0.0.0
```
终端会输出类似以下地址：
```
  ➜  Local:   http://localhost:5173/
  ➜  Network: http://192.168.1.50:5173/
```
手机浏览器直接打开 `http://192.168.1.50:5173/` 即可。

### 2. 自定义域名 / 反代访问报错排查（Vite 6+ Host 限制）
**现象**：
通过自定义域名（如 `kidstask.danzz.top`）、内网穿透（FRP / Cloudflare Tunnel）或 Nginx 反向代理访问开发服务器时，页面提示：
```text
Blocked request. This host ("kidstask.danzz.top") is not allowed.
To allow this host, add "kidstask.danzz.top" to `server.allowedHosts` in vite.config.js.
```

**原因**：
Vite 6+ 引入了严格的 Host 校验安全机制，默认只放行 `localhost` 与局域网 IP。

**解决方案**：
在 [web/vite.config.ts](file:///Users/danzz/Code/github/openchore/web/vite.config.ts) 中配置 `server.allowedHosts: true`（或指定域名数组）：
```ts
export default defineConfig({
  plugins: [react(), preloadFonts()],
  server: {
    // 允许任何反代或自定义域名访问（推荐开发及内网环境使用）
    allowedHosts: true,
    // 或者指定明确域名白名单：
    // allowedHosts: ['kidstask.danzz.top', 'localhost'],
    proxy: {
      '/api': { target: apiTarget, changeOrigin: true, xfwd: true },
      '/uploads': { target: apiTarget, changeOrigin: true, xfwd: true },
      '/tts': { target: apiTarget, changeOrigin: true, xfwd: true }
    }
  }
})
```

### 3. iOS / iPad 添加到主屏幕 (PWA)
OpenChore 具备完整的 Progressive Web App 支持：
1. 在 iPhone / iPad Safari 中打开 OpenChore 网址。
2. 点击 Safari 底部的 **分享** 按钮。
3. 选择 **添加到主屏幕**（Add to Home Screen）。
4. 在主屏幕以独立 App 模式运行，无浏览器地址栏，沉浸感极强。

---

## 六、数据库运维与迁移

### 1. SQLite 维护与备份
- **存储位置**：项目根目录下的 `openchore.db`（以及运行时的 `openchore.db-wal` 和 `openchore.db-shm`）。
- **备份操作**：
  直接复制 `openchore.db` 文件以及 `data/uploads`（图片文件）目录即可：
  ```bash
  mkdir -p backups/$(date +%Y%m%d)
  cp openchore.db backups/$(date +%Y%m%d)/
  cp -r data/uploads backups/$(date +%Y%m%d)/
  ```
- **还原操作**：
  在服务停止状态下，覆盖替换 `openchore.db` 并清除可能遗留的 `-wal` 和 `-shm` 文件：
  ```bash
  rm -f openchore.db-shm openchore.db-wal
  cp backups/20261021/openchore.db .
  ```

### 2. PostgreSQL 维护与备份
- **自动建库**：当配置连接 PostgreSQL 且目标数据库不存在时，系统启动会自动执行 `CREATE DATABASE openchore`。
- **导出备份**：
  ```bash
  pg_dump -h <host> -U <user> openchore > openchore_backup.sql
  ```
- **导入恢复**：
  ```bash
  psql -h <host> -U <user> -d openchore < openchore_backup.sql
  ```

### 3. 数据无损迁移（SQLite → PostgreSQL）
如果本地积累了打卡记录和积分数据，希望迁移到独立的 PostgreSQL 数据库：
1. 在 `.env` 中配置好目标 PostgreSQL 连接参数。
2. 确保项目根目录下存在包含数据的 `openchore.db`。
3. 执行一键迁移工具：
   ```bash
   make db-migrate-to-postgres
   # 或直接运行：go run cmd/migrate-db/main.go
   ```
4. 迁移程序会自动：
   - 在 PostgreSQL 中按顺序执行完整的版本升级迁移脚本。
   - 按外键依赖顺序（用户 -> 家务 -> 规则 -> 完成记录 -> 积分账本 -> 奖励等）将数据逐表转移。
   - 自动重置自增序列（`SERIAL` / `SEQUENCE`），防止后续插入主键冲突。

---

## 七、生产与常驻运维（PM2 + Nginx 无 Docker 部署）

适用于 Mac mini、家用服务器、旧电脑等主机 7x24 小时常驻运行。

### 1. 整体流程
1. 编译 Go 二进制执行文件并打包前端静态资源：
   ```bash
   make build
   # 生成项目根目录可执行文件 ./server 与前端目录 web/dist
   ```
2. 使用 PM2 守护后端 Go 进程：
   ```bash
   pm2 start ./server --name openchore-api
   pm2 save
   pm2 startup
   ```
3. Nginx 托管前端 `web/dist` 并将 `/api/` 等反向代理至 `127.0.0.1:8080`（详细配置文件见 [mac-mini-deployment.md](mac-mini-deployment.md)）。

### 2. 常用监控与运维命令速查

| 操作 | 对应命令 | 说明 |
| :--- | :--- | :--- |
| **查看后端运行状态** | `pm2 status` | 查看 CPU、内存及在线状态 |
| **实时查看后端日志** | `pm2 logs openchore-api` | 排查接口报错、打卡事件 |
| **查看最近 200 行日志** | `pm2 logs openchore-api --lines 200` | 快速翻阅历史日志 |
| **重启后端服务** | `pm2 restart openchore-api` | 修改后端编译后重启 |
| **停止/启动后端服务** | `pm2 stop openchore-api` / `pm2 start openchore-api` | 维护暂停与启动 |
| **查看 Nginx 运行状态** | `brew services list` | 检查 Nginx 是否启动 |
| **重载 Nginx 配置** | `nginx -s reload` | 修改虚拟主机配置后生效 |
| **查看端口占用** | `lsof -i :8080` 或 `lsof -i :5173` | 定位占用端口的进程 PID |

---

## 八、质量检测与打包编译

在提交代码或发布前，建议运行以下测试保证质量：

```bash
# 1. 运行 Go 后端测试（基于真实 SQLite 数据库测试，严禁 Mock）
make test

# 2. 运行前端代码构建检测（校验 TypeScript 类型与语法）
cd web && npm run build && cd ..

# 3. 运行端到端自动化测试（Playwright 自动拉起前后端服务）
make test-e2e

# 4. 全量编译验证
make build
```

---

## 九、高频问题排查与避坑指南 (FAQ)

### Q1: 启动时报 `bind: address already in use` (8080 端口被占用)
**原因**：之前运行的 `server`、Docker 容器或旧进程未完全退出。  
**排查与解决**：
```bash
# 查找占用 8080 端口的进程
lsof -i :8080
# 示例输出：COMMAND   PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
#          server  88658 danzz    3u  IPv6 0x...      0t0  TCP *:http-alt (LISTEN)

# 杀掉对应残留进程
kill -9 88658
```

### Q2: 访问前端页面报 `Blocked request. This host is not allowed`
**原因**：使用了自建域名或局域网反代访问 Vite 开发服务器，触发 Vite 6 的 Host 安全拦截。  
**解决**：检查 [web/vite.config.ts](file:///Users/danzz/Code/github/openchore/web/vite.config.ts)，确认在 `server` 配置中加入了 `allowedHosts: true`。

### Q3: 家务管理删除家务时弹窗瞬间关闭或无法弹出确认框
**原因与机制**：
1. 部分浏览器环境（如 Webview、移动端全屏模式或无头环境）对原生 `window.confirm()` 做了拦截或默认拒绝。
2. 弹窗组件的遮罩层点击逻辑若使用 `!modalRef.contains(e.target)`，当点击删除按钮导致按钮 DOM 节点被提前卸载时，冒泡判断会失效导致父级弹窗错误关闭。  
**规范方案**：
- 严格使用统一的 React 自定义对话框组件 [ConfirmModal.tsx](file:///Users/danzz/Code/github/openchore/web/src/components/ConfirmModal/ConfirmModal.tsx)，禁止在业务代码中直接调用 `window.confirm()`。
- 遮罩层仅允许判断 `e.target === e.currentTarget` 触发关闭。

### Q4: Nginx 反代后访问 API 接口返回 404
**原因**：Homebrew 默认安装的 Nginx 会在自带的 `nginx.conf` 中开启一个监听 `8080` 的示例 server，阻断了本地 Go 后端。  
**解决**：编辑 `$(brew --prefix)/etc/nginx/nginx.conf`，将默认的 `server { listen 8080; ... }` 完整注释掉，然后再执行 `nginx -s reload`。

### Q5: SQLite 报错 `database is locked`
**原因**：SQLite 在多并发写操作时发生争用。  
**解决**：OpenChore 严格保证单一写入连接池（`MaxOpenConns=1`），并开启 WAL 模式。避免在外部通过数据库客户端工具对正在运行的 `openchore.db` 进行未提交事务的编辑锁定。
