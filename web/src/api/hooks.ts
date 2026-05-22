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
