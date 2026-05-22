import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/auth";
import { useThemeStore } from "@/stores/theme";
import { cn } from "@/lib/utils";
import { Sun, Moon, LogOut, Menu, Globe } from "lucide-react";
import { useState, useRef, useEffect } from "react";

interface HeaderProps {
  onMenuClick: () => void;
}

const languages = [
  { code: "en", label: "English" },
  { code: "zh-CN", label: "简体中文" },
  { code: "zh-TW", label: "繁體中文" },
  { code: "ja", label: "日本語" },
  { code: "ru", label: "Русский" },
];

function Header({ onMenuClick }: HeaderProps) {
  const { t, i18n } = useTranslation();
  const { logout } = useAuthStore();
  const { theme, toggleTheme } = useThemeStore();
  const [langOpen, setLangOpen] = useState(false);
  const langRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (langRef.current && !langRef.current.contains(e.target as Node)) {
        setLangOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const changeLanguage = (code: string) => {
    i18n.changeLanguage(code);
    setLangOpen(false);
  };

  return (
    <header className="flex h-[var(--header-height)] items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4">
      {/* Left: mobile menu button */}
      <button
        onClick={onMenuClick}
        className="rounded-md p-2 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] lg:hidden"
      >
        <Menu className="h-5 w-5" />
      </button>

      {/* Spacer for desktop */}
      <div className="hidden lg:block" />

      {/* Right: actions */}
      <div className="flex items-center gap-2">
        {/* Language switcher */}
        <div ref={langRef} className="relative">
          <button
            onClick={() => setLangOpen(!langOpen)}
            className="flex items-center gap-1 rounded-md p-2 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text)] transition-colors"
            title={t("common.language")}
          >
            <Globe className="h-5 w-5" />
          </button>
          {langOpen && (
            <div className="absolute right-0 top-full z-50 mt-1 w-40 rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] py-1 shadow-lg">
              {languages.map((lang) => (
                <button
                  key={lang.code}
                  onClick={() => changeLanguage(lang.code)}
                  className={cn(
                    "w-full px-4 py-2 text-left text-sm transition-colors",
                    i18n.language === lang.code
                      ? "bg-[var(--color-primary-light)] text-[var(--color-primary)]"
                      : "text-[var(--color-text)] hover:bg-[var(--color-bg-tertiary)]",
                  )}
                >
                  {lang.label}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Theme toggle */}
        <button
          onClick={toggleTheme}
          className="rounded-md p-2 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text)] transition-colors"
          title={theme === "light" ? t("common.darkMode") : t("common.lightMode")}
        >
          {theme === "light" ? (
            <Moon className="h-5 w-5" />
          ) : (
            <Sun className="h-5 w-5" />
          )}
        </button>

        {/* Logout */}
        <button
          onClick={logout}
          className="flex items-center gap-2 rounded-md px-3 py-2 text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text)] transition-colors"
          title={t("nav.logout")}
        >
          <LogOut className="h-4 w-4" />
          <span className="hidden sm:inline">{t("nav.logout")}</span>
        </button>
      </div>
    </header>
  );
}

export { Header };
