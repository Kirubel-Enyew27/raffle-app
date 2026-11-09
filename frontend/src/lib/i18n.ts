import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import enTranslation from './locales/en.json';
import amTranslation from './locales/am.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: {
        translation: enTranslation,
      },
      am: {
        translation: amTranslation,
      },
    },
    fallbackLng: 'en',
    debug: false,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
  });

// Keep the document language in sync so assistive tech, spell-check and
// browser font/glyph selection follow the selected language.
function syncDocumentLanguage(lng?: string) {
  if (typeof document !== 'undefined' && lng) {
    document.documentElement.lang = lng.split('-')[0]
  }
}

i18n.on('languageChanged', syncDocumentLanguage)
i18n.on('initialized', () => syncDocumentLanguage(i18n.language))
if (i18n.isInitialized) syncDocumentLanguage(i18n.language)

export default i18n;
