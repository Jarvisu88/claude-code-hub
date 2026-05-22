import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Navigate } from "react-router-dom";
import { useAuthStore } from "@/stores/auth";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Spinner } from "@/components/ui/Spinner";

function Login() {
  const { t } = useTranslation();
  const { isAuthenticated, isLoading, error, login, clearError } =
    useAuthStore();
  const [token, setToken] = useState("");

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (token.trim()) {
      login(token.trim());
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-bg-secondary)] p-4">
      <div className="w-full max-w-sm">
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] p-8 shadow-sm">
          {/* Header */}
          <div className="mb-8 text-center">
            <h1 className="text-2xl font-bold text-[var(--color-text)]">
              Claude Code Hub
            </h1>
            <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
              {t("auth.login")}
            </p>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            <Input
              type="password"
              label={t("auth.password")}
              placeholder={t("auth.password")}
              value={token}
              onChange={(e) => {
                setToken(e.target.value);
                if (error) clearError();
              }}
              error={error ? t(error) : undefined}
              autoFocus
            />

            <Button
              type="submit"
              className="w-full"
              size="lg"
              disabled={isLoading || !token.trim()}
            >
              {isLoading ? (
                <span className="flex items-center gap-2">
                  <Spinner size="sm" />
                  {t("common.loading")}
                </span>
              ) : (
                t("auth.submit")
              )}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}

export { Login };
