// 通用 API 响应结构
export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

interface GroupConfigBase {
  timeout?: number;
  retry?: number;
}

interface GroupConfigChannelSpecific {
  openai?: {
    model?: string;
  };
  gemini?: {
    model?: string;
  };
  anthropic?: {
    model?: string;
  };
}

export type GroupConfig = GroupConfigBase & GroupConfigChannelSpecific;

// 密钥状态
export type KeyStatus = "active" | "invalid";

// 分组类型
export type GroupType = "standard" | "aggregate";

// 渠道类型
export type ChannelType = "openai" | "openai-response" | "gemini" | "anthropic";

// 数据模型定义
export interface APIKey {
  id: number;
  group_id: number;
  key_value: string;
  notes?: string;
  status: KeyStatus;
  request_count: number;
  failure_count: number;
  last_used_at?: string;
  created_at: string;
  updated_at: string;
}

export interface KeyRow extends APIKey {
  is_visible: boolean;
}

export interface UpstreamInfo {
  url: string;
  weight: number;
}

export interface HeaderRule {
  key: string;
  value: string;
  action: "set" | "remove";
}

// 子分组配置（用于创建/更新）
export interface SubGroupConfig {
  group_id: number;
  weight: number;
}

// 子分组信息（用于显示）
export interface SubGroupInfo {
  group: Group;
  weight: number;
  total_keys: number;
  active_keys: number;
  invalid_keys: number;
}

// 父聚合分组信息（用于显示）
export interface ParentAggregateGroup {
  group_id: number;
  name: string;
  display_name: string;
  weight: number;
}

// 模型映射目标配置
export interface ModelMappingTarget {
  sub_group_id: number;
  weight: number;
  sub_group_name?: string;
  model: string;
  models?: string[];
}

// 模型映射配置
export interface ModelMapping {
  model: string;
  targets: ModelMappingTarget[];
}

export interface Group {
  id?: number;
  name: string;
  display_name: string;
  description: string;
  sort: number;
  test_model: string;
  channel_type: ChannelType;
  upstreams: UpstreamInfo[];
  validation_endpoint: string;
  config?: GroupConfig;
  api_keys?: APIKey[];
  endpoint?: string;
  param_overrides?: Record<string, unknown>;
  model_redirect_rules?: Record<string, string>;
  model_redirect_strict: boolean;
  model_mapping_strict?: boolean;
  header_rules?: HeaderRule[];
  proxy_keys: string;
  group_type?: GroupType;
  sub_groups?: SubGroupInfo[];
  sub_group_ids?: number[];
  model_mappings?: string | ModelMapping[];
  model_mappings_list?: ModelMapping[];
  created_at?: string;
  updated_at?: string;
}

export interface GroupConfigOption {
  key: string;
  name: string;
  description: string;
  default_value: string | number;
}

export interface ConfigItem {
  key: string;
  value: number | string | boolean;
}

export interface HeaderRuleItem {
  key: string;
  value: string;
  action: "set" | "remove";
}

export interface GroupFormData {
  name: string;
  display_name: string;
  description: string;
  upstreams: UpstreamInfo[];
  channel_type: ChannelType;
  sort: number;
  test_model: string;
  validation_endpoint: string;
  param_overrides: string;
  model_redirect_rules: string;
  model_redirect_strict: boolean;
  model_mapping_strict: boolean;
  config: Record<string, number | string | boolean>;
  configItems: ConfigItem[];
  header_rules: HeaderRuleItem[];
  proxy_keys: string;
  group_type?: string;
}

// GroupStatsResponse 定义分组的完整统计数据
export interface GroupStatsResponse {
  key_stats: KeyStats;
  stats_24_hour: RequestStats;
  stats_7_day: RequestStats;
  stats_30_day: RequestStats;
}

// KeyStats 定义分组中 API 密钥的统计数据
export interface KeyStats {
  total_keys: number;
  active_keys: number;
  invalid_keys: number;
}

// RequestStats 定义一段时间内的请求统计数据
export interface RequestStats {
  total_requests: number;
  failed_requests: number;
  failure_rate: number;
}

export type TaskType = "KEY_VALIDATION" | "KEY_IMPORT" | "KEY_DELETE" | "GROUP_SYNC";

export interface KeyValidationResult {
  invalid_keys: number;
  total_keys: number;
  valid_keys: number;
}

export interface KeyImportResult {
  added_count: number;
  ignored_count: number;
}

export interface KeyDeleteResult {
  deleted_count: number;
  ignored_count: number;
}

export interface GroupSyncResult {
  synced_count: number;
  failed_count: number;
}

export interface TaskInfo {
  id?: number;
  task_type: TaskType;
  is_running: boolean;
  status?: "pending" | "running" | "completed" | "failed";
  group_name?: string;
  processed?: number;
  total?: number;
  started_at?: string;
  finished_at?: string;
  created_at?: string;
  updated_at?: string;
  result?: KeyValidationResult | KeyImportResult | KeyDeleteResult | GroupSyncResult;
  error?: string;
}

// 基于后端响应
export interface RequestLog {
  id: string;
  timestamp: string;
  group_id: number;
  key_id: number;
  is_success: boolean;
  source_ip: string;
  status_code: number;
  request_path: string;
  duration_ms: number;
  error_message: string;
  user_agent: string;
  request_type: "retry" | "final";
  group_name?: string;
  parent_group_name?: string;
  key_value?: string;
  model: string;
  original_model?: string; // 原始请求的模型名称
  upstream_addr: string;
  is_stream: boolean;
  request_body?: string;
  // Token 统计字段
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cached_tokens: number;
}

export interface Pagination {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface LogsResponse {
  items: RequestLog[];
  pagination: Pagination;
}

export interface LogFilter {
  page?: number;
  page_size?: number;
  group_name?: string;
  parent_group_name?: string;
  key_value?: string;
  model?: string;
  is_success?: boolean | null;
  status_code?: number | null;
  source_ip?: string;
  error_contains?: string;
  start_time?: string | null;
  end_time?: string | null;
  request_type?: "retry" | "final";
}

export interface DashboardStats {
  total_requests: number;
  success_requests: number;
  success_rate: number;
  group_stats: GroupRequestStat[];
}

export interface GroupRequestStat {
  display_name: string;
  request_count: number;
}

// 仪表盘统计卡片数据
export interface StatCard {
  value: number;
  sub_value?: number;
  sub_value_tip?: string;
  trend: number;
  trend_is_growth: boolean;
}

// 安全警告信息
export interface SecurityWarning {
  type: string; // 警告类型：auth_key、encryption_key 等
  message: string; // 警告消息
  severity: string; // 严重程度：low、medium、high
  suggestion: string; // 建议的解决方案
}

// 仪表盘基础统计响应
export interface DashboardStatsResponse {
  key_count: StatCard;
  token_consumption: StatCard;
  prompt_tokens: StatCard;
  non_cached_prompt_tokens: StatCard;
  cached_tokens: StatCard;
  completion_tokens: StatCard;
  total_tokens: StatCard;
  rpm: StatCard;
  request_count: StatCard;
  error_rate: StatCard;
  security_warnings: SecurityWarning[];
}

// 图表数据集
export interface ChartDataset {
  label: string;
  label_key?: string;
  data: number[];
}

// 图表数据
export interface ChartData {
  labels: string[];
  datasets: ChartDataset[];
}

// 仪表盘图表视图类型
export type ChartViewType = "request" | "token" | "token_speed";
