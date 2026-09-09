import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiRequest, clearStoredSession } from '@/lib/api'

export function LoginPage() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // If already logged in, redirect to dashboard
  useEffect(() => {
    apiRequest<{ role: string }>('/api/auth/session').then((res) => {
      if (res.ok && res.data?.role === 'super_admin') navigate('/dashboard', { replace: true })
    })
  }, [navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const res = await apiRequest<{
      role: string
      token: string
      user_id: string
      email: string
      school_id: string
      active_academic_year_id?: string
    }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: email.trim().toLowerCase(), password }),
    })

    setLoading(false)

    if (!res.ok || !res.data) {
      setError(res.message || 'Invalid email or password')
      return
    }

    const data = res.data

    // Verify role - only platform super_admin can access this panel
    if (data.role !== 'super_admin') {
      setError('Access denied. This panel is only for platform administrators.')
      return
    }

    clearStoredSession()
    sessionStorage.setItem('sa_user', JSON.stringify({
      id: data.user_id,
      email: data.email,
      role: data.role,
      school_id: data.school_id,
    }))

    navigate('/dashboard', { replace: true })
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-50 to-blue-50 p-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="h-16 w-16 rounded-2xl overflow-hidden shadow-lg shadow-blue-600/15 border border-slate-200/80 bg-white mx-auto mb-4 flex items-center justify-center p-1.5 ring-1 ring-slate-100">
            <img src="/logo.jpeg" alt="Eduplexo" className="h-full w-full object-contain rounded-xl" />
          </div>
          <h1 className="text-2xl font-black text-slate-900 tracking-tight">Super Admin</h1>
          <p className="text-xs text-slate-500 mt-1 font-medium">Eduplexo Platform Control Panel</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-white rounded-2xl border border-slate-200 p-6 shadow-xl shadow-slate-200/50 space-y-4">
          {error && (
            <div className="p-3 rounded-lg bg-red-50 border border-red-100 text-sm text-red-700 font-medium flex items-start gap-2">
              <AppIcon name="AlertCircle" size={16} className="flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1.5">Email Address</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              className="w-full h-11 px-3.5 rounded-lg border border-slate-200 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all"
              placeholder="eduplexo@gmail.com"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1.5">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="current-password"
              className="w-full h-11 px-3.5 rounded-lg border border-slate-200 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all"
              placeholder="••••••••"
            />
          </div>

          <button
            type="submit"
            disabled={loading || !email || !password}
            className="w-full h-11 bg-blue-600 text-white text-sm font-semibold rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {loading ? (
              <>
                <span className="h-4 w-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                Signing in...
              </>
            ) : (
              <>
                <AppIcon name="LogIn" size={16} />
                Sign In
              </>
            )}
          </button>

          
        </form>

        <p className="text-center text-xs text-slate-400 mt-6">
          © 2026 Eduplexo. Platform administrators only.
        </p>
      </div>
    </div>
  )
}
