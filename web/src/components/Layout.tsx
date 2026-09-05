import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import { useTranslation } from '../i18n/LanguageContext'
import type { TranslationKey } from '../i18n/translations'
import './Layout.css'

const navItems: { to: string; key: TranslationKey; end?: boolean }[] = [
  { to: '/', key: 'nav.discovery', end: true },
  { to: '/elements', key: 'nav.elements' },
  { to: '/clusters', key: 'nav.clusters' },
  { to: '/rules', key: 'nav.rules' },
]

function LanguageSwitcher() {
  const { lang, setLang } = useTranslation()
  return (
    <div className="row" style={{ marginLeft: 'auto', gap: '0.25rem' }}>
      <button className={lang === 'ru' ? '' : 'secondary'} onClick={() => setLang('ru')}>
        RU
      </button>
      <button className={lang === 'en' ? '' : 'secondary'} onClick={() => setLang('en')}>
        EN
      </button>
    </div>
  )
}

export default function Layout({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className="layout">
      <header className="layout-header">
        <span className="layout-title">netcluster</span>
        <nav className="layout-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => (isActive ? 'nav-link nav-link-active' : 'nav-link')}
            >
              {t(item.key)}
            </NavLink>
          ))}
        </nav>
        <LanguageSwitcher />
      </header>
      <main className="layout-main">{children}</main>
    </div>
  )
}
