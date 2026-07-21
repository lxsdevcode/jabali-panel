// i18n bootstrap (ADR-0007 superseded — see docs/adr for the multi-language
// decision). English is the SOURCE language: `src/locales/en/common.json` is the
// only catalog edited by hand in this repo. Every other locale is written by
// translators through Weblate and lands here as a PR — never hand-edit them.
//
// Adding a locale:
//   1. add `src/locales/<lng>/common.json` (Weblate creates it)
//   2. import it below + add it to `resources` and `SUPPORTED`
//   3. map its AntD + dayjs locale in `ANTD_LOCALES` / `DAYJS_LOCALES`
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import type { Locale as AntdLocale } from "antd/es/locale";
import enUS from "antd/locale/en_US";
import heIL from "antd/locale/he_IL";
import dayjs from "dayjs";
import "dayjs/locale/he";

import enCommon from "./locales/en/common.json";
import heCommon from "./locales/he/common.json";

export const DEFAULT_LNG = "en";

/** Locales the panel ships. Keep in sync with src/locales/*. */
export const SUPPORTED = ["en", "he"] as const;
export type SupportedLng = (typeof SUPPORTED)[number];

/** Right-to-left scripts. Drives AntD's `direction` + the <html dir> attribute. */
const RTL = new Set<string>(["he", "ar", "fa", "ur"]);

const ANTD_LOCALES: Record<string, AntdLocale> = { en: enUS, he: heIL };
// dayjs ships "en" built in; every other locale must be imported above.
const DAYJS_LOCALES: Record<string, string> = { en: "en", he: "he" };

const STORAGE_KEY = "jabali-lng";

/** Normalise "he-IL" / "en-GB" down to a locale we actually ship. */
function normalise(raw: string | null | undefined): SupportedLng {
  if (!raw) return DEFAULT_LNG;
  const base = raw.toLowerCase().split(/[-_]/)[0];
  return (SUPPORTED as readonly string[]).includes(base)
    ? (base as SupportedLng)
    : DEFAULT_LNG;
}

/**
 * Resolution order: explicit user choice (localStorage) → browser preference →
 * English. A server-side per-user preference can be layered on top later by
 * calling setLanguage() once the profile loads.
 */
function detect(): SupportedLng {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored) return normalise(stored);
  } catch {
    // localStorage can throw in private mode — fall through to the browser.
  }
  return normalise(typeof navigator !== "undefined" ? navigator.language : null);
}

export function isRTL(lng: string = i18n.resolvedLanguage ?? DEFAULT_LNG): boolean {
  return RTL.has(normalise(lng));
}

export function antdLocale(lng: string = i18n.resolvedLanguage ?? DEFAULT_LNG): AntdLocale {
  return ANTD_LOCALES[normalise(lng)] ?? enUS;
}

/** Apply <html lang>/<html dir> + the dayjs locale for the active language. */
function applyDocumentLocale(lng: string): void {
  const l = normalise(lng);
  dayjs.locale(DAYJS_LOCALES[l] ?? "en");
  if (typeof document !== "undefined") {
    document.documentElement.setAttribute("lang", l);
    document.documentElement.setAttribute("dir", isRTL(l) ? "rtl" : "ltr");
  }
}

/** Switch language + remember the choice. */
export async function setLanguage(lng: string): Promise<void> {
  const l = normalise(lng);
  try {
    window.localStorage.setItem(STORAGE_KEY, l);
  } catch {
    // Non-fatal: the language still applies for this session.
  }
  await i18n.changeLanguage(l);
}

void i18n.use(initReactI18next).init({
  lng: detect(),
  fallbackLng: DEFAULT_LNG,
  supportedLngs: SUPPORTED as unknown as string[],
  // "he" should serve the "he-IL" catalog rather than fall back to English.
  nonExplicitSupportedLngs: true,
  defaultNS: "common",
  ns: ["common"],
  resources: {
    en: { common: enCommon },
    he: { common: heCommon },
  },
  interpolation: {
    // React already escapes interpolated values.
    escapeValue: false,
  },
  // A key with no translation yet renders the English source, never a blank
  // label — important while Weblate catalogs are still filling up.
  returnEmptyString: false,
});

applyDocumentLocale(i18n.resolvedLanguage ?? DEFAULT_LNG);
i18n.on("languageChanged", applyDocumentLocale);

export default i18n;
