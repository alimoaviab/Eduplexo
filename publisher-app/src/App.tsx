import { useState, useEffect, useCallback } from 'react'
import { apiRequest } from './lib/api'

interface ReferredSchool {
  id: string
  school_id: string
  name: string
  code: string
  contact_email: string
  contact_phone: string
  admin_name?: string
  admin_email?: string
  login_password?: string
  login_url?: string
  status: string
  created_at: string
}

interface DashboardData {
  publisher_id: string
  publisher_name: string
  referral_token: string
  referral_url: string
  total_referred_schools: number
  schools: ReferredSchool[]
}

const TOKEN_KEY = 'eduplexo_publisher_token'

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
  const [dashboard, setDashboard] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  // Login form state
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loginError, setLoginError] = useState('')
  const [isLoggingIn, setIsLoggingIn] = useState(false)

  // School Detail Modal state
  const [selectedSchool, setSelectedSchool] = useState<ReferredSchool | null>(null)
  const [showModalPassword, setShowModalPassword] = useState(false)
  const [copiedField, setCopiedField] = useState<string | null>(null)
  const [isChangingPassword, setIsChangingPassword] = useState(false)
  const [newPasswordValue, setNewPasswordValue] = useState('')
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [isSavingPassword, setIsSavingPassword] = useState(false)
  const [passwordFeedback, setPasswordFeedback] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const loadDashboard = useCallback(async (authToken: string) => {
    setLoading(true)
    const res = await apiRequest<DashboardData>('/api/publisher/dashboard', {
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
    })

    if (res.ok && res.data) {
      setDashboard(res.data)
    } else if (res.status === 401 || res.status === 403) {
      handleLogout()
      setLoginError('Your session has expired. Please sign in again.')
    } else {
      setLoginError(res.message || 'Failed to load partner dashboard')
    }
    setLoading(false)
  }, [])

  useEffect(() => {
    if (token) {
      loadDashboard(token)
    }
  }, [token, loadDashboard])

  // ESC key to close modal
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && selectedSchool) {
        closeSchoolModal()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedSchool])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoginError('')
    setIsLoggingIn(true)

    const res = await apiRequest<{ token: string }>('/api/publisher/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        email: email.trim().toLowerCase(),
        password,
      }),
    })

    if (res.ok && res.data?.token) {
      const issuedToken = res.data.token
      localStorage.setItem(TOKEN_KEY, issuedToken)
      setToken(issuedToken)
      setEmail('')
      setPassword('')
    } else {
      setLoginError(res.message || 'Invalid email or password.')
    }
    setIsLoggingIn(false)
  }

  const handleLogout = () => {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setDashboard(null)
    setCopied(false)
    setSelectedSchool(null)
  }

  const copyReferralLink = (url: string) => {
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2500)
  }

  const copyField = (text: string, fieldId: string) => {
    if (!text) return
    navigator.clipboard.writeText(text)
    setCopiedField(fieldId)
    setTimeout(() => setCopiedField(null), 2500)
  }

  const openSchoolModal = (school: ReferredSchool) => {
    setSelectedSchool(school)
    setShowModalPassword(false)
    setIsChangingPassword(false)
    setNewPasswordValue('')
    setShowNewPassword(false)
    setPasswordFeedback(null)
  }

  const closeSchoolModal = () => {
    setSelectedSchool(null)
    setIsChangingPassword(false)
    setNewPasswordValue('')
    setPasswordFeedback(null)
  }

  const generateRandomPassword = () => {
    const uppercase = 'ABCDEFGHJKLMNPQRSTUVWXYZ'
    const lowercase = 'abcdefghijkmnpqrstuvwxyz'
    const numbers = '23456789'
    const symbols = '!@#$%^&*'
    const all = uppercase + lowercase + numbers + symbols

    let pwd = ''
    pwd += uppercase.charAt(Math.floor(Math.random() * uppercase.length))
    pwd += lowercase.charAt(Math.floor(Math.random() * lowercase.length))
    pwd += numbers.charAt(Math.floor(Math.random() * numbers.length))
    pwd += symbols.charAt(Math.floor(Math.random() * symbols.length))

    for (let i = 4; i < 12; i++) {
      pwd += all.charAt(Math.floor(Math.random() * all.length))
    }
    setNewPasswordValue(pwd)
  }

  const handleSavePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedSchool || !token) return

    const trimmed = newPasswordValue.trim()
    if (trimmed.length < 8) {
      setPasswordFeedback({ type: 'error', text: 'Password must be at least 8 characters long.' })
      return
    }

    setIsSavingPassword(true)
    setPasswordFeedback(null)

    const targetID = selectedSchool.school_id || selectedSchool.id
    const res = await apiRequest<ReferredSchool>(`/api/publisher/schools/${targetID}/password`, {
      method: 'PATCH',
      headers: {
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ password: trimmed }),
    })

    setIsSavingPassword(false)

    if (res.ok && res.data) {
      const updated = res.data
      setSelectedSchool(updated)
      setIsChangingPassword(false)
      setNewPasswordValue('')
      setPasswordFeedback({ type: 'success', text: 'School login password updated successfully!' })

      // Update in local dashboard list
      if (dashboard) {
        const nextSchools = dashboard.schools.map((s) =>
          (s.school_id === updated.school_id || s.id === updated.id) ? updated : s
        )
        setDashboard({ ...dashboard, schools: nextSchools })
      }
    } else {
      setPasswordFeedback({ type: 'error', text: res.message || 'Failed to update password.' })
    }
  }

  const copyAllCredentials = (school: ReferredSchool) => {
    const portalUrl = school.login_url || 'https://app.eduplexo.com/auth/login'
    const emailToUse = school.admin_email || school.contact_email || '—'
    const passwordToUse = school.login_password || '(Password not set)'

    const block = [
      `🏛️ Eduplexo School Administration Access`,
      `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
      `Institution: ${school.name}`,
      `School Code: ${school.code || '—'}`,
      `Portal URL: ${portalUrl}`,
      `Login Email: ${emailToUse}`,
      `Password: ${passwordToUse}`,
      `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
      `Note: Keep these credentials secure.`,
    ].join('\n')

    copyField(block, 'all')
  }

  // ─── Login Screen ────────────────────────────────────────────────────────
  if (!token || !dashboard) {
    if (token && loading) {
      return (
        <div className="min-h-screen flex items-center justify-center bg-slate-50 p-4 font-sans text-slate-700">
          <div className="flex flex-col items-center gap-3">
            <div className="h-8 w-8 border-3 border-blue-200 border-t-blue-600 rounded-full animate-spin" />
            <span className="text-sm font-medium text-slate-500">Loading partner dashboard...</span>
          </div>
        </div>
      )
    }

    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50/80 p-4 font-sans">
        <div className="bg-white p-8 sm:p-10 rounded-2xl shadow-xl shadow-slate-200/50 border border-slate-200/80 max-w-md w-full animate-in fade-in zoom-in-95 duration-200">
          <div className="flex justify-center mb-5">
            <div className="h-16 w-16 rounded-2xl overflow-hidden shadow-sm border border-slate-200 bg-white flex items-center justify-center">
              <img
                src="/logo.jpeg"
                alt="Eduplexo"
                className="h-full w-full object-cover"
                onError={(e) => {
                  ;(e.currentTarget as HTMLImageElement).src = '/favicon.png'
                }}
              />
            </div>
          </div>

          <div className="text-center mb-6">
            <h1 className="text-2xl font-black text-slate-900 tracking-tight">Partner Portal</h1>
            <p className="text-slate-500 text-xs mt-1.5">
              Sign in with your partner credentials to track school referrals.
            </p>
          </div>

          {loginError && (
            <div className="mb-5 p-3.5 bg-rose-50 border border-rose-200 rounded-xl text-rose-700 text-xs font-medium flex items-start gap-2">
              <span className="font-bold shrink-0">•</span>
              <span>{loginError}</span>
            </div>
          )}

          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-600 mb-1.5">
                Email Address
              </label>
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="partner@example.com"
                className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 text-sm transition-all"
              />
            </div>

            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-600 mb-1.5">
                Password
              </label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter your password"
                  className="w-full pl-4 pr-11 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 text-sm font-mono transition-all"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-xs font-medium text-slate-400 hover:text-slate-700"
                >
                  {showPassword ? 'Hide' : 'Show'}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={isLoggingIn}
              className="w-full mt-2 bg-blue-600 hover:bg-blue-700 text-white font-bold py-2.5 rounded-xl text-sm transition-all shadow-md shadow-blue-500/20 disabled:opacity-50 active:scale-[0.98]"
            >
              {isLoggingIn ? 'Signing in...' : 'Sign In to Partner Portal'}
            </button>
          </form>

          <div className="mt-8 pt-6 border-t border-slate-100 text-center text-xs text-slate-400">
            Partner accounts are provisioned exclusively by Eduplexo Administration.
          </div>
        </div>
      </div>
    )
  }

  // ─── Dashboard Screen ────────────────────────────────────────────────────
  const referralLink =
    dashboard.referral_url ||
    `https://app.eduplexo.com/auth/register?ref=${dashboard.referral_token}`

  return (
    <div className="min-h-screen bg-slate-50/60 font-sans text-slate-900 pb-20">
      {/* ─── Header ────────────────────────────────────────────────────────── */}
      <header className="bg-white border-b border-slate-200/90 sticky top-0 z-30 shadow-xs">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl overflow-hidden shadow-xs border border-slate-200/80 bg-white flex items-center justify-center shrink-0">
              <img
                src="/logo.jpeg"
                alt="Eduplexo"
                className="h-full w-full object-cover"
                onError={(e) => {
                  ;(e.currentTarget as HTMLImageElement).src = '/favicon.png'
                }}
              />
            </div>
            <div className="flex items-center gap-2">
              <span className="font-extrabold text-sm sm:text-base tracking-tight text-slate-900">
                Eduplexo
              </span>
              <span className="px-2 py-0.5 text-[11px] font-bold bg-blue-50 text-blue-700 rounded-md border border-blue-100">
                Partner Portal
              </span>
            </div>
          </div>

          <div className="flex items-center gap-4">
            <div className="hidden sm:flex items-center gap-2 text-right">
              <div className="h-8 w-8 rounded-full bg-blue-50 text-blue-700 font-bold text-xs flex items-center justify-center border border-blue-100">
                {dashboard.publisher_name ? dashboard.publisher_name.charAt(0).toUpperCase() : 'P'}
              </div>
              <div className="text-left">
                <div className="text-xs font-bold text-slate-800 leading-tight">
                  {dashboard.publisher_name}
                </div>
                <div className="text-[10px] font-mono text-slate-400">
                  {dashboard.referral_token}
                </div>
              </div>
            </div>

            <button
              onClick={handleLogout}
              className="px-3 py-1.5 text-xs font-semibold text-slate-600 hover:text-slate-900 hover:bg-slate-100 rounded-lg transition-colors border border-slate-200 flex items-center gap-1.5"
            >
              <svg className="w-3.5 h-3.5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
              <span>Sign out</span>
            </button>
          </div>
        </div>
      </header>

      {/* ─── Main Content Container ───────────────────────────────────────── */}
      <main className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 mt-8">
        {/* ─── Referral Hero Card ──────────────────────────────────────────── */}
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 sm:p-8 shadow-xl shadow-slate-950/10 text-white relative overflow-hidden">
          <div className="absolute top-0 right-0 w-96 h-96 bg-blue-500/10 rounded-full blur-3xl pointer-events-none -mr-20 -mt-20" />

          <div className="relative z-10">
            <div className="flex items-center gap-2 mb-3">
              <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400 text-xs font-medium border border-emerald-500/20">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                Active Partner Link
              </span>
            </div>

            <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-white">
              Welcome back, {dashboard.publisher_name}
            </h1>

            <p className="text-slate-300 text-xs sm:text-sm mt-1.5 max-w-2xl leading-relaxed">
              Share your referral link with school administrators. Schools registered through your link are automatically attributed to your account with full administrative login access.
            </p>

            <div className="mt-6 pt-5 border-t border-slate-800/90 flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <span className="text-[11px] font-bold uppercase tracking-wider text-slate-400">
                Your Referral Link
              </span>
              <div className="flex items-center gap-2 text-xs text-slate-400">
                <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
                  Referral Code:
                </span>
                <span className="font-mono font-bold text-slate-200 bg-slate-800/90 px-2 py-0.5 rounded border border-slate-700/60 text-[11px]">
                  {dashboard.referral_token}
                </span>
              </div>
            </div>

            <div className="mt-2.5 flex flex-col sm:flex-row items-stretch gap-2 bg-slate-950/90 p-1.5 rounded-xl border border-slate-800/90 focus-within:border-blue-500/50 transition-colors">
              <div className="flex-1 flex items-center min-w-0 px-3 py-2">
                <svg className="w-4 h-4 text-slate-400 shrink-0 mr-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                </svg>
                <span className="font-mono text-xs sm:text-sm text-slate-200 truncate select-all">
                  {referralLink}
                </span>
              </div>

              <button
                type="button"
                onClick={() => copyReferralLink(referralLink)}
                className={`px-5 py-2.5 font-bold text-xs sm:text-sm rounded-lg transition-all duration-200 flex items-center justify-center gap-2 shrink-0 select-none shadow-sm ${
                  copied
                    ? 'bg-emerald-600 text-white shadow-emerald-950/20'
                    : 'bg-blue-600 hover:bg-blue-500 active:scale-[0.98] text-white shadow-blue-950/30'
                }`}
              >
                {copied ? (
                  <>
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                    </svg>
                    <span>Copied</span>
                  </>
                ) : (
                  <>
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                    </svg>
                    <span>Copy Referral Link</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>

        {/* ─── Stat Cards ──────────────────────────────────────────────────── */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mt-6">
          <div className="bg-white p-6 rounded-2xl border border-slate-200/80 shadow-xs">
            <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 mb-1">
              Total Referred Schools
            </div>
            <div className="text-3xl sm:text-4xl font-extrabold text-slate-900 tracking-tight">
              {dashboard.total_referred_schools}
            </div>
            <p className="text-xs text-slate-500 mt-2 font-normal">
              Live count of schools successfully registered via your referral link.
            </p>
          </div>

          <div className="bg-white p-6 rounded-2xl border border-slate-200/80 shadow-xs flex flex-col justify-between">
            <div>
              <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 mb-1">
                Referral Status
              </div>
              <div className="flex items-center gap-2 mt-1">
                <span className="h-2.5 w-2.5 rounded-full bg-emerald-500 animate-pulse" />
                <span className="text-base sm:text-lg font-bold text-slate-800">
                  Active & Tracking
                </span>
              </div>
              <p className="text-xs text-slate-500 mt-2 font-normal">
                Credentials and attribution are accessible exclusively to your partner account.
              </p>
            </div>
            <div className="text-[11px] text-slate-400 pt-3 mt-3 border-t border-slate-100 font-mono">
              Partner ID: {dashboard.publisher_id}
            </div>
          </div>
        </div>

        {/* ─── Referred Schools Section & Table ───────────────────────────── */}
        <div className="mt-8 mb-4 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-bold text-slate-900">Referred Schools</h2>
            <p className="text-xs text-slate-500 mt-0.5">
              Schools registered using your partner link. Click any row or &quot;View Details&quot; to inspect administrative login credentials.
            </p>
          </div>
          <span className="text-xs font-semibold bg-white text-slate-700 px-3 py-1 rounded-full border border-slate-200/80 shadow-xs">
            {dashboard.schools?.length || 0} {dashboard.schools?.length === 1 ? 'School' : 'Schools'}
          </span>
        </div>

        <div className="bg-white rounded-2xl border border-slate-200/80 shadow-xs overflow-hidden">
          {!dashboard.schools || dashboard.schools.length === 0 ? (
            <div className="py-14 px-4 text-center">
              <div className="w-12 h-12 bg-slate-100 text-slate-400 rounded-xl flex items-center justify-center mx-auto mb-3">
                <svg className="w-6 h-6 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                </svg>
              </div>
              <h3 className="font-bold text-slate-800 text-sm mb-1">No Schools Registered Yet</h3>
              <p className="text-xs text-slate-500 max-w-sm mx-auto mb-4">
                When schools use your referral link to sign up, they will appear here automatically with their registration details and administrative login credentials.
              </p>
              <button
                type="button"
                onClick={() => copyReferralLink(referralLink)}
                className="px-4 py-2 bg-blue-50 text-blue-700 hover:bg-blue-100 rounded-lg text-xs font-bold transition-colors inline-flex items-center gap-1.5"
              >
                {copied ? '✓ Link Copied' : 'Copy Referral Link'}
              </button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm whitespace-nowrap">
                <thead className="bg-slate-50/80 text-slate-500 text-[11px] uppercase tracking-wider font-bold border-b border-slate-200/80">
                  <tr>
                    <th className="px-6 py-3.5">School Name</th>
                    <th className="px-6 py-3.5">School Code</th>
                    <th className="px-6 py-3.5">Login Email</th>
                    <th className="px-6 py-3.5">Registration Date</th>
                    <th className="px-6 py-3.5 text-center">Status</th>
                    <th className="px-6 py-3.5 text-right">Credentials</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {dashboard.schools.map((school) => {
                    const loginEmail = school.admin_email || school.contact_email || '—'
                    const hasPassword = Boolean(school.login_password)

                    return (
                      <tr
                        key={school.id || school.school_id}
                        onClick={() => openSchoolModal(school)}
                        className="hover:bg-slate-50/80 transition-colors cursor-pointer group"
                      >
                        <td className="px-6 py-4">
                          <div className="font-bold text-slate-900 group-hover:text-blue-600 transition-colors flex items-center gap-2">
                            <span>{school.name}</span>
                          </div>
                          {school.admin_name && (
                            <div className="text-xs text-slate-400 mt-0.5">
                              Admin: {school.admin_name}
                            </div>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <code className="px-2 py-0.5 bg-slate-100 text-slate-700 rounded text-xs font-mono font-semibold border border-slate-200/60">
                            {school.code || '—'}
                          </code>
                        </td>
                        <td className="px-6 py-4 text-xs font-mono text-slate-600">
                          {loginEmail}
                        </td>
                        <td className="px-6 py-4 text-xs text-slate-500 font-medium">
                          {new Date(school.created_at).toLocaleDateString(undefined, {
                            year: 'numeric',
                            month: 'short',
                            day: 'numeric',
                          })}
                        </td>
                        <td className="px-6 py-4 text-center">
                          <span
                            className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold border ${
                              school.status === 'active'
                                ? 'bg-emerald-50 text-emerald-700 border-emerald-200/80'
                                : 'bg-amber-50 text-amber-700 border-amber-200/80'
                            }`}
                          >
                            <span
                              className={`h-1.5 w-1.5 rounded-full ${
                                school.status === 'active' ? 'bg-emerald-500' : 'bg-amber-500'
                              }`}
                            />
                            {(school.status || 'active').charAt(0).toUpperCase() +
                              (school.status || 'active').slice(1)}
                          </span>
                        </td>
                        <td className="px-6 py-4 text-right" onClick={(e) => e.stopPropagation()}>
                          <button
                            type="button"
                            onClick={() => openSchoolModal(school)}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold rounded-lg border border-slate-200 bg-white hover:bg-blue-50 hover:border-blue-200 hover:text-blue-700 text-slate-700 transition-all shadow-2xs"
                          >
                            <svg className="w-3.5 h-3.5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                            </svg>
                            <span>{hasPassword ? 'View Login' : 'Credentials'}</span>
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </main>

      {/* ─── School Detail & Login Credentials Modal ──────────────────────── */}
      {selectedSchool && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-xs animate-in fade-in duration-200"
          onClick={closeSchoolModal}
        >
          <div
            className="bg-white rounded-2xl shadow-2xl border border-slate-200 max-w-xl w-full max-h-[90vh] overflow-y-auto animate-in zoom-in-95 duration-200"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="p-6 border-b border-slate-100 flex items-start justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="h-10 w-10 rounded-xl bg-blue-50 border border-blue-100 flex items-center justify-center text-blue-600 shrink-0 mt-0.5">
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                  </svg>
                </div>
                <div>
                  <div className="flex items-center gap-2 flex-wrap">
                    <h2 className="text-lg font-black text-slate-900 leading-snug">
                      {selectedSchool.name}
                    </h2>
                    <code className="px-2 py-0.5 bg-slate-100 text-slate-700 rounded text-xs font-mono font-bold border border-slate-200/80">
                      {selectedSchool.code || '—'}
                    </code>
                  </div>
                  <p className="text-xs text-slate-500 mt-1 flex items-center gap-2">
                    <span>Referred by you</span>
                    <span>•</span>
                    <span>Registered on {new Date(selectedSchool.created_at).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })}</span>
                  </p>
                </div>
              </div>

              <button
                type="button"
                onClick={closeSchoolModal}
                className="text-slate-400 hover:text-slate-700 p-1.5 rounded-lg hover:bg-slate-100 transition-colors"
                aria-label="Close modal"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-6 space-y-6">
              {/* Feedback toast */}
              {passwordFeedback && (
                <div
                  className={`p-3.5 rounded-xl text-xs font-medium flex items-center justify-between gap-2 ${
                    passwordFeedback.type === 'success'
                      ? 'bg-emerald-50 text-emerald-800 border border-emerald-200'
                      : 'bg-rose-50 text-rose-800 border border-rose-200'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <span>{passwordFeedback.type === 'success' ? '✓' : '•'}</span>
                    <span>{passwordFeedback.text}</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setPasswordFeedback(null)}
                    className="text-xs font-bold opacity-60 hover:opacity-100"
                  >
                    Dismiss
                  </button>
                </div>
              )}

              {/* ─── Credentials Card (Dark Navy / SaaS Style) ─────────────── */}
              <div className="bg-slate-900 rounded-2xl p-5 border border-slate-800 shadow-md text-white relative overflow-hidden">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-2">
                    <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
                    <span className="text-xs font-bold uppercase tracking-wider text-emerald-400">
                      Administrative Login Credentials
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => copyAllCredentials(selectedSchool)}
                    className={`text-[11px] font-bold px-2.5 py-1 rounded-md transition-all flex items-center gap-1.5 ${
                      copiedField === 'all'
                        ? 'bg-emerald-600 text-white'
                        : 'bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700'
                    }`}
                  >
                    {copiedField === 'all' ? (
                      <>
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                        </svg>
                        <span>Copied All</span>
                      </>
                    ) : (
                      <>
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                        </svg>
                        <span>Copy All</span>
                      </>
                    )}
                  </button>
                </div>

                <div className="space-y-3.5">
                  {/* Field: Portal URL */}
                  <div>
                    <span className="block text-[10px] font-bold uppercase tracking-wider text-slate-400 mb-1">
                      School Login Portal
                    </span>
                    <div className="flex items-center justify-between gap-2 bg-slate-950/90 px-3 py-2 rounded-xl border border-slate-800">
                      <span className="font-mono text-xs text-slate-200 truncate select-all">
                        {selectedSchool.login_url || 'https://app.eduplexo.com/auth/login'}
                      </span>
                      <div className="flex items-center gap-1.5 shrink-0">
                        <button
                          type="button"
                          onClick={() =>
                            copyField(
                              selectedSchool.login_url || 'https://app.eduplexo.com/auth/login',
                              'url'
                            )
                          }
                          className="px-2 py-1 text-[11px] font-semibold text-slate-300 hover:text-white hover:bg-slate-800 rounded transition-colors"
                        >
                          {copiedField === 'url' ? '✓ Copied' : 'Copy'}
                        </button>
                        <a
                          href={selectedSchool.login_url || 'https://app.eduplexo.com/auth/login'}
                          target="_blank"
                          rel="noreferrer"
                          className="px-2 py-1 text-[11px] font-semibold text-blue-400 hover:text-blue-300 hover:bg-slate-800 rounded transition-colors inline-flex items-center gap-1"
                        >
                          <span>Open</span>
                          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                          </svg>
                        </a>
                      </div>
                    </div>
                  </div>

                  {/* Field: Admin Login Email */}
                  <div>
                    <span className="block text-[10px] font-bold uppercase tracking-wider text-slate-400 mb-1">
                      Admin Login Email / Username
                    </span>
                    <div className="flex items-center justify-between gap-2 bg-slate-950/90 px-3 py-2 rounded-xl border border-slate-800">
                      <span className="font-mono text-xs font-semibold text-slate-200 truncate select-all">
                        {selectedSchool.admin_email || selectedSchool.contact_email || '—'}
                      </span>
                      <button
                        type="button"
                        onClick={() =>
                          copyField(
                            selectedSchool.admin_email || selectedSchool.contact_email || '',
                            'email'
                          )
                        }
                        className="px-2 py-1 text-[11px] font-semibold text-slate-300 hover:text-white hover:bg-slate-800 rounded transition-colors shrink-0"
                      >
                        {copiedField === 'email' ? '✓ Copied' : 'Copy'}
                      </button>
                    </div>
                  </div>

                  {/* Field: Password */}
                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
                        Login Password
                      </span>
                      {selectedSchool.login_password && (
                        <button
                          type="button"
                          onClick={() => setShowModalPassword(!showModalPassword)}
                          className="text-[11px] font-medium text-slate-400 hover:text-slate-200 transition-colors"
                        >
                          {showModalPassword ? 'Hide Password' : 'Show Password'}
                        </button>
                      )}
                    </div>

                    <div className="flex items-center justify-between gap-2 bg-slate-950/90 px-3 py-2 rounded-xl border border-slate-800">
                      {selectedSchool.login_password ? (
                        <>
                          <span className="font-mono text-xs font-bold text-slate-200 tracking-wider select-all">
                            {showModalPassword ? selectedSchool.login_password : '••••••••••••'}
                          </span>
                          <button
                            type="button"
                            onClick={() => copyField(selectedSchool.login_password || '', 'password')}
                            className="px-2 py-1 text-[11px] font-semibold text-slate-300 hover:text-white hover:bg-slate-800 rounded transition-colors shrink-0"
                          >
                            {copiedField === 'password' ? '✓ Copied' : 'Copy'}
                          </button>
                        </>
                      ) : (
                        <div className="flex items-center justify-between w-full">
                          <span className="text-xs text-slate-400 italic">
                            No initial password stored
                          </span>
                          <button
                            type="button"
                            onClick={() => {
                              setIsChangingPassword(true)
                              generateRandomPassword()
                            }}
                            className="px-2.5 py-1 text-[11px] font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg transition-colors shadow-2xs"
                          >
                            Set Password
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                </div>

                {/* Reset / Change Password Toggle */}
                {!isChangingPassword && selectedSchool.login_password && (
                  <div className="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between">
                    <span className="text-[11px] text-slate-400">
                      Need to update credentials for this institution?
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setIsChangingPassword(true)
                        generateRandomPassword()
                      }}
                      className="text-xs font-bold text-blue-400 hover:text-blue-300 transition-colors inline-flex items-center gap-1"
                    >
                      <span>Reset Password</span>
                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                      </svg>
                    </button>
                  </div>
                )}

                {/* Inline Change Password Form */}
                {isChangingPassword && (
                  <form onSubmit={handleSavePassword} className="mt-4 pt-4 border-t border-slate-800 space-y-3">
                    <div className="flex items-center justify-between">
                      <label className="text-[11px] font-bold uppercase tracking-wider text-slate-300">
                        New Administrator Password
                      </label>
                      <button
                        type="button"
                        onClick={generateRandomPassword}
                        className="text-[11px] font-semibold text-blue-400 hover:text-blue-300"
                      >
                        ⚡ Generate Random
                      </button>
                    </div>

                    <div className="relative">
                      <input
                        type={showNewPassword ? 'text' : 'password'}
                        required
                        value={newPasswordValue}
                        onChange={(e) => setNewPasswordValue(e.target.value)}
                        placeholder="Min. 8 characters"
                        className="w-full bg-slate-950 px-3 py-2 pr-16 rounded-xl border border-slate-700 text-xs font-mono text-white focus:outline-none focus:border-blue-500"
                      />
                      <button
                        type="button"
                        onClick={() => setShowNewPassword(!showNewPassword)}
                        className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[11px] font-medium text-slate-400 hover:text-white"
                      >
                        {showNewPassword ? 'Hide' : 'Show'}
                      </button>
                    </div>

                    <div className="flex items-center justify-end gap-2 pt-1">
                      <button
                        type="button"
                        onClick={() => {
                          setIsChangingPassword(false)
                          setNewPasswordValue('')
                        }}
                        className="px-3 py-1.5 text-xs font-semibold text-slate-400 hover:text-slate-200 rounded-lg"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        disabled={isSavingPassword}
                        className="px-4 py-1.5 text-xs font-bold bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg transition-all shadow-sm"
                      >
                        {isSavingPassword ? 'Saving...' : 'Save New Password'}
                      </button>
                    </div>
                  </form>
                )}
              </div>

              {/* ─── Institution Profile Info ──────────────────────────────── */}
              <div>
                <h3 className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3">
                  Institution Overview
                </h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                  <div className="p-3 bg-slate-50 rounded-xl border border-slate-100">
                    <span className="text-slate-400 font-medium block">Admin Full Name</span>
                    <span className="font-bold text-slate-800 mt-0.5 block">
                      {selectedSchool.admin_name || '—'}
                    </span>
                  </div>

                  <div className="p-3 bg-slate-50 rounded-xl border border-slate-100">
                    <span className="text-slate-400 font-medium block">Contact Phone</span>
                    <span className="font-bold text-slate-800 mt-0.5 block">
                      {selectedSchool.contact_phone || '—'}
                    </span>
                  </div>

                  <div className="p-3 bg-slate-50 rounded-xl border border-slate-100">
                    <span className="text-slate-400 font-medium block">School ID</span>
                    <span className="font-mono text-slate-700 mt-0.5 block truncate">
                      {selectedSchool.school_id || selectedSchool.id}
                    </span>
                  </div>

                  <div className="p-3 bg-slate-50 rounded-xl border border-slate-100">
                    <span className="text-slate-400 font-medium block">Account Status</span>
                    <span className="font-bold text-emerald-700 mt-0.5 block">
                      {(selectedSchool.status || 'active').toUpperCase()}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="p-4 sm:p-6 bg-slate-50/80 border-t border-slate-100 rounded-b-2xl flex items-center justify-between">
              <span className="text-[11px] text-slate-400">
                Visible exclusively to partner {dashboard.publisher_name}.
              </span>
              <button
                type="button"
                onClick={closeSchoolModal}
                className="px-4 py-2 bg-white border border-slate-200 hover:bg-slate-100 font-bold text-xs text-slate-700 rounded-xl transition-colors shadow-2xs"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
