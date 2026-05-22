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
  token: string;
  expiresAt: string;
}

export interface TestConnectionResult {
  success: boolean;
  latency: number;
  message: string;
}
