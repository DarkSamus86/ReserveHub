const HEX_COLOR = /^#[0-9a-fA-F]{6}$/;

export const BRAND_PALETTES = [
  { name: 'Лесная', primary: '#176B4B', secondary: '#10543A', accent: '#59A77E' },
  { name: 'Морская', primary: '#145C75', secondary: '#123F58', accent: '#54B6CC' },
  { name: 'Сливовая', primary: '#694B80', secondary: '#49315F', accent: '#B58CCB' },
  { name: 'Терракотовая', primary: '#A34732', secondary: '#713626', accent: '#E3A069' },
] as const;

export function validHexColor(value: string): boolean {
  return HEX_COLOR.test(value);
}

export function previewColor(value: string, fallback: string): string {
  return validHexColor(value) ? value : fallback;
}

function luminance(hex: string): number {
  const channels = [1, 3, 5].map((position) => {
    const value = Number.parseInt(hex.slice(position, position + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
}

export function readableTextColor(background: string): '#FFFFFF' | '#000000' {
  return luminance(background) > 0.179 ? '#000000' : '#FFFFFF';
}

interface BrandColorFieldProps {
  id: string;
  label: string;
  value: string;
  fallback: string;
  disabled: boolean;
  onChange: (value: string) => void;
}

export function BrandColorField({ id, label, value, fallback, disabled, onChange }: BrandColorFieldProps) {
  const color = previewColor(value, fallback);
  return (
    <div className="brand-color-field">
      <div className="brand-color-title">
        <label htmlFor={`${id}-hex`}>{label}</label>
        <span className="brand-color-swatch" style={{ backgroundColor: color }} aria-hidden="true" />
      </div>
      <div className="brand-color-controls">
        <input
          id={`${id}-picker`}
          type="color"
          aria-label={`Выбрать ${label.toLowerCase()} из палитры`}
          value={color}
          onChange={(event) => onChange(event.target.value.toUpperCase())}
          disabled={disabled}
        />
        <input
          id={`${id}-hex`}
          type="text"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder="#176B4B"
          pattern="#[0-9A-Fa-f]{6}"
          maxLength={7}
          spellCheck={false}
          autoComplete="off"
          aria-invalid={value.length > 0 && !validHexColor(value)}
          aria-describedby={`${id}-hint`}
          required
          disabled={disabled}
        />
      </div>
      <small id={`${id}-hint`}>
        {value.length > 0 && !validHexColor(value)
          ? 'Введите шесть цифр или букв от A до F после знака #.'
          : 'Выберите цвет или укажите код вида #176B4B.'}
      </small>
    </div>
  );
}

interface BrandingPreviewProps {
  name: string;
  primary: string;
  secondary: string;
  accent: string;
}

export function BrandingPreview({ name, primary, secondary, accent }: BrandingPreviewProps) {
  return (
    <div className="branding-preview" role="region" aria-label="Предпросмотр оформления компании">
      <div className="branding-preview-header" style={{ backgroundColor: primary, color: readableTextColor(primary) }}>
        <span className="branding-preview-mark" style={{ backgroundColor: accent, color: readableTextColor(accent) }} aria-hidden="true">R</span>
        <strong>{name || 'Название компании'}</strong>
      </div>
      <div className="branding-preview-body">
        <p>Так будут сочетаться выбранные цвета.</p>
        <div className="branding-preview-actions">
          <span className="branding-preview-button" style={{ backgroundColor: secondary, color: readableTextColor(secondary) }}>Пример кнопки</span>
          <span className="branding-preview-accent" style={{ backgroundColor: accent, color: readableTextColor(accent) }}>Акцент</span>
        </div>
      </div>
    </div>
  );
}
