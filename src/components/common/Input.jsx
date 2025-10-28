export default function Input({ label, type = 'text', value, onChange, placeholder, required, ...props }) {
  return (
    <label style={{ display: 'block', marginBottom: 12 }}>
      {label && <div style={{ marginBottom: 6, opacity: 0.9 }}>{label}</div>}
      <input
        type={type}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        required={required}
        {...props}
        style={{
          width: '100%',
          padding: '12px 14px',
          borderRadius: 10,
          border: '1px solid',
          borderColor: 'color-mix(in srgb, var(--color-text) 14%, transparent)',
          background: 'color-mix(in srgb, var(--color-bg) 90%, #fff 10%)',
          color: 'var(--color-text)'
        }}
      />
    </label>
  )
}
