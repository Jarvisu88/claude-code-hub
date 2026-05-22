import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient } from "./client";
import type {
  OverviewData,
  User,
  CreateUserRequest,
  UpdateUserRequest,
  ApiKey,
  CreateKeyRequest,
  UpdateKeyRequest,
  Provider,
  CreateProviderRequest,
  UpdateProviderRequest,
  UsageLog,
  UsageFilters,
  SystemSettings,
  PaginatedResponse,
  TestConnectionResult,
  ProviderGroup,
  CreateProviderGroupRequest,
  UpdateProviderGroupRequest,
  ProviderEndpoint,
  ProbeLog,
  StatisticsData,
  LeaderboardEntry,
  ModelPrice,
  UpdateModelPriceRequest,
  ErrorRule,
  CreateErrorRuleRequest,
  UpdateErrorRuleRequest,
  RequestFilter,
  CreateRequestFilterRequest,
  UpdateRequestFilterRequest,
  SensitiveWord,
  CreateSensitiveWordRequest,
  UpdateSensitiveWordRequest,
  NotificationSettings,
  WebhookTarget,
  CreateWebhookTargetRequest,
  UpdateWebhookTargetRequest,
  NotificationBinding,
  AuditLog,
  AuditLogFilters,
  MyUsageData,
} from "./types";

// ============================================================
// Dashboard / Overview
// ============================================================

export function useOverview() {
  return useQuery<OverviewData>({
    queryKey: ["overview"],
    queryFn: () =>
      apiClient.get<OverviewData>("/actions/overview/getOverviewData"),
    refetchInterval: 60_000,
  });
}

// ============================================================
// Users
// ============================================================

export function useUsers(params: { page: number; pageSize: number; search?: string }) {
  const searchParam = params.search ? `&search=${encodeURIComponent(params.search)}` : "";
  return useQuery<PaginatedResponse<User>>({
    queryKey: ["users", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<User>>(
        `/actions/users/list?page=${params.page}&pageSize=${params.pageSize}${searchParam}`,
      ),
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateUserRequest) =>
      apiClient.post<User>("/actions/users/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUserRequest }) =>
      apiClient.put<User>(`/actions/users/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      queryClient.invalidateQueries({ queryKey: ["overview"] });
    },
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/users/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      queryClient.invalidateQueries({ queryKey: ["overview"] });
    },
  });
}

export function useToggleUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.patch<User>(`/actions/users/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

// ============================================================
// API Keys
// ============================================================

export function useKeys(params: { page: number; pageSize: number; search?: string }) {
  const searchParam = params.search ? `&search=${encodeURIComponent(params.search)}` : "";
  return useQuery<PaginatedResponse<ApiKey>>({
    queryKey: ["keys", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<ApiKey>>(
        `/actions/keys/list?page=${params.page}&pageSize=${params.pageSize}${searchParam}`,
      ),
  });
}

export function useCreateKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateKeyRequest) =>
      apiClient.post<ApiKey>("/actions/keys/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
    },
  });
}

export function useUpdateKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateKeyRequest }) =>
      apiClient.put<ApiKey>(`/actions/keys/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
    },
  });
}

export function useDeleteKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/keys/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
    },
  });
}

export function useToggleKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.patch<ApiKey>(`/actions/keys/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
    },
  });
}

// ============================================================
// Providers
// ============================================================

export function useProviders(params: { page: number; pageSize: number; search?: string }) {
  const searchParam = params.search ? `&search=${encodeURIComponent(params.search)}` : "";
  return useQuery<PaginatedResponse<Provider>>({
    queryKey: ["providers", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<Provider>>(
        `/actions/providers/list?page=${params.page}&pageSize=${params.pageSize}${searchParam}`,
      ),
  });
}

export function useCreateProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateProviderRequest) =>
      apiClient.post<Provider>("/actions/providers/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["providers"] });
      queryClient.invalidateQueries({ queryKey: ["overview"] });
    },
  });
}

export function useUpdateProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateProviderRequest }) =>
      apiClient.put<Provider>(`/actions/providers/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}

export function useDeleteProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/providers/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["providers"] });
      queryClient.invalidateQueries({ queryKey: ["overview"] });
    },
  });
}

export function useToggleProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.patch<Provider>(`/actions/providers/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}

export function useTestProvider() {
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<TestConnectionResult>(`/actions/providers/${id}/test`),
  });
}

// ============================================================
// Usage Logs
// ============================================================

export function useUsageLogs(filters: UsageFilters) {
  const params = new URLSearchParams();
  if (filters.page) params.set("page", String(filters.page));
  if (filters.pageSize) params.set("pageSize", String(filters.pageSize));
  if (filters.startDate) params.set("startDate", filters.startDate);
  if (filters.endDate) params.set("endDate", filters.endDate);
  if (filters.userId) params.set("userId", filters.userId);
  if (filters.model) params.set("model", filters.model);
  if (filters.status !== undefined) params.set("status", String(filters.status));

  const queryString = params.toString();
  return useQuery<PaginatedResponse<UsageLog>>({
    queryKey: ["usage-logs", filters],
    queryFn: () =>
      apiClient.get<PaginatedResponse<UsageLog>>(
        `/actions/usage-logs/getUsageLogs${queryString ? `?${queryString}` : ""}`,
      ),
  });
}

