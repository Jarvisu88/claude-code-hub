import { redirect } from "@/i18n/routing";

export const dynamic = "force-dynamic";

interface SettingsAccountingRedirectProps {
  params: Promise<{
    locale: string;
  }>;
}

export default async function SettingsAccountingRedirect({
  params,
}: SettingsAccountingRedirectProps) {
  const { locale } = await params;
  return redirect({ href: "/dashboard/accounting", locale });
}
