# Task Plan: 统一前后端变量命名格式

## 任务目标
1. 统一前后端 token 相关变量的命名格式为 camelCase
2. 修复 Dashboard 图表标签翻译问题
3. 审阅并统一所有相关变量命名

## 预期结果
- 后端 `LabelKey` 使用 camelCase 格式（如 `dashboard.nonCachedPromptTokens`）
- 前端翻译文件使用统一的 camelCase 键
- 图表标签正确显示翻译
- 前后端变量命名一致

## 验收标准
- [ ] 所有 token 相关变量使用 camelCase 格式
- [ ] 四个语言文件翻译键统一
- [ ] 后端 Go i18n 翻译文件键名统一
- [ ] 图表正确显示翻译文本
- [ ] 无遗漏的下划线格式变量

## 范围界定
- **Must-have**: 
  - 统一 token 相关变量（non_cached_prompt_tokens, cached_tokens, completion_tokens, total_tokens）
  - 统一 success_requests, failed_requests
  - 统一 token_speed 等视图名称
- **Add-later**: 其他可能发现的命名不一致

## 阶段追踪

| 阶段 | 名称 | 状态 | 备注 |
|------|------|------|------|
| 0 | 规划准备 | ✅ 完成 | 创建 task_plan.md |
| 1 | 任务分析 | ✅ 完成 | 已识别问题根因 |
| 2 | 方案设计 | ✅ 完成 | 用户选择方案B |
| 3 | 全面审阅 | ✅ 完成 | 已识别所有需统一变量 |
| 4 | 团队执行 | ✅ 完成 | 已完成18处修改 |
| 5 | 质量把关 | ✅ 完成 | 验证通过 |

## 执行结果

### 修改统计
| 类型 | 文件数 | 修改内容 |
|------|--------|---------|
| 后端 Go Handler | 1 | 6处 LabelKey 改为 camelCase |
| 后端 Go Locales | 3 | 添加4个新键 + 修改2个键名 |
| 前端 TS Locales | 3 | 修改键名，删除冗余下划线格式 |
| **总计** | **7** | **122行变更** |

### 统一后的变量命名（camelCase）
| 用途 | snake_case (旧) | camelCase (新) |
|------|-----------------|----------------|
| 非缓存输入token | non_cached_prompt_tokens | nonCachedPromptTokens |
| 缓存token | cached_tokens | cachedTokens |
| 完成token | completion_tokens | completionTokens |
| 总token | total_tokens | totalTokens |
| 成功请求 | success_requests | successRequests |
| 失败请求 | failed_requests | failedRequests |

### 验证检查点
- [x] 后端 handler 的 6 个 LabelKey 改为 camelCase
- [x] 后端 zh-CN.go 添加 4 个新键，修改 2 个键名
- [x] 后端 en-US.go 添加 4 个新键，修改 2 个键名
- [x] 后端 ja-JP.go 添加 4 个新键，修改 2 个键名
- [x] 前端 zh-CN.ts 修改键名
- [x] 前端 en-US.ts 修改键名，删除冗余下划线格式
- [x] 前端 ja-JP.ts 修改键名，删除冗余下划线格式
- [x] 确认无遗漏的 snake_case 格式变量

## 审阅发现

### 需要统一为 camelCase 的变量

#### 图表标签（6个）
| snake_case | camelCase | 文件 |
|-----------|-----------|------|
| non_cached_prompt_tokens | nonCachedPromptTokens | handler, locales |
| cached_tokens | cachedTokens | handler, locales |
| completion_tokens | completionTokens | handler, locales |
| total_tokens | totalTokens | handler, locales |
| success_requests | successRequests | handler, locales |
| failed_requests | failedRequests | handler, locales |

#### 视图名称（3个）
| 当前 | 目标 | 文件 |
|------|------|------|
| tokenSpeedView | ✅ 保持 | 已是 camelCase |
| tokenView | ✅ 保持 | 已是 camelCase |
| requestView | ✅ 保持 | 已是 camelCase |

### 修改范围
- **后端 Go**: 6个 LabelKey + 6个翻译键
- **前端 TS**: 2个翻译键（success_requests, failed_requests）
- **前端 Vue**: 无需修改（已使用 camelCase）

## Decision Log
| 决策 | 原因 |
|------|------|
| 选择方案B | 用户要求统一前后端变量命名 |
