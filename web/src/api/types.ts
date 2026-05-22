export interface User {
  id: string;
  name: string;
  email: string;
  role: "admin" | "user";
  enabled: boolean;
  rateLimit: number;
  createdAt: string;
  updatedAt: string;
}

export interface ApiKey {
  id: string;
  name: string;
  key: string;
  userId: string;
  enabled: boolean;
  expiresAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Provider {
  id: string;
  name: string;
  type: string;
  endpoint: string;
  enabled: boolean;
  weight: number;
  priority: number;
  models: string[];
  createdAt: string;
  updatedAt: string;
}

export interface UsageLog {
  id: string;
  userId: string;
  keyId: string;
  providerId: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  cost: number;
  duration: number;
  status: number;
  createdAt: string;
}

export interface DashboardStats {
  totalRequests: number;
  totalCost: number;
  activeUsers: number;
  activeProviders: number;
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
