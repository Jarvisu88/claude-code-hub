import { getTranslations } from "next-intl/server";
import { getAccountingSummary } from "@/actions/accounting";
import { Section } from "@/components/section";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { redirect } from "@/i18n/routing";
import { getSession } from "@/lib/auth";
import { AccountingPanel } from "./_components/accounting-panel";

export const dynamic = "force-dynamic";

interface DashboardAccountingPageProps {
  params: Promise<{
    locale: string;
  }>;
}

export default async function DashboardAccountingPage({ params }: DashboardAccountingPageProps) {
  const { locale } = await params;
  const session = await getSession();

  if (session?.user.role !== "admin") {
    return redirect({ href: "/dashboard", locale });
  }

  const t = await getTranslations({ locale, namespace: "settings" });
  const result = await getAccountingSummary();

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t("accounting.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("accounting.description")}</p>
      </div>
      <Section
        title={t("accounting.section.title")}
        description={t("accounting.section.description")}
        icon="dollar-sign"
        iconColor="text-[#0F766E]"
        variant="default"
      >
        {result.ok ? (
          <AccountingPanel summary={result.data} />
        ) : (
          <Alert variant="destructive">
            <AlertTitle>{t("accounting.loadFailed")}</AlertTitle>
            <AlertDescription>{result.error}</AlertDescription>
          </Alert>
        )}
      </Section>
    </div>
  );
}
