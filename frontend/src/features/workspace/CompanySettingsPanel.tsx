import { useState } from 'react';
import { Field, prettyStatus, StatusRow, statusTone } from '../../shared/ui/Primitives';
import type {
  TenantBrandingDTO,
  TenantDomainDTO,
  TenantDTO,
  TenantSettingsDTO,
} from '../../types';

interface CompanySettingsPanelProps {
  tenant: TenantDTO;
  settings: TenantSettingsDTO | null;
  branding: TenantBrandingDTO | null;
  domains: TenantDomainDTO[];
  canEdit: boolean;
  busy: string;
  onUpdateProfile: (name: string, slug: string) => void;
  onUpdateSettings: (settings: {
    timezone: string;
    currency_code: string;
    locale_code: string;
    default_language: string;
  }) => void;
  onUpdateBranding: (branding: {
    display_name: string;
    primary_color: string;
    secondary_color: string;
    accent_color: string;
  }) => void;
}

export function CompanySettingsPanel({
  tenant,
  settings,
  branding,
  domains,
  canEdit,
  busy,
  onUpdateProfile,
  onUpdateSettings,
  onUpdateBranding,
}: CompanySettingsPanelProps) {
  const [name, setName] = useState(tenant.name || '');
  const [slug, setSlug] = useState(tenant.slug || '');
  const [timezone, setTimezone] = useState(settings?.timezone || 'Asia/Almaty');
  const [currencyCode, setCurrencyCode] = useState(settings?.currency_code || 'KZT');
  const [localeCode, setLocaleCode] = useState(settings?.locale_code || 'ru-KZ');
  const [defaultLanguage, setDefaultLanguage] = useState(settings?.default_language || 'ru');
  const [displayName, setDisplayName] = useState(branding?.display_name || tenant.name || '');
  const [primaryColor, setPrimaryColor] = useState(branding?.primary_color || '#176B4B');
  const [secondaryColor, setSecondaryColor] = useState(branding?.secondary_color || '#10543A');
  const [accentColor, setAccentColor] = useState(branding?.accent_color || '#59A77E');

  return (
    <section className="panel create-panel">
      <div className="panel-heading">
        <div>
          <p className="eyebrow">ПРОФИЛЬ И РЕГИОН</p>
          <h2>Настройки компании</h2>
        </div>
        <span className={`badge ${statusTone(tenant.status)}`}>{prettyStatus(tenant.status)}</span>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          onUpdateProfile(name.trim(), slug.trim().toLowerCase());
        }}
      >
        <div className="field-grid">
          <Field className="field-span" label="Название компании">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!canEdit}
              required
              maxLength={160}
            />
          </Field>
          <Field className="field-span" label="Slug компании">
            <div className="input-prefix">
              <span>rh/</span>
              <input
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                disabled={!canEdit}
                required
                maxLength={48}
                pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
              />
            </div>
          </Field>
        </div>
        {canEdit && (
          <div className="operation-actions">
            <button type="submit" className="button button-primary" disabled={busy === 'save-profile'}>
              {busy === 'save-profile' ? 'Сохраняем…' : 'Сохранить профиль'}
            </button>
          </div>
        )}
      </form>

      <hr className="section-divider" />

      <form
        onSubmit={(e) => {
          e.preventDefault();
          onUpdateSettings({
            timezone: timezone.trim(),
            currency_code: currencyCode,
            locale_code: localeCode,
            default_language: defaultLanguage,
          });
        }}
      >
        <div className="field-grid">
          <Field label="Часовой пояс">
            <input
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              disabled={!canEdit}
              required
              maxLength={64}
            />
          </Field>
          <Field label="Валюта">
            <select
              value={currencyCode}
              onChange={(e) => setCurrencyCode(e.target.value)}
              disabled={!canEdit}
            >
              <option value="KZT">KZT — тенге</option>
              <option value="USD">USD — доллар США</option>
              <option value="EUR">EUR — евро</option>
              <option value="RUB">RUB — рубль</option>
            </select>
          </Field>
          <Field label="Формат региона">
            <select
              value={localeCode}
              onChange={(e) => setLocaleCode(e.target.value)}
              disabled={!canEdit}
            >
              <option value="ru-KZ">ru-KZ — Казахстан</option>
              <option value="en-US">en-US — США</option>
              <option value="ru-RU">ru-RU — Россия</option>
              <option value="en-GB">en-GB — Великобритания</option>
            </select>
          </Field>
          <Field label="Язык по умолчанию">
            <select
              value={defaultLanguage}
              onChange={(e) => setDefaultLanguage(e.target.value)}
              disabled={!canEdit}
            >
              <option value="ru">Русский</option>
              <option value="en">English</option>
            </select>
          </Field>
        </div>
        {canEdit && (
          <div className="operation-actions">
            <button type="submit" className="button button-secondary" disabled={busy === 'save-settings'}>
              {busy === 'save-settings' ? 'Сохраняем…' : 'Обновить региональные настройки'}
            </button>
          </div>
        )}
      </form>

      <hr className="section-divider" />

      <form
        onSubmit={(e) => {
          e.preventDefault();
          onUpdateBranding({
            display_name: displayName.trim(),
            primary_color: primaryColor.trim(),
            secondary_color: secondaryColor.trim(),
            accent_color: accentColor.trim(),
          });
        }}
      >
        <div className="field-grid">
          <Field className="field-span" label="Публичное отображаемое имя (Branding)">
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              disabled={!canEdit}
              required
              maxLength={160}
            />
          </Field>
          <Field label="Основной цвет (#HEX)">
            <input
              value={primaryColor}
              onChange={(e) => setPrimaryColor(e.target.value)}
              disabled={!canEdit}
              pattern="^#[0-9A-Fa-f]{6}$"
            />
          </Field>
          <Field label="Вторичный цвет (#HEX)">
            <input
              value={secondaryColor}
              onChange={(e) => setSecondaryColor(e.target.value)}
              disabled={!canEdit}
              pattern="^#[0-9A-Fa-f]{6}$"
            />
          </Field>
          <Field className="field-span" label="Акцентный цвет (#HEX)">
            <input
              value={accentColor}
              onChange={(e) => setAccentColor(e.target.value)}
              disabled={!canEdit}
              pattern="^#[0-9A-Fa-f]{6}$"
            />
          </Field>
        </div>
        {canEdit && (
          <div className="operation-actions">
            <button type="submit" className="button button-outline" disabled={busy === 'save-branding'}>
              {busy === 'save-branding' ? 'Сохраняем…' : 'Сохранить брендинг'}
            </button>
          </div>
        )}
      </form>

      {domains.length > 0 && (
        <>
          <hr className="section-divider" />
          <div className="status-list">
            {domains.map((d) => (
              <StatusRow
                key={d.id}
                label={d.domain + (d.is_primary ? ' (Основной)' : '')}
                value={prettyStatus(d.status)}
                tone={statusTone(d.status)}
              />
            ))}
          </div>
        </>
      )}
    </section>
  );
}
