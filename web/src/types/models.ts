// Generic API response structure
export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

// Key status
export type KeyStatus = "active" | "invalid" | undefined;

// Group type
export type GroupType = "standard" | "aggregate";

// Channel type
export type ChannelType = "openai" | "openai-responses" | "gemini" | "anthropic";

// Data model definition
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

export interface UpstreamInfo {
  url: string;
  weight: number;
}

export interface HeaderRule {
  key: string;
  value: string;
  action: "set" | "remove";
}

// Subgroup configuration (for create/update)
export interface SubGroupConfig {
  group_id: number;
  weight: number;
}

// Subgroup information (for display)
export interface SubGroupInfo {
  group: Group;
  weight: number;
  total_keys: number;
  active_keys: number;
  invalid_keys: number;
}

// Parent aggregate group information (for display)
export interface ParentAggregateGroup {
  group_id: number;
  name: string;
  display_name: string;
  weight: number;
}

// Model mapping target configuration
export interface ModelMappingTarget {
  sub_group_id: number;
  weight: number;
  sub_group_name?: string;
  model: string;
  models?: string[];
}

// Model mapping configuration
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
  config: Record<string, unknown>;
  api_keys?: APIKey[];
  endpoint?: string;
  param_overrides: Record<string, unknown>;
  model_redirect_rules: Record<string, string>;
  model_redirect_strict: boolean;
  model_mapping_strict?: boolean;
  header_rules?: HeaderRule[];
  proxy_keys: string;
  group_type?: GroupType;
  sub_groups?: SubGroupInfo[]; // Subgroup list (aggregate groups only)
  sub_group_ids?: number[]; // Subgroup ID list
  model_mappings?: string | ModelMapping[]; // JSON string or array (pass array for update)
  model_mappings_list?: ModelMapping[]; // Parsed array
  created_at?: string;
  updated_at?: string;
}

export interface GroupConfigOption {
  key: string;
  name: string;
  description: string;
  default_value: string | number;
}

// GroupStatsResponse defines the complete statistics for a group.
export interface GroupStatsResponse {
  key_stats: KeyStats;
  stats_24_hour: RequestStats;
  stats_7_day: RequestStats;
  stats_30_day: RequestStats;
}

// KeyStats defines the statistics for API keys in a group.
export interface KeyStats {
  total_keys: number;
  active_keys: number;
  invalid_keys: number;
}

// RequestStats defines the statistics for requests over a period.
export interface RequestStats {
  total_requests: number;
  failed_requests: number;
  failure_rate: number;
}

export type TaskType = "KEY_VALIDATION" | "KEY_IMPORT" | "KEY_DELETE";

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

export interface TaskInfo {
  task_type: TaskType;
  is_running: boolean;
  group_name?: string;
  processed?: number;
  total?: number;
  started_at?: string;
  finished_at?: string;
  result?: KeyValidationResult | KeyImportResult | KeyDeleteResult;
  error?: string;
}

// Based on backend response
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
  original_model?: string; // Original requested model name
  upstream_addr: string;
  is_stream: boolean;
  request_body?: string;
  // Token statistics fields
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

// Dashboard statistics card data
export interface StatCard {
  value: number;
  sub_value?: number;
  sub_value_tip?: string;
  trend: number;
  trend_is_growth: boolean;
}

// Security warning information
export interface SecurityWarning {
  type: string; // Warning type: auth_key, encryption_key, etc.
  message: string; // Warning message
  severity: string; // Severity: low, medium, high
  suggestion: string; // Suggested solution
}

// Dashboard basic statistics response
export interface DashboardStatsResponse {
  key_count: StatCard
  token_consumption: StatCard
  prompt_tokens: StatCard
  uncached_prompt_tokens: StatCard
  cached_tokens: StatCard
  completion_tokens: StatCard
  total_tokens: StatCard
  rpm: StatCard
  request_count: StatCard
  error_rate: StatCard
  security_warnings: SecurityWarning[]
}

// Chart dataset
export interface ChartDataset {
  label: string;
  label_key?: string;
  data: number[];
}

// Chart data
export interface ChartData {
  labels: string[];
  datasets: ChartDataset[];
}
