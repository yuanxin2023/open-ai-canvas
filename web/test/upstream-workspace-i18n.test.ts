import { afterEach, expect, test } from "bun:test";
import { i18n, translate } from "@/i18n";
import { SUPPORTED_LOCALES } from "@/i18n/locales";
import { LOCAL_UI_ROWS } from "@/i18n/messages/local";

afterEach(async () => {
    await i18n.changeLanguage("zh-CN");
});

test("retained navigation and custom UI text resolve in every supported language", async () => {
    for (const { code } of SUPPORTED_LOCALES) {
        await i18n.changeLanguage(code);
        for (const key of Object.keys(LOCAL_UI_ROWS)) {
            expect(i18n.exists(`local.ui.${key}`, { lng: code, fallbackLng: false })).toBe(true);
            expect(translate(`local.ui.${key}`)).not.toBe(`local.ui.${key}`);
        }
        expect(translate("local.nav.prompts")).not.toBe("local.nav.prompts");
        expect(translate("local.auth.referralCode")).not.toBe("local.auth.referralCode");
    }
});
