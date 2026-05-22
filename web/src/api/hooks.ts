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
      apiClient.post<OverviewData>("/actions/overview/getOverviewData", {}),
    refetchInterval: 60_000,
  });
}

// ============================================================
// Users
// ============================================================

export function useUsers(params: { page: number; pageSize: number; search?: string }) {
  return useQuery<PaginatedResponse<User>>({
    queryKey: ["users", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<User>>(
        `/actions/users?page=${params.page}&pageSize=${params.pageSize}${params.search ? `&search=${encodeURIComponent(params.search)}` : ""}`,
      ),
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateUserRequest) =>
      apiClient.post<User>("/actions/users", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      queryClient.invalidateQueries({ queryKey: ["overview"] });
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
      apiClient.put<User>(`/actions/users/${id}`, { isEnabled: enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

// ============================================================
// API Keys
// ============================================================

export function useKeys(params: { page: number; pageSize: number; search?: string }) {
  return useQuery<PaginatedResponse<ApiKey>>({
    queryKey: ["keys", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<ApiKey>>(
        `/actions/keys?page=${params.page}&pageSize=${params.pageSize}${params.search ? `&search=${encodeURIComponent(params.search)}` : ""}`,
      ),
  });
}

export function useCreateKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateKeyRequest) =>
      apiClient.post<ApiKey>("/actions/keys", data),
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
      apiClient.put<ApiKey>(`/actions/keys/${id}`, { isEnabled: enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
    },
  });
}

// ============================================================
// Providers
// ============================================================

export function useProviders(params: { page: number; pageSize: number; search?: string }) {
  return useQuery<PaginatedResponse<Provider>>({
    queryKey: ["providers", params.page, params.pageSize, params.search],
    queryFn: () =>
      apiClient.get<PaginatedResponse<Provider>>(
        `/actions/providers?page=${params.page}&pageSize=${params.pageSize}${params.search ? `&search=${encodeURIComponent(params.search)}` : ""}`,
      ),
  });
}

export function useCreateProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateProviderRequest) =>
      apiClient.post<Provider>("/actions/providers", data),
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
      apiClient.put<Provider>(`/actions/providers/${id}`, { isEnabled: enabled }),
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
  return useQuery<PaginatedResponse<UsageLog>>({
    queryKey: ["usage-logs", filters],
    queryFn: () =>
      apiClient.post<PaginatedResponse<UsageLog>>("/actions/usage-logs/getUsageLogs", filters),
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
      apiClient.post<PaginatedResponse<ProviderGroup>>("/actions/providers/getProviderGroups", params),
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
      apiClient.post<ProviderGroup>("/actions/providers/updateProviderGroup", { id, ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-groups"] });
    },
  });
}

export function useDeleteProviderGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post("/actions/providers/deleteProviderGroup", { id }),
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
      apiClient.post<PaginatedResponse<ProviderEndpoint>>("/actions/providers/getProviderEndpoints", params),
  });
}

export function useProbeEndpoint() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean; latency: number }>("/actions/providers/probeProviderEndpoint", { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["endpoints"] });
    },
  });
}

export function useEndpointProbeLogs(endpointId: string | null) {
  return useQuery<ProbeLog[]>({
    queryKey: ["probe-logs", endpointId],
    queryFn: () =>
      apiClient.post<ProbeLog[]>("/actions/providers/getProviderEndpointProbeLogs", { endpointId }),
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
      apiClient.post<StatisticsData>("/actions/statistics/getUserStatistics", { range }),
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
      apiClient.get<LeaderboardEntry[]>(`/leaderboard?period=${period}`),
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
        `/prices?page=${params.page}&pageSize=${params.pageSize}${searchParam}`,
      ),
  });
}

