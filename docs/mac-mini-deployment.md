# OpenChore Mac mini 原生部署与运维手册（非 Docker / PM2）

本文档详细记录在 macOS (Mac mini / Apple Silicon 或 Intel) 上采用原生方式（无 Docker）部署与运维 OpenChore 的完整步骤。

---

## 架构概览

```
[浏览器 / 客户端 (手机/平板/电脑)]
                │
                ▼ (HTTP :8088)
        ┌───────────────┐
        │     Nginx     │ ── 静态资源 ──> web/dist (React SPA)
        └───────┬───────┘
                │ 反向代理 (/api/, /uploads/, /tts/)
                ▼ (HTTP :8080)
        ┌───────────────┐
        │  Go Backend   │ (PM2 守护管理，服务名 openchore-api)
        └───────┬───────┘
                │
        ┌───────┴───────┐
        ▼               ▼
     SQLite        PostgreSQL
 (单机 openchore.db)  (局域网独立数据库)
```

- **前端**：React 18 + Vite 构建的单页应用（SPA），构建产物位于 `web/dist`，由 Nginx 托管。
- **后端**：Go 原生二进制文件（内置 SQLite/Postgres 迁移驱动），监听 `127.0.0.1:8080`，由 PM2 守护。
- **反向代理**：Nginx 监听对外端口（如 `8088`），转发接口请求与静态文件，避免端口冲突与跨域问题。

---

## 一、首次部署环境准备

### 1. 安装基础开发工具链
打开终端执行：

```bash
# 1. 安装 Xcode 命令行工具
xcode-select --install

# 2. 安装 Homebrew 包管理器（若已安装跳过）
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 3. 安装 Go、Node.js 与 Nginx
brew install go node nginx

# 4. 全局安装 PM2 进程守护工具
npm install -g pm2
```

### 2. 配置国内镜像加速（重要）
国内网络环境下必须配置 Go 代理与 npm 镜像，否则依赖下载会阻塞挂起：

```bash
# 配置 Go 国内代理（七牛云镜像）
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GO111MODULE=on

# （可选）配置 npm 镜像加速
npm config set registry https://registry.npmmirror.com
```

---

## 二、项目配置与初始化

### 1. 获取代码
```bash
mkdir -p ~/Code/github
cd ~/Code/github
git clone <你的git仓库地址> openchore
cd openchore
```

### 2. 配置环境变量 `.env`
复制配置模板并编辑：
```bash
cp .env.example .env
nano .env
```
根据需求配置数据库（二选一）：

#### 选项 A：使用内置 SQLite（单机默认，轻量）
```env
PORT=8080
CONFIG_PATH=config/config.yaml
DB_TYPE=sqlite
DB_PATH=openchore.db
```

#### 选项 B：使用独立 PostgreSQL 数据库
```env
PORT=8080
CONFIG_PATH=config/config.yaml
DB_TYPE=postgres
POSTGRES_HOST=192.168.199.11
POSTGRES_PORT=5432
POSTGRES_USER=admin
POSTGRES_PASSWORD=asb@1234
POSTGRES_DATABASE=openchore
POSTGRES_SSLMODE=disable
```

### 3. 配置家庭初始化数据 `config/config.yaml`
首次启动且数据库为空时，Go 后端会自动加载此文件导入家庭成员与任务：
```bash
cp config/config.example.yaml config/config.yaml
# 可按需自定义 PIN 码、成员与任务
nano config/config.yaml
```

---

## 三、编译前后端

在项目根目录（`~/Code/github/openchore`）下执行：

```bash
# 1. 编译 Go 后端二进制文件
go mod download
CGO_ENABLED=0 go build -o server cmd/server/main.go

# 2. 安装前端依赖并打包构建
cd web
npm install
npm run build
cd ..
```

---

## 四、配置 Nginx 静态托管与反向代理

### 1. 解决 Nginx 默认 8080 端口冲突（关键避坑）
Homebrew 安装的 Nginx 默认会在 `8080` 端口开启一个演示服务，会与 Go 后端端口冲突并导致请求被 Nginx 自身拦截返回 404。

