import { NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import {
  LayoutDashboard,
  Users,
  KeyRound,
  Server,
  Layers,
  Globe,
  BarChart3,
  LineChart,
  Trophy,
  DollarSign,
  AlertTriangle,
  Filter,
  ShieldAlert,
  Bell,
  FileText,
  User,
  Settings,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  onMobileClose?: () => void;
}

const navSections = [
  {
    items: [
      { path: "/dashboard", icon: LayoutDashboard, labelKey: "nav.dashboard" },
    ],
  },
  {
    titleKey: "nav.sectionUsers",
    items: [
      { path: "/users", icon: Users, labelKey: "nav.users" },
      { path: "/keys", icon: KeyRound, labelKey: "nav.keys" },
    ],
  },
  {
    titleKey: "nav.sectionProviders",
    items: [
      { path: "/providers", icon: Server, labelKey: "nav.providers" },
      { path: "/provider-groups", icon: Layers, labelKey: "nav.providerGroups" },
      { path: "/endpoints", icon: Globe, labelKey: "nav.endpoints" },
    ],
  },
  {
    titleKey: "nav.sectionAnalytics",
    items: [
      { path: "/usage", icon: BarChart3, labelKey: "nav.usage" },
      { path: "/statistics", icon: LineChart, labelKey: "nav.statistics" },
      { path: "/leaderboard", icon: Trophy, labelKey: "nav.leaderboard" },
      { path: "/prices", icon: DollarSign, labelKey: "nav.prices" },
    ],
  },
  {
    titleKey: "nav.sectionSecurity",
    items: [
      { path: "/error-rules", icon: AlertTriangle, labelKey: "nav.errorRules" },
      { path: "/request-filters", icon: Filter, labelKey: "nav.requestFilters" },
      { path: "/sensitive-words", icon: ShieldAlert, labelKey: "nav.sensitiveWords" },
    ],
  },
  {
    titleKey: "nav.sectionSystem",
    items: [
      { path: "/notifications", icon: Bell, labelKey: "nav.notifications" },
      { path: "/audit-logs", icon: FileText, labelKey: "nav.auditLogs" },
      { path: "/my-usage", icon: User, labelKey: "nav.myUsage" },
      { path: "/settings", icon: Settings, labelKey: "nav.settings" },
    ],
  },
];

function Sidebar({ collapsed, onToggle, onMobileClose }: SidebarProps) {
  const { t } = useTranslation();

  return (
    <aside
      className={cn(
        "flex h-full flex-col border-r border-[var(--color-border)] bg-[var(--color-bg-secondary)] transition-all duration-300",
        collapsed ? "w-[var(--sidebar-collapsed-width)]" : "w-[var(--sidebar-width)]",
      )}
    >
      {/* Logo / Brand */}
      <div className="flex h-[var(--header-height)] items-center border-b border-[var(--color-border)] px-4">
        {!collapsed && (
          <span className="text-lg font-bold text-[var(--color-text)]">
            CCH
          </span>
        )}
        {collapsed && (
          <span className="text-lg font-bold text-[var(--color-text)] mx-auto">
            C
          </span>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto px-2 py-4">
        {navSections.map((section, sectionIndex) => (
          <div key={sectionIndex} className={sectionIndex > 0 ? "mt-4" : ""}>
            {section.titleKey && !collapsed && (
              <div className="mb-1 px-3 text-xs font-semibold uppercase tracking-wider text-[var(--color-text-tertiary)]">
                {t(section.titleKey)}
              </div>
            )}
            {collapsed && sectionIndex > 0 && (
              <div className="mx-2 mb-2 border-t border-[var(--color-border)]" />
            )}
            <div className="space-y-0.5">
              {section.items.map((item) => (
                <NavLink
                  key={item.path}
                  to={item.path}
                  onClick={onMobileClose}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      isActive
                        ? "bg-[var(--color-primary-light)] text-[var(--color-primary)]"
                        : "text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text)]",
                      collapsed && "justify-center px-2",
                    )
                  }
                >
                  <item.icon className="h-5 w-5 shrink-0" />
                  {!collapsed && <span>{t(item.labelKey)}</span>}
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>

      {/* Collapse toggle */}
      <div className="border-t border-[var(--color-border)] p-2">
        <button
          onClick={onToggle}
          className="flex w-full items-center justify-center rounded-md p-2 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text)] transition-colors"
        >
          {collapsed ? (
            <ChevronRight className="h-4 w-4" />
          ) : (
            <ChevronLeft className="h-4 w-4" />
          )}
        </button>
      </div>
    </aside>
  );
}

export { Sidebar };