export function useUpdatePrice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateModelPriceRequest }) =>
      apiClient.put<ModelPrice>(`/prices/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["prices"] });
    },
  });
}

export function useSyncPrices() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient.post<{ synced: number }>("/actions/prices/syncLiteLLMPrices", {}),
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
      apiClient.post<PaginatedResponse<ErrorRule>>("/actions/error-rules/list", params),
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
      apiClient.post<ErrorRule>("/actions/error-rules/update", { id, ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["error-rules"] });
    },
  });
}

export function useDeleteErrorRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post("/actions/error-rules/delete", { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["error-rules"] });
    },
  });
}

export function useToggleErrorRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.post<ErrorRule>("/actions/error-rules/update", { id, isEnabled: enabled }),
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
      apiClient.post<PaginatedResponse<RequestFilter>>("/actions/request-filters/list", params),
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
      apiClient.post<RequestFilter>("/actions/request-filters/update", { id, ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["request-filters"] });
    },
  });
}

export function useDeleteRequestFilter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post("/actions/request-filters/delete", { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["request-filters"] });
    },
  });
}

export function useToggleRequestFilter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.post<RequestFilter>("/actions/request-filters/update", { id, isEnabled: enabled }),
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
      apiClient.post<PaginatedResponse<SensitiveWord>>("/actions/sensitive-words/list", params),
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
      apiClient.post<SensitiveWord>("/actions/sensitive-words/update", { id, ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useDeleteSensitiveWord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post("/actions/sensitive-words/delete", { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useToggleSensitiveWord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiClient.post<SensitiveWord>("/actions/sensitive-words/update", { id, isEnabled: enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sensitive-words"] });
    },
  });
}

export function useRefreshSensitiveWordCache() {
  return useMutation({
    mutationFn: () =>
      apiClient.post("/actions/sensitive-words/refreshCache", {}),
  });
}

// ============================================================
// Notifications
// ============================================================

export function useNotificationSettings() {
  return useQuery<NotificationSettings>({
    queryKey: ["notification-settings"],
    queryFn: () =>
      apiClient.post<NotificationSettings>("/actions/notifications/getNotificationSettings", {}),
  });
}

export function useUpdateNotificationSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: NotificationSettings) =>
      apiClient.post<NotificationSettings>("/actions/notifications/updateNotificationSettings", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notification-settings"] });
    },
  });
}

export function useWebhookTargets() {
  return useQuery<WebhookTarget[]>({
    queryKey: ["webhook-targets"],
    queryFn: () =>
      apiClient.post<WebhookTarget[]>("/actions/webhook-targets/list", {}),
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
      apiClient.post<WebhookTarget>("/actions/webhook-targets/update", { id, ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhook-targets"] });
    },
  });
}

export function useDeleteWebhookTarget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post("/actions/webhook-targets/delete", { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhook-targets"] });
    },
  });
}

export function useTestWebhook() {
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean; message: string }>("/actions/webhook-targets/testWebhook", { id }),
  });
}

export function useNotificationBindings() {
  return useQuery<NotificationBinding[]>({
    queryKey: ["notification-bindings"],
    queryFn: () =>
      apiClient.post<NotificationBinding[]>("/actions/notification-bindings/list", {}),
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
      apiClient.post("/actions/notification-bindings/delete", { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notification-bindings"] });
    },
  });
}

// ============================================================
// Audit Logs
// ============================================================

export function useAuditLogs(filters: AuditLogFilters) {
  return useQuery<PaginatedResponse<AuditLog>>({
    queryKey: ["audit-logs", filters],
    queryFn: () =>
      apiClient.post<PaginatedResponse<AuditLog>>("/actions/audit-logs/getAuditLogsBatch", filters),
  });
}

// ============================================================
// My Usage (Self-service)
// ============================================================

export function useMyUsage() {
  return useQuery<MyUsageData>({
    queryKey: ["my-usage"],
    queryFn: () =>
      apiClient.post<MyUsageData>("/actions/my-usage/getMyUsageMetadata", {}),
    refetchInterval: 60_000,
  });
}