// ============================================================
// System Settings
// ============================================================

export function useSettings() {
  return useQuery<SystemSettings>({
    queryKey: ["settings"],
    queryFn: () => apiClient.get<SystemSettings>("/system-settings"),
    staleTime: 60_000,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: SystemSettings) =>
      apiClient.put<SystemSettings>("/system-settings", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
}

// ============================================================
// Provider Groups
// ============================================================

export function useProviderGroups(params: { page: number; pageSize: number }) {
  return useQuery<PaginatedResponse<ProviderGroup>>({
    queryKey: ["provider-groups", params.page, params.pageSize],
    queryFn: () =>
      apiClient.get<PaginatedResponse<ProviderGroup>>(
        `/actions/providers/getProviderGroups?page=${params.page}&pageSize=${params.pageSize}`,
      ),
  });
}

export function useCreateProviderGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateProviderGroupRequest) =>
      apiClient.post<ProviderGroup>("/actions/providers/createProviderGroup", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-groups"] });
    },
  });
}

export function useUpdateProviderGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateProviderGroupRequest }) =>
      apiClient.put<ProviderGroup>(`/actions/providers/updateProviderGroup/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-groups"] });
    },
  });
}

export function useDeleteProviderGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/providers/deleteProviderGroup/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-groups"] });
    },
  });
}

// ============================================================
// Endpoints
// ============================================================

export function useEndpoints(params: { page: number; pageSize: number }) {
  return useQuery<PaginatedResponse<ProviderEndpoint>>({
    queryKey: ["endpoints", params.page, params.pageSize],
    queryFn: () =>
      apiClient.get<PaginatedResponse<ProviderEndpoint>>(
        `/actions/providers/getProviderEndpoints?page=${params.page}&pageSize=${params.pageSize}`,
      ),
  });
}

export function useProbeEndpoint() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean; latency: number }>(`/actions/providers/probeEndpoint/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["endpoints"] });
    },
  });
}

export function useEndpointProbeLogs(endpointId: string | null) {
  return useQuery<ProbeLog[]>({
    queryKey: ["probe-logs", endpointId],
    queryFn: () =>
      apiClient.get<ProbeLog[]>(`/actions/providers/getProbeLog/${endpointId}`),
    enabled: !!endpointId,
  });
}

// ============================================================
// Statistics
// ============================================================

export function useStatistics(range: string) {
  return useQuery<StatisticsData>({
    queryKey: ["statistics", range],
    queryFn: () =>
      apiClient.get<StatisticsData>(`/actions/statistics/getStatistics?range=${range}`),
    refetchInterval: 120_000,
  });
}

// ============================================================
// Leaderboard
// ============================================================

export function useLeaderboard(period: string) {
  return useQuery<LeaderboardEntry[]>({
    queryKey: ["leaderboard", period],
    queryFn: () =>
      apiClient.get<LeaderboardEntry[]>(`/api/leaderboard?period=${period}`),
  });
}

// ============================================================
// Prices
// ============================================================

export function usePrices(params: { page: number; pageSize: number; search?: string }) {
  const searchParam = params.search ? `&search=${encodeURIComponent(params.search)}` : "";
  return useQuery<PaginatedResponse<ModelPrice>>({
    queryKey: ["prices", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<ModelPrice>>(
        `/api/prices?page=${params.page}&pageSize=${params.pageSize}${searchParam}`,
      ),
  });
}

export function useUpdatePrice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateModelPriceRequest }) =>
      apiClient.put<ModelPrice>(`/api/prices/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["prices"] });
    },
  });
}

export function useSyncPrices() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient.post<{ synced: number }>("/api/prices/sync"),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["prices"] });
    },
  });
}

// ============================================================
// Error Rules
// ============================================================

export function useErrorRules(params: { page: number; pageSize: number }) {
  return useQuery<PaginatedResponse<ErrorRule>>({
    queryKey: ["error-rules", params.page, params.pageSize],
    queryFn: () =>
      apiClient.get<PaginatedResponse<ErrorRule>>(
        `/actions/error-rules/list?page=${params.page}&pageSize=${params.pageSize}`,
      ),
  });
}

export function useCreateErrorRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateErrorRuleRequest) =>
      apiClient.post<ErrorRule>("/actions/error-rules/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["error-rules"] });
    },
  });
}

export function useUpdateErrorRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateErrorRuleRequest }) =>
      apiClient.put<ErrorRule>(`/actions/error-rules/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["error-rules"] });
    },
  });
}

export function useDeleteErrorRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/error-rules/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["error-rules"] });
    },
  });
}

