# Research Findings

## 审阅总结

### 问题根因
后端返回的图表数据使用下划线格式的 `LabelKey`，但前端翻译文件使用 camelCase 格式的键，导致翻译失败。

### 需要修改的文件清单

#### 1. 后端 Handler (internal/handler/dashboard_handler.go)
**6处 LabelKey 修改：**
```go
// Line 436: 成功请求
"dashboard.success_requests" → "dashboard.successRequests"

// Line 441: 失败请求
"dashboard.failed_requests" → "dashboard.failedRequests"

// Line 602: 非缓存输入token
"dashboard.non_cached_prompt_tokens" → "dashboard.nonCachedPromptTokens"

// Line 607: 缓存token
"dashboard.cached_tokens" → "dashboard.cachedTokens"

// Line 612: 完成token
"dashboard.completion_tokens" → "dashboard.completionTokens"

// Line 617: 总token
"dashboard.total_tokens" → "dashboard.totalTokens"
```

#### 2. 后端 Go 翻译文件（3个语言文件）

##### zh-CN.go
```go
// 添加新键（4个）
"dashboard.nonCachedPromptTokens": "输入",
"dashboard.cachedTokens": "缓存",
"dashboard.completionTokens": "输出",
"dashboard.totalTokens": "总量",

// 修改键名（2个）
"dashboard.success_requests" → "dashboard.successRequests"
"dashboard.failed_requests" → "dashboard.failedRequests"
```

##### en-US.go
```go
// 添加新键（4个）
"dashboard.nonCachedPromptTokens": "Input",
"dashboard.cachedTokens": "Cached",
"dashboard.completionTokens": "Output",
"dashboard.totalTokens": "Total",

// 修改键名（2个）
"dashboard.success_requests" → "dashboard.successRequests"
"dashboard.failed_requests" → "dashboard.failedRequests"
```

##### ja-JP.go
```go
// 添加新键（4个）
"dashboard.nonCachedPromptTokens": "入力",
"dashboard.cachedTokens": "キャッシュ",
"dashboard.completionTokens": "出力",
"dashboard.totalTokens": "合計",

// 修改键名（2个）
"dashboard.success_requests" → "dashboard.successRequests"
"dashboard.failed_requests" → "dashboard.failedRequests"
```

#### 3. 前端 TS 翻译文件（3个语言文件）

##### zh-CN.ts
```typescript
// 修改键名（2个）
success_requests: "成功请求" → successRequests: "成功请求"
failed_requests: "失败请求" → failedRequests: "失败请求"
```

##### en-US.ts
```typescript
// 修改键名（2个）
success_requests: "Success Requests" → successRequests: "Success Requests"
failed_requests: "Failed Requests" → failedRequests: "Failed Requests"
```

##### ja-JP.ts
```typescript
// 修改键名（2个）
success_requests: "成功リクエスト" → successRequests: "成功リクエスト"
failed_requests: "失敗リクエスト" → failedRequests: "失敗リクエスト"
```

### 修改统计
| 类型 | 文件数 | 修改次数 |
|------|--------|---------|
| 后端 Go | 4 | 12处 |
| 前端 TS | 3 | 6处 |
| 前端 Vue | 0 | 0处 |
| **总计** | **7** | **18处** |

### 命名映射表

| 用途 | snake_case | camelCase |
|------|-----------|-----------|
| 非缓存输入token | non_cached_prompt_tokens | nonCachedPromptTokens |
| 缓存token | cached_tokens | cachedTokens |
| 完成token | completion_tokens | completionTokens |
| 总token | total_tokens | totalTokens |
| 成功请求 | success_requests | successRequests |
| 失败请求 | failed_requests | failedRequests |

### 验证检查点
1. [ ] 后端 handler 的 6 个 LabelKey 改为 camelCase
2. [ ] 后端 zh-CN.go 添加 4 个新键，修改 2 个键名
3. [ ] 后端 en-US.go 添加 4 个新键，修改 2 个键名
4. [ ] 后端 ja-JP.go 添加 4 个新键，修改 2 个键名
5. [ ] 前端 zh-CN.ts 修改 2 个键名
6. [ ] 前端 en-US.ts 修改 2 个键名
7. [ ] 前端 ja-JP.ts 修改 2 个键名
8. [ ] 验证图表标签正确显示翻译