编辑主配置文件：
```bash
nano $(brew --prefix)/etc/nginx/nginx.conf
```
找到 `server { listen 8080; ... }` 区块，**将其整段注释或删除**：
```nginx
    # 将自带的 8080 示例配置注释掉：
    # server {
    #     listen       8080;
    #     server_name  localhost;
    #     location / {
    #         root   html;
    #         index  index.html index.htm;
    #     }
    # }

    include servers/*;
}
```

### 2. 添加 OpenChore 虚拟主机配置
创建配置文件：
```bash
mkdir -p $(brew --prefix)/etc/nginx/servers
nano $(brew --prefix)/etc/nginx/servers/openchore.conf
```

贴入以下配置（**注意将 root 路径中的项目路径替换为实际绝对路径**）：
```nginx
server {
    # 局域网对外访问端口
    listen 8088;
    server_name _;

    # 前端构建产物目录（注意替换真实绝对路径）
    root /Users/danzz/Code/github/openchore/web/dist;
    index index.html;
    client_max_body_size 20m;

    # 1. API 接口反代到本地 Go 后端（注意：8080 末尾千万不要加斜杠）
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }

    # 2. 上传图片与文件反代
    location /uploads/ {
        proxy_pass http://127.0.0.1:8080;
    }

    # 3. 语音朗读 TTS 反代
    location /tts/ {
        proxy_pass http://127.0.0.1:8080;
    }

    # 4. 前端 SPA 单页应用路由兜底
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### 3. 测试并启动 Nginx
```bash
# 测试语法有效性
nginx -t

# 启动 Nginx 并注册开机自启服务
brew services start nginx

# 若已有运行中的 Nginx，执行重载
nginx -s reload
```

---

## 五、使用 PM2 守护后端并配置开机自启

```bash
cd ~/Code/github/openchore

# 1. 启动 Go 后端服务
pm2 start ./server --name openchore-api

# 2. 查看进程运行状态与输出日志
pm2 status
pm2 logs openchore-api

# 3. 保存当前的 PM2 进程列表
pm2 save

# 4. 生成开机自启动脚本并按终端提示执行
pm2 startup
# 终端会输出一条带有 sudo env PATH=... 的命令，复制并执行它即可完成开机自启配置
```

---

## 六、验证访问

1. **查看 Mac mini 局域网 IP**：
   ```bash
   ipconfig getifaddr en0
   ```
   *(例如获取到 `192.168.1.100`)*

2. **验证接口与页面**：
   - 本地接口测试：`curl http://127.0.0.1:8088/api/users`
   - 浏览器访问：在同一局域网的手机、iPad 或电脑浏览器中打开：
     ```
     http://<Mac-mini-IP>:8088
     ```

---

## 七、日常运维与后续更新流程

### 1. 代码拉取与快速更新
当有新功能或 Bug 修复时，在项目目录下执行：

```bash
cd ~/Code/github/openchore

# 1. 拉取最新代码
git pull

# 2. 重新编译 Go 后端并热重启
go build -o server cmd/server/main.go
pm2 restart openchore-api

# 3. 重新构建前端（仅前端代码变动时需要）
cd web
npm install
npm run build
cd ..

# 4. 如果改动了 Nginx 配置才需要重载
nginx -s reload
```

### 2. 常用监控与维护命令

| 操作 | 命令 |
| :--- | :--- |
| **查看后端状态** | `pm2 status` |
| **实时查看后端日志** | `pm2 logs openchore-api` |
| **查看最近 200 行日志** | `pm2 logs openchore-api --lines 200` |
| **重启后端服务** | `pm2 restart openchore-api` |
| **停止后端服务** | `pm2 stop openchore-api` |
| **查看 Nginx 状态** | `brew services list` |
| **重载 Nginx 配置** | `nginx -s reload` |
| **重启 Nginx 服务** | `brew services restart nginx` |
| **查看 Nginx 错误日志** | `tail -f $(brew --prefix)/var/log/nginx/error.log` |

### 3. 数据备份
- **SQLite 模式**：
  定期备份项目根目录下的 `openchore.db` 文件以及 `data/uploads` 目录即可。
- **PostgreSQL 模式**：
  直接在 PostgreSQL 所在机器执行 `pg_dump -U admin openchore > openchore_backup.sql`。
