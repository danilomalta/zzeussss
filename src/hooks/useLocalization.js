import { useTranslation } from 'react-i18next'

export default function useLocalization() {
  const { t } = useTranslation()
  return t
}
