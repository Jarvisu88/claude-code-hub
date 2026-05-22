import { NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import {
  LayoutDashboard,
  Users,
  KeyRound,
  Server,
  BarChart3,
  Settings,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  onMobileClose?: () => void;
}

const navItems = [
  { path: "/dashboard", icon: LayoutDashboard, labelKey: "nav.dashboard" },
  { path: "/users", icon: Users, labelKey: "nav.users" },
  { path: "/keys", icon: KeyRound, labelKey: "nav.keys" },
  { path: "/providers", icon: Server, labelKey: "nav.providers" },
  { path: "/usage", icon: BarChart3, labelKey: "nav.usage" },
  { path: "/settings", icon: Settings, labelKey: "nav.settings" },
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
      <nav className="flex-1 space-y-1 px-2 py-4">
        {navItems.map((item) => (
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
