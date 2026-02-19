# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此仓库中工作时提供指导。

## 项目概述

GPT-Load 是一个高性能的 AI API 透明代理服务，使用 Go 语言（Gin + GORM）后端和 Vue 3 + TypeScript 前端构建。它支持 OpenAI、Google Gemini 和 Anthropic Claude 等多种 AI 服务提供商，具有智能密钥管理、负载均衡和故障恢复功能。

## 构建、检查和测试命令

### 后端（Go）
- `go run ./main.go` - 直接运行后端
- `go build -o gpt-load .` - 构建二进制文件
- `go test ./...` - 运行所有测试（如果可用）
- `go test ./internal/package -run TestName` - 运行特定测试

### 前端（Vue 3）
- `cd web && npm run dev` - 启动开发服务器
- `cd web && npm run build` - 生产环境构建
- `cd web && npm run preview` - 预览构建产物
- `cd web && npm run lint` - 修复 lint 问题
- `cd web && npm run lint:check` - 检查 lint 不修复
- `cd web && npm run format` - 使用 Prettier 格式化
- `cd web && npm run format:check` - 检查格式不修复
- `cd web && npm run type-check` - TypeScript 类型检查
- `cd web && npm run check-all` - 运行所有检查（lint、format、type-check）

## 核心架构

### 分层架构

项目采用依赖注入（`go.uber.org/dig`）进行分层架构设计：

1. **基础设施层 (Infrastructure)**
   - `internal/config` - 配置管理（环境变量 + 数据库动态配置）
   - `internal/db` - 数据库连接和迁移
   - `internal/store` - 存储抽象（Redis/内存）
   - `internal/encryption` - API 密钥加密服务
   - `internal/httpclient` - HTTP 客户端管理
   - `internal/commands` - 命令行处理（密钥迁移等）
   - `internal/errors` - 错误类型定义
   - `internal/i18n` - 国际化支持
   - `internal/response` - 统一响应格式
   - `internal/types` - 通用类型定义
   - `internal/utils` - 工具函数
   - `internal/version` - 版本信息

2. **业务服务层 (Business Services)**
   - `internal/services` - 核心业务逻辑
   - `internal/keypool` - 密钥池和密钥选择（原子操作、轮换、故障处理）
   - `internal/channel` - AI 服务商通道抽象（OpenAI/Gemini/Anthropic）

3. **处理层 (Handlers)**
   - `internal/handler` - HTTP API 处理器
   - `internal/proxy` - 代理服务器核心逻辑
   - `internal/router` - 路由配置
   - `internal/middleware` - 中间件（认证、CORS、限流等）

4. **应用层 (Application)**
   - `internal/app` - 应用生命周期管理
   - `main.go` - 入口点

### 依赖注入容器

所有依赖通过 `internal/container/container.go` 中的 `BuildContainer()` 函数注册。添加新服务时需要在此注册。

### 主从节点架构

- **Master 节点** (`IS_SLAVE=false` 或未设置): 执行数据库迁移、密钥验证、日志清理等后台任务
- **Slave 节点** (`IS_SLAVE=true`): 仅处理代理请求，不执行后台任务
- 所有节点共享同一数据库和 Redis

### 核心概念

#### 1. 组 (Group)
- 普通组: 包含同一 AI 服务商的多个 API 密钥
- 聚合组 (Aggregate): 可包含多个子组，支持模型映射路由

#### 2. 密钥池 (Key Pool)
- 原子性密钥选择和轮换
- 自动故障检测和黑名单管理
- 支持密钥加密存储

#### 3. 通道 (Channel)
- `ChannelProxy` 接口抽象不同 AI 服务商的 API 格式
- 实现于 `internal/channel/`: `base_channel.go`, `openai_channel.go`, `gemini_channel.go`, `anthropic_channel.go`
- 通过 `Factory` 模式创建（`factory.go`）

