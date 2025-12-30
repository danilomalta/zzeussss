export default function Input({ label, type = 'text', value, onChange, placeholder, required, error, ...props }) {
  const hasError = !!error

  return (
    <label style={{ display: 'block', marginBottom: 12 }}>
      {label && (
        <div style={{ marginBottom: 6, opacity: 0.9, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>{label}</span>
          {required && <span style={{ fontSize: 12, color: 'var(--color-muted)' }}>*</span>}
        </div>
      )}
      <input
        type={type}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        required={required}
        aria-invalid={hasError}
        aria-describedby={hasError ? `${props.id || 'input'}-error` : undefined}
        {...props}
        style={{
          width: '100%',
          padding: '12px 14px',
          borderRadius: 10,
          border: '1px solid',
          borderColor: hasError
            ? 'var(--color-error, #ef4444)'
            : 'color-mix(in srgb, var(--color-text) 14%, transparent)',
          background: 'color-mix(in srgb, var(--color-bg) 90%, #fff 10%)',
          color: 'var(--color-text)',
          transition: 'border-color 0.2s ease',
          ...(hasError && {
            boxShadow: '0 0 0 3px rgba(239, 68, 68, 0.1)',
          }),
        }}
      />
      {hasError && (
        <div
          id={`${props.id || 'input'}-error`}
          role="alert"
          style={{
            fontSize: 12,
            color: 'var(--color-error, #ef4444)',
            marginTop: 4,
            display: 'flex',
            alignItems: 'center',
            gap: 4,
          }}
        >
          <span>⚠</span>
          <span>{error}</span>
        </div>
      )}
    </label>
  )
}
