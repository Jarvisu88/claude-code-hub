import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/Button";

function NotFound() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-[var(--color-bg-secondary)] p-4">
      <div className="text-center">
        <h1 className="text-6xl font-bold text-[var(--color-text-tertiary)]">
          404
        </h1>
        <h2 className="mt-4 text-2xl font-semibold text-[var(--color-text)]">
          {t("notFound.title")}
        </h2>
        <p className="mt-2 text-[var(--color-text-secondary)]">
          {t("notFound.description")}
        </p>
        <div className="mt-6">
          <Link to="/dashboard">
            <Button>{t("notFound.backHome")}</Button>
          </Link>
        </div>
      </div>
    </div>
  );
}

export { NotFound };