#### 4. 代理流程
1. 请求到达 `/proxy/{group_name}/*path`
2. 认证中间件验证代理密钥
3. 根据模型映射选择子组（聚合组）
4. 从密钥池原子性地选择一个可用密钥
5. 通过对应通道转发请求到上游服务
6. 记录日志、更新密钥状态

## 配置系统

### 静态配置（环境变量）
- 数据库连接、服务器端口、认证密钥等
- 需要重启应用才能生效

### 动态配置（热重载）
- 系统设置和组配置存储在数据库
- 修改后立即生效，无需重启
- 优先级: 组配置 > 系统设置 > 环境配置

## 数据库迁移

- 使用 GORM AutoMigrate
- 迁移文件在 `internal/db/migrations/`
- `HandleLegacyIndexes()` 处理旧版本索引

## 前端

- Vue 3 + TypeScript + Vite
- 源码在 `web/src/`
- 构建产物嵌入到 Go 二进制文件 (`go:embed`)
- 开发时先构建前端再运行后端

## 核心功能

### Token 统计
- 支持 OpenAI 和 Claude 的 token 统计
- **Prompt caching tokens**:
  - Claude: `input_tokens` + `cache_read_input_tokens`
  - OpenAI: `prompt_tokens` + `prompt_tokens_details.cached_tokens`
- 统计字段：`prompt_tokens`, `completion_tokens`, `total_tokens`, `cached_tokens`
- 通用解析逻辑：自动识别 `prompt_tokens`/`input_tokens` 和 `completion_tokens`/`output_tokens`，支持所有兼容格式

### 模型映射与重定向
- **Model Mapping**: 将请求模型映射到不同子组，实现按模型分配流量
- **Model Redirect**: 直接重定向模型名称到目标模型
- `model_redirect_rules`: 模型重定向规则
- `model_redirect_strict`: 严格模式，未匹配规则时拒绝请求

### 密钥管理
- 支持批量导入密钥（文件上传）
- 密钥备注功能（`notes` 字段）
- 密钥哈希存储（`key_hash`）用于快速查找
- 密钥验证和自动故障转移

### 请求日志
- 区分原始模型（`original_model`）和实际模型（`model`）
- 支持 retry/final 请求类型
- Token 使用统计
- 请求体日志记录（可配置）

## 代码风格指南

### Go 后端
- **缩进**: 制表符（等同于 4 个空格）
- **行长度**: 无严格限制，优先考虑可读性
- **命名规范**:
  - 导出: PascalCase（例如 `type Server struct`, `func NewServer()`）
  - 未导出: camelCase（例如 `type config struct`, `func handleError()`）
  - 常量: TitleCase 或 camelCase（例如 `KeyStatusActive`）
  - 接口: 简单名称（例如 `Handler`, `Service`）
- **导入**: 标准库在前，第三方库在后，除非必要否则不使用别名
- **错误处理**: 始终检查错误，使用 logrus 记录日志，返回有意义的错误
- **注释**: 仅包级别和导出函数需要注释，不使用行内注释
- **结构体标签**: JSON 使用 snake_case，使用 json 和 gorm 标签保持一致

### Vue 3 前端
- **缩进**: 2 个空格
- **行结尾**: LF（Unix 风格）
- **组件**:
  - 文件名: PascalCase（例如 `KeyTable.vue`, `GroupFormModal.vue`）
  - 模板: kebab-case 属性（例如 `@click`, `:model-value`）
  - Props: camelCase（例如 `modelValue`, `showDialog`）
- **导入**: 使用 `@` 别名指向 src 目录（例如 `@/components/NavBar.vue`）
- **API 调用**: 使用 `@/api/` 中的服务函数（例如 `@/api/keys.ts`）
- **类型**:
  - 接口: PascalCase（例如 `interface APIKey`）
  - 类型别名: PascalCase（例如 `type KeyStatus`）
  - 使用显式类型，避免 `any`（ESLint 会警告）
