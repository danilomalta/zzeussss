import { useNavigate } from 'react-router-dom'
import AuthForm from '../components/features/AuthForm.jsx'

export default function AuthPage() {
  const navigate = useNavigate()
  return (
    <div className="container" style={{ padding: '60px 0' }}>
      <AuthForm onRegistered={() => navigate('/onboarding')} />
    </div>
  )
}
