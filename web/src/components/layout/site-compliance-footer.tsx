import { ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import { useAppearanceStore } from "@/stores/use-appearance-store";

const MIIT_FILING_URL = "https://beian.miit.gov.cn/";

export function SiteComplianceFooter({ className, variant = "default" }: { className?: string; variant?: "default" | "auth" | "workspace" }) {
    const { t } = useTranslation();
    const appearance = useAppearanceStore((state) => state.appearance);

    return (
        <footer className={cn("site-compliance-footer", variant === "auth" && "is-auth", variant === "workspace" && "is-workspace", className)} aria-label={t("account.layout.footer.label")}>
            <span>{appearance.footerCopyright}</span>
            {appearance.icpFilingEnabled && appearance.icpFilingNumber ? (
                <a href={MIIT_FILING_URL} target="_blank" rel="noopener noreferrer" aria-label={t("account.layout.footer.icpLink", { number: appearance.icpFilingNumber })}>
                    <span>{appearance.icpFilingNumber}</span>
                    <ExternalLink aria-hidden="true" />
                </a>
            ) : null}
        </footer>
    );
}
