import { useTranslation } from 'react-i18next';
import { Button } from './button';

export function LanguageSwitcher() {
  const { i18n } = useTranslation();

  const toggleLanguage = () => {
    const newLang = i18n.language.startsWith('en') ? 'am' : 'en';
    i18n.changeLanguage(newLang);
  };

  return (
    <Button variant="ghost" size="sm" onClick={toggleLanguage} aria-label="Toggle language">
      {i18n.language.startsWith('en') ? 'አማ' : 'EN'}
    </Button>
  );
}
