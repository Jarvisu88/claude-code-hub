import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/Card";

function UsagePage() {
  const { t } = useTranslation();

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold text-[var(--color-text)]">
        {t("usage.title")}
      </h1>
      <Card>
        <CardHeader>
          <CardTitle>{t("usage.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex h-64 items-center justify-center text-[var(--color-text-tertiary)]">
            {t("usage.placeholder")}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export { UsagePage };
