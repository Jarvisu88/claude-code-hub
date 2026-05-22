export interface User {
  id: string;
  name: string;
  email: string;
  role: "admin" | "user";
  description: string;
  enabled: boolean;
  rateLimit: number;
  dailyCostLimit: number;
  monthlyCostLimit: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateUserRequest {
  name: string;
  role: "admin" | "user";
  description?: string;
  rateLimit?: number;
  dailyCostLimit?: number;
  monthlyCostLimit?: number;
}

export interface UpdateUserRequest {
  name?: string;
  role?: "admin" | "user";
  description?: string;
  enabled?: boolean;
  rateLimit?: number;
  dailyCostLimit?: number;
  monthlyCostLimit?: number;
}

export interface ApiKey {
  id: string;
  name: string;
  key: string;
  userId: string;
  userName?: string;
  enabled: boolean;
  rateLimit: number;
  dailyCostLimit: number;
  expiresAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface CreateKeyRequest {
  name: string;
  userId: string;
  rateLimit?: number;
  dailyCostLimit?: number;
  expiresAt?: string | null;
}

export interface UpdateKeyRequest {
  name?: string;
  enabled?: boolean;
  rateLimit?: number;
  dailyCostLimit?: number;
  expiresAt?: string | null;
}

export interface Provider {
  id: string;
  name: string;
  type: string;
  endpoint: string;
  apiKey: string;
  enabled: boolean;
  weight: number;
  priority: number;
  models: string[];
  healthStatus: "healthy" | "degraded" | "down" | "unknown";
  maxRetries: number;
  timeout: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateProviderRequest {
  name: string;
  type: string;
  endpoint: string;
  apiKey: string;
  weight?: number;
  priority?: number;
  models?: string[];
  maxRetries?: number;
  timeout?: number;
}

export interface UpdateProviderRequest {
  name?: string;
  type?: string;
  endpoint?: string;
  apiKey?: string;
  enabled?: boolean;
  weight?: number;
  priority?: number;
  models?: string[];
  maxRetries?: number;
  timeout?: number;
}

export interface UsageLog {
  id: string;
  userId: string;
  userName?: string;
  keyId: string;
  keyName?: string;
  providerId: string;
  providerName?: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  cost: number;
  duration: number;
  status: number;
  errorMessage?: string;
  requestBody?: string;
  responseBody?: string;
  createdAt: string;
}

export interface UsageFilters {
  startDate?: string;
  endDate?: string;
  userId?: string;
  model?: string;
  status?: number;
  page?: number;
  pageSize?: number;
}

export interface DashboardStats {
  totalRequests: number;
  totalCost: number;
  activeUsers: number;
  activeProviders: number;
}

export interface HourlyRequestData {
  hour: string;
  count: number;
}

export interface DailyCostData {
  date: string;
  cost: number;
}

export interface RecentActivity {
  id: string;
  userName: string;
  model: string;
  cost: number;
  status: number;
  createdAt: string;
}

export interface OverviewData {
  stats: DashboardStats;
  requestsPerHour: HourlyRequestData[];
  costPerDay: DailyCostData[];
  recentActivity: RecentActivity[];
}

export interface SystemSettings {
  general: {
    siteName: string;
    adminEmail: string;
    logLevel: string;
  };
  proxy: {
    timeout: number;
    maxRetries: number;
    retryDelay: number;
    streamBufferSize: number;
  };
  rateLimit: {
    enabled: boolean;
    windowSeconds: number;
    maxRequests: number;
    maxTokensPerMinute: number;
  };
  circuitBreaker: {
    enabled: boolean;
    failureThreshold: number;
    recoveryTimeout: number;
    halfOpenMaxRequests: number;
  };
  cleanup: {
    enabled: boolean;
    retentionDays: number;
    batchSize: number;
    cronExpression: string;
  };
  features: {
    enableRegistration: boolean;
    enableUsageTracking: boolean;
    enableCostTracking: boolean;
    enableModelMapping: boolean;
  };
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface ApiError {
  error: string;
  message: string;
  statusCode: number;
}

export interface LoginRequest {
  token: string;
}

export interface LoginResponse {
  ok: boolean;
  redirectTo?: string;
  loginType?: string;
  user?: {
    id: number;
    name: string;
    description?: string;
    role: string;
  };
  error?: string;
  errorCode?: string;
}

export interface TestConnectionResult {
  success: boolean;
  latency: number;
  message: string;
}

// ============================================================
// Provider Groups
// ============================================================

export interface ProviderGroup {
  id: string;
  name: string;
  description: string;
  costMultiplier: number;
  providerCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateProviderGroupRequest {
  name: string;
  description?: string;
  costMultiplier?: number;
}

export interface UpdateProviderGroupRequest {
  name?: string;
  description?: string;
  costMultiplier?: number;
}

// ============================================================
// Endpoints
// ============================================================

export interface ProviderEndpoint {
  id: string;
  providerId: string;
  providerName?: string;
  vendor: string;
  type: string;
  url: string;
  enabled: boolean;
  probeStatus: "ok" | "fail" | "unknown";
  latency: number;
  lastProbeAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ProbeLog {
  id: string;
  endpointId: string;
  status: "ok" | "fail";
  latency: number;
  errorMessage?: string;
  createdAt: string;
}

// ============================================================
// Statistics
// ============================================================

export interface StatisticsData {
  requestsOverTime: { time: string; count: number }[];
  costOverTime: { time: string; cost: number }[];
  topModels: { model: string; count: number }[];
  topUsers: { userName: string; count: number }[];
}

// ============================================================
// Leaderboard
// ============================================================

export interface LeaderboardEntry {
  rank: number;
  userId: string;
  userName: string;
  totalRequests: number;
  totalCost: number;
  avgCost: number;
}

// ============================================================
// Prices
// ============================================================

export interface ModelPrice {
  id: string;
  model: string;
  inputPrice: number;
  outputPrice: number;
  source: string;
  updatedAt: string;
}

export interface UpdateModelPriceRequest {
  inputPrice: number;
  outputPrice: number;
}

// ============================================================
// Error Rules
// ============================================================

export interface ErrorRule {
  id: string;
  pattern: string;
  matchType: "contains" | "regex" | "exact";
  category: string;
  enabled: boolean;
  priority: number;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateErrorRuleRequest {
  pattern: string;
  matchType: "contains" | "regex" | "exact";
  category: string;
  priority?: number;
  description?: string;
}

export interface UpdateErrorRuleRequest {
  pattern?: string;
  matchType?: "contains" | "regex" | "exact";
  category?: string;
  enabled?: boolean;
  priority?: number;
  description?: string;
}

// ============================================================
// Request Filters
// ============================================================

export interface RequestFilter {
  id: string;
  name: string;
  scope: string;
  action: "block" | "allow" | "rewrite";
  priority: number;
  enabled: boolean;
  config: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateRequestFilterRequest {
  name: string;
  scope: string;
  action: "block" | "allow" | "rewrite";
  priority?: number;
  config?: string;
}

export interface UpdateRequestFilterRequest {
  name?: string;
  scope?: string;
  action?: "block" | "allow" | "rewrite";
  enabled?: boolean;
  priority?: number;
  config?: string;
}

// ============================================================
// Sensitive Words
// ============================================================

export interface SensitiveWord {
  id: string;
  word: string;
  matchType: "contains" | "regex" | "exact";
  description: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSensitiveWordRequest {
  word: string;
  matchType: "contains" | "regex" | "exact";
  description?: string;
}

export interface UpdateSensitiveWordRequest {
  word?: string;
  matchType?: "contains" | "regex" | "exact";
  description?: string;
  enabled?: boolean;
}

// ============================================================
// Notifications
// ============================================================

export interface NotificationSettings {
  circuitBreaker: { enabled: boolean; threshold: number };
  leaderboard: { enabled: boolean; cron: string };
  costAlert: { enabled: boolean; dailyThreshold: number; monthlyThreshold: number };
  cacheHitRate: { enabled: boolean; minRate: number };
}

export interface WebhookTarget {
  id: string;
  name: string;
  url: string;
  type: "generic" | "slack" | "discord" | "feishu" | "dingtalk";
  enabled: boolean;
  createdAt: string;
}

export interface NotificationBinding {
  id: string;
  notificationType: string;
  webhookTargetId: string;
  webhookTargetName?: string;
  enabled: boolean;
}

export interface CreateWebhookTargetRequest {
  name: string;
  url: string;
  type: "generic" | "slack" | "discord" | "feishu" | "dingtalk";
}

export interface UpdateWebhookTargetRequest {
  name?: string;
  url?: string;
  type?: "generic" | "slack" | "discord" | "feishu" | "dingtalk";
  enabled?: boolean;
}

// ============================================================
// Audit Logs
// ============================================================

export interface AuditLog {
  id: string;
  action: string;
  userId: string;
  userName: string;
  details: string;
  ip: string;
  createdAt: string;
}

export interface AuditLogFilters {
  page?: number;
  pageSize?: number;
  action?: string;
  startDate?: string;
  endDate?: string;
}

// ============================================================
// My Usage (self-service)
// ============================================================

export interface MyUsageData {
  quotas: {
    daily: { used: number; limit: number };
    monthly: { used: number; limit: number };
    rateLimit: { used: number; limit: number };
  };
  recentUsage: {
    id: string;
    model: string;
    inputTokens: number;
    outputTokens: number;
    cost: number;
    createdAt: string;
  }[];
  availableModels: string[];
}