- **Composition API**: 优先使用 `<script setup lang="ts">`，从 vue 使用 ref/reactive
- **状态管理**: 使用 `composables/` 文件夹中的 composables（例如 `useLoading.ts`, `useApi.ts`）
- **HTTP**: 使用 `@/utils/http.ts` 中的 axios 实例，自动注入认证 token
- **错误显示**: 使用 `window.$message.error()` 或 `window.$message.success()`
- **i18n**: 所有面向用户的字符串使用 `i18n.global.t('key')`
- **未使用变量**: 使用下划线 `_` 前缀忽略 ESLint 警告

### 文件组织
- **后端**:
  - `internal/models/` - 数据库模型和结构体
  - `internal/handler/` - HTTP 请求处理器
  - `internal/services/` - 业务逻辑
  - `internal/middleware/` - HTTP 中间件
  - `internal/channel/` - AI 服务商通道
  - `internal/keypool/` - 密钥管理
  - `internal/db/` - 数据库和迁移
  - `internal/proxy/` - 代理核心逻辑和 token 解析
  - `internal/commands/` - 命令行处理
  - `internal/errors/` - 错误类型定义
  - `internal/i18n/` - 国际化支持
  - `internal/response/` - 统一响应格式
  - `internal/types/` - 通用类型定义
  - `internal/utils/` - 工具函数
  - `internal/version/` - 版本信息
  - `internal/syncer/` - 配置同步
- **前端**:
  - `src/api/` - API 服务函数
  - `src/components/` - 可复用的 Vue 组件
  - `src/views/` - 页面级组件
  - `src/router/` - 路由配置
  - `src/services/` - 业务服务层
  - `src/types/` - TypeScript 接口
  - `src/composables/` - Composition API 钩子
  - `src/utils/` - 工具函数
  - `src/locales/` - i18n 翻译

### EditorConfig 规则
- UTF-8 编码
- LF 行结尾
- 去除尾随空格（Markdown 除外）
- 插入最终换行符
- Go: 制表符缩进
- JS/TS/Vue: 2 空格缩进

## 依赖管理
- **Go**: 使用 `go mod tidy` 清理依赖，固定到特定版本
- **前端**: 使用 `npm install` 安装依赖，查看 package.json 获取版本

## 测试说明
- 当前仓库未配置自动化测试
- 建议通过 localhost:3001 的 Web 界面进行手动测试
- 使用 curl 或 Postman 测试 API 端点，使用配置的代理密钥

## 安全注意事项
- 永远不要提交 AUTH_KEY 或 ENCRYPTION_KEY 值
- 使用环境变量配置敏感信息
- 当设置 ENCRYPTION_KEY 时，API 密钥在存储时会加密
- 使用 `crypto/subtle.ConstantTimeCompare` 进行密码/密钥比较
- 验证所有用户输入，使用 GORM 绑定标签

## 常见模式
- Go: 使用依赖注入配合 `go.uber.org/dig` 容器
- Go: 结构体字段使用 `gorm:"-"` 标记非持久化字段
- Vue: 使用 composables 处理可复用逻辑（加载状态、API 调用）
- Vue: 类型定义镜像后端模型（snake_case 到 camelCase 转换）
- 使用常量表示状态值（例如 `KeyStatusActive = "active"`）

## 关键文件位置

| 功能 | 文件 |
|------|------|
| 应用入口 | `main.go` |
| 依赖注入 | `internal/container/container.go` |
| 应用生命周期 | `internal/app/app.go` |
| 代理核心逻辑 | `internal/proxy/server.go` |
| Token 解析 | `internal/proxy/token_parser.go` |
| 密钥池 | `internal/keypool/provider.go` |
| 路由配置 | `internal/router/router.go` |
| AI 通道 | `internal/channel/*.go` |
| 通道工厂 | `internal/channel/factory.go` |
| API 处理器 | `internal/handler/*.go` |
| 数据库模型 | `internal/models/types.go` |
| 数据库迁移 | `internal/db/migrations/*.go` |
| 命令处理 | `internal/commands/*.go` |
| 国际化 | `internal/i18n/locales/*.go` |
