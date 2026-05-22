import { client } from "./client";

// Overview
export const getOverview = () => client.post("/actions/overview/getOverviewData", {});

// Users CRUD
export const getUsers = () => client.get("/actions/users");
export const createUser = (data: any) => client.post("/actions/users", data);
export const updateUser = (id: string | number, data: any) =>
  client.put(`/actions/users/${id}`, data);
export const deleteUser = (id: string | number) =>
  client.delete(`/actions/users/${id}`);

// Keys CRUD
export const getKeys = () => client.get("/actions/keys");
export const createKey = (data: any) => client.post("/actions/keys", data);
export const updateKey = (id: string | number, data: any) =>
  client.put(`/actions/keys/${id}`, data);
export const deleteKey = (id: string | number) =>
  client.delete(`/actions/keys/${id}`);

// Providers CRUD
export const getProviders = () => client.get("/actions/providers");
export const createProvider = (data: any) =>
  client.post("/actions/providers", data);
export const updateProvider = (id: string | number, data: any) =>
  client.put(`/actions/providers/${id}`, data);
export const deleteProvider = (id: string | number) =>
  client.delete(`/actions/providers/${id}`);

// Error Rules CRUD
export const getErrorRules = () => client.get("/actions/error-rules");
export const createErrorRule = (data: any) =>
  client.post("/actions/error-rules", data);
export const updateErrorRule = (id: string | number, data: any) =>
  client.put(`/actions/error-rules/${id}`, data);
export const deleteErrorRule = (id: string | number) =>
  client.delete(`/actions/error-rules/${id}`);

// Request Filters CRUD
export const getRequestFilters = () => client.get("/actions/request-filters");
export const createRequestFilter = (data: any) =>
  client.post("/actions/request-filters", data);
export const updateRequestFilter = (id: string | number, data: any) =>
  client.put(`/actions/request-filters/${id}`, data);
export const deleteRequestFilter = (id: string | number) =>
  client.delete(`/actions/request-filters/${id}`);

// Sensitive Words CRUD
export const getSensitiveWords = () => client.get("/actions/sensitive-words");
export const createSensitiveWord = (data: any) =>
  client.post("/actions/sensitive-words", data);
export const updateSensitiveWord = (id: string | number, data: any) =>
  client.put(`/actions/sensitive-words/${id}`, data);
export const deleteSensitiveWord = (id: string | number) =>
  client.delete(`/actions/sensitive-words/${id}`);

// Webhook Targets CRUD
export const getWebhookTargets = () => client.get("/actions/webhook-targets");
export const createWebhookTarget = (data: any) =>
  client.post("/actions/webhook-targets", data);
export const updateWebhookTarget = (id: string | number, data: any) =>
  client.put(`/actions/webhook-targets/${id}`, data);
export const deleteWebhookTarget = (id: string | number) =>
  client.delete(`/actions/webhook-targets/${id}`);

// Usage Logs
export const getUsageLogs = (params: any) =>
  client.post("/actions/usage-logs/getUsageLogs", params);

// Statistics
export const getStatistics = (range: string) =>
  client.post("/actions/statistics/getUserStatistics", { range });

// System Settings
export const getSystemSettings = () => client.get("/system-settings");
export const updateSystemSettings = (data: any) =>
  client.put("/system-settings", data);

// Notifications
export const getNotificationSettings = () => client.get("/actions/notifications");
export const updateNotificationSettings = (data: any) =>
  client.put("/actions/notifications", data);

// Notification Bindings
export const getNotificationBindings = () =>
  client.get("/actions/notification-bindings");

// Prices
export const getPrices = () => client.get("/prices");

// Leaderboard
export const getLeaderboard = (period: string) =>
  client.get(`/leaderboard?period=${period}`);

// Version
export const getVersion = () => client.get("/version");
