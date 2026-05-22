import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/Card";

function KeysPage() {
  const { t } = useTranslation();

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold text-[var(--color-text)]">
        {t("keys.title")}
      </h1>
      <Card>
        <CardHeader>
          <CardTitle>{t("keys.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex h-64 items-center justify-center text-[var(--color-text-tertiary)]">
            {t("keys.placeholder")}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export { KeysPage };