export function useToggleErrorRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.patch<ErrorRule>(`/actions/error-rules/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["error-rules"] });
    },
  });
}

// ============================================================
// Request Filters
// ============================================================

export function useRequestFilters(params: { page: number; pageSize: number }) {
  return useQuery<PaginatedResponse<RequestFilter>>({
    queryKey: ["request-filters", params.page, params.pageSize],
    queryFn: () =>
      apiClient.get<PaginatedResponse<RequestFilter>>(
        `/actions/request-filters/list?page=${params.page}&pageSize=${params.pageSize}`,
      ),
  });
}

export function useCreateRequestFilter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateRequestFilterRequest) =>
      apiClient.post<RequestFilter>("/actions/request-filters/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["request-filters"] });
    },
  });
}

export function useUpdateRequestFilter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateRequestFilterRequest }) =>
      apiClient.put<RequestFilter>(`/actions/request-filters/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["request-filters"] });
    },
  });
}

export function useDeleteRequestFilter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/request-filters/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["request-filters"] });
    },
  });
}

export function useToggleRequestFilter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.patch<RequestFilter>(`/actions/request-filters/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["request-filters"] });
    },
  });
}

// ============================================================
// Sensitive Words
// ============================================================

export function useSensitiveWords(params: { page: number; pageSize: number }) {
  return useQuery<PaginatedResponse<SensitiveWord>>({
    queryKey: ["sensitive-words", params.page, params.pageSize],
    queryFn: () =>
      apiClient.get<PaginatedResponse<SensitiveWord>>(
        `/actions/sensitive-words/list?page=${params.page}&pageSize=${params.pageSize}`,
      ),
  });
}

export function useCreateSensitiveWord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateSensitiveWordRequest) =>
      apiClient.post<SensitiveWord>("/actions/sensitive-words/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useUpdateSensitiveWord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateSensitiveWordRequest }) =>
      apiClient.put<SensitiveWord>(`/actions/sensitive-words/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useDeleteSensitiveWord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/sensitive-words/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useToggleSensitiveWord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.patch<SensitiveWord>(`/actions/sensitive-words/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useRefreshSensitiveWordCache() {
  return useMutation({
    mutationFn: () =>
      apiClient.post("/actions/sensitive-words/refreshCache"),
  });
}

// ============================================================
// Notifications
// ============================================================

export function useNotificationSettings() {
  return useQuery<NotificationSettings>({
    queryKey: ["notification-settings"],
    queryFn: () =>
      apiClient.get<NotificationSettings>("/actions/notifications/getSettings"),
  });
}

export function useUpdateNotificationSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: NotificationSettings) =>
      apiClient.put<NotificationSettings>("/actions/notifications/updateSettings", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notification-settings"] });
    },
  });
}

export function useWebhookTargets() {
  return useQuery<WebhookTarget[]>({
    queryKey: ["webhook-targets"],
    queryFn: () =>
      apiClient.get<WebhookTarget[]>("/actions/webhook-targets/list"),
  });
}

export function useCreateWebhookTarget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWebhookTargetRequest) =>
      apiClient.post<WebhookTarget>("/actions/webhook-targets/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhook-targets"] });
    },
  });
}

export function useUpdateWebhookTarget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateWebhookTargetRequest }) =>
      apiClient.put<WebhookTarget>(`/actions/webhook-targets/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhook-targets"] });
    },
  });
}

export function useDeleteWebhookTarget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/webhook-targets/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhook-targets"] });
    },
  });
}

export function useTestWebhook() {
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean; message: string }>(`/actions/webhook-targets/${id}/test`),
  });
}

export function useNotificationBindings() {
  return useQuery<NotificationBinding[]>({
    queryKey: ["notification-bindings"],
    queryFn: () =>
      apiClient.get<NotificationBinding[]>("/actions/notification-bindings/list"),
  });
}

export function useCreateNotificationBinding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: { notificationType: string; webhookTargetId: string }) =>
      apiClient.post<NotificationBinding>("/actions/notification-bindings/create", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notification-bindings"] });
    },
  });
}

export function useDeleteNotificationBinding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(`/actions/notification-bindings/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notification-bindings"] });
    },
  });
}

// ============================================================
// Audit Logs
// ============================================================

export function useAuditLogs(filters: AuditLogFilters) {
  const params = new URLSearchParams();
  if (filters.page) params.set("page", String(filters.page));
  if (filters.pageSize) params.set("pageSize", String(filters.pageSize));
  if (filters.action) params.set("action", filters.action);
  if (filters.startDate) params.set("startDate", filters.startDate);
  if (filters.endDate) params.set("endDate", filters.endDate);

  const queryString = params.toString();
  return useQuery<PaginatedResponse<AuditLog>>({
    queryKey: ["audit-logs", filters],
    queryFn: () =>
      apiClient.get<PaginatedResponse<AuditLog>>(
        `/actions/audit-logs/list${queryString ? `?${queryString}` : ""}`,
      ),
  });
}

// ============================================================
// My Usage (Self-service)
// ============================================================

export function useMyUsage() {
  return useQuery<MyUsageData>({
    queryKey: ["my-usage"],
    queryFn: () =>
      apiClient.get<MyUsageData>("/actions/my-usage/getData"),
    refetchInterval: 60_000,
  });
}
