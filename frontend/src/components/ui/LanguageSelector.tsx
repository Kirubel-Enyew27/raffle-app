import { useTranslation } from 'react-i18next'
import { Globe } from 'lucide-react'
import { Select } from '@/components/ui/select'

const languages = [
  { code: 'en', nativeName: 'English' },
  { code: 'am', nativeName: 'አማርኛ' },
]

export function LanguageSelector() {
  const { i18n } = useTranslation()

  const handleLanguageChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    i18n.changeLanguage(e.target.value)
  }

  return (
    <div className="flex items-center gap-2">
      <Globe className="h-4 w-4 shrink-0 text-muted-foreground" />
      <Select value={i18n.language.split('-')[0]} onChange={handleLanguageChange} className="w-[130px]">
        {languages.map((lang) => (
          <option key={lang.code} value={lang.code}>
            {lang.nativeName}
          </option>
        ))}
      </Select>
    </div>
  )
}
