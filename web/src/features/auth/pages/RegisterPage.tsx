import { Navigate } from 'react-router-dom'
import { useAuth } from '../AuthProvider'
import { AuthLayout } from '../components/AuthLayout'
import { CredentialsForm } from '../components/CredentialsForm'

export function RegisterPage() {
  const { status } = useAuth()
  if (status === 'authenticated') return <Navigate to="/" replace />
  return <AuthLayout><CredentialsForm mode="register" /></AuthLayout>
}
