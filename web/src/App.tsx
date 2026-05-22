import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Layout } from "@/components/layout/Layout";
import { Login } from "@/pages/Login";
import { Dashboard } from "@/pages/Dashboard";
import { UsersPage } from "@/pages/Users";
import { KeysPage } from "@/pages/Keys";
import { ProvidersPage } from "@/pages/Providers";
import { ProviderGroupsPage } from "@/pages/ProviderGroups";
import { EndpointsPage } from "@/pages/Endpoints";
import { UsagePage } from "@/pages/Usage";
import { StatisticsPage } from "@/pages/Statistics";
import { LeaderboardPage } from "@/pages/Leaderboard";
import { PricesPage } from "@/pages/Prices";
import { ErrorRulesPage } from "@/pages/ErrorRules";
import { RequestFiltersPage } from "@/pages/RequestFilters";
import { SensitiveWordsPage } from "@/pages/SensitiveWords";
import { NotificationsPage } from "@/pages/Notifications";
import { AuditLogsPage } from "@/pages/AuditLogs";
import { MyUsagePage } from "@/pages/MyUsage";
import { SettingsPage } from "@/pages/Settings";
import { BigScreen } from "@/pages/BigScreen";
import { NotFound } from "@/pages/NotFound";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 30_000,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<Login />} />
          <Route path="/big-screen" element={<BigScreen />} />

          {/* Protected routes */}
          <Route element={<Layout />}>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/users" element={<UsersPage />} />
            <Route path="/keys" element={<KeysPage />} />
            <Route path="/providers" element={<ProvidersPage />} />
            <Route path="/provider-groups" element={<ProviderGroupsPage />} />
            <Route path="/endpoints" element={<EndpointsPage />} />
            <Route path="/usage" element={<UsagePage />} />
            <Route path="/statistics" element={<StatisticsPage />} />
            <Route path="/leaderboard" element={<LeaderboardPage />} />
            <Route path="/prices" element={<PricesPage />} />
            <Route path="/error-rules" element={<ErrorRulesPage />} />
            <Route path="/request-filters" element={<RequestFiltersPage />} />
            <Route path="/sensitive-words" element={<SensitiveWordsPage />} />
            <Route path="/notifications" element={<NotificationsPage />} />
            <Route path="/audit-logs" element={<AuditLogsPage />} />
            <Route path="/my-usage" element={<MyUsagePage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>

          {/* 404 */}
          <Route path="*" element={<NotFound />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
