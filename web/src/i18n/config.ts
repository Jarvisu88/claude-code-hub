import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

import en from "./locales/en/common.json";
import zhCN from "./locales/zh-CN/common.json";
import zhTW from "./locales/zh-TW/common.json";
import ja from "./locales/ja/common.json";
import ru from "./locales/ru/common.json";

const resources = {
  en: { translation: en },
  "zh-CN": { translation: zhCN },
  "zh-TW": { translation: zhTW },
  ja: { translation: ja },
  ru: { translation: ru },
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: "en",
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["localStorage", "navigator", "htmlTag"],
      caches: ["localStorage"],
    },
  });

export default i18n;
