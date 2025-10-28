export default function Spinner() {
  return (
    <span style={{ display: 'inline-block', width: 18, height: 18, border: '2px solid color-mix(in srgb, var(--color-text) 30%, transparent)', borderTopColor: 'var(--color-primary)', borderRadius: '50%', animation: 'spin 1s linear infinite' }} />
  )
}
