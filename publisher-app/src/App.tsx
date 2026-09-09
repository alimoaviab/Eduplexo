import { useState, useEffect, useCallback } from 'react'
import { apiRequest } from './lib/api'

interface ReferredSchool {
  id: string
  school_id: string
  name: string
  code: string
  contact_email: string
  contact_phone: string
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
      // Token invalid or suspended
      handleLogout()
      if (res.message) {
        setLoginError(res.message)
      }
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
  }

  const copyReferralLink = (url: string) => {
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2500)
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
                  (e.currentTarget as HTMLImageElement).src = '/favicon.png'
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
      {/* ─── 7. Refined Header ────────────────────────────────────────────── */}
      <header className="bg-white border-b border-slate-200/90 sticky top-0 z-30 shadow-xs">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl overflow-hidden shadow-xs border border-slate-200/80 bg-white flex items-center justify-center shrink-0">
              <img
                src="/logo.jpeg"
                alt="Eduplexo"
                className="h-full w-full object-cover"
                onError={(e) => {
                  (e.currentTarget as HTMLImageElement).src = '/favicon.png'
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
        {/* ─── 2. Referral Hero Card (Dark Navy, SaaS Polished) ────────────── */}
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 sm:p-8 shadow-xl shadow-slate-950/10 text-white relative overflow-hidden">
          {/* Subtle background glow */}
          <div className="absolute top-0 right-0 w-96 h-96 bg-blue-500/10 rounded-full blur-3xl pointer-events-none -mr-20 -mt-20" />

          <div className="relative z-10">
            {/* 3. Hierarchy 1: Small status indicator */}
            <div className="flex items-center gap-2 mb-3">
              <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400 text-xs font-medium border border-emerald-500/20">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                Active Partner Link
              </span>
            </div>

            {/* 3. Hierarchy 2 & 8: Welcome message with dynamic Publisher name */}
            <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-white">
              Welcome back, {dashboard.publisher_name}
            </h1>

            {/* 3. Hierarchy 3 & 8: Short supporting message */}
            <p className="text-slate-300 text-xs sm:text-sm mt-1.5 max-w-2xl leading-relaxed">
              Share your referral link with school administrators. Schools created through your link are automatically attributed to your account.
            </p>

            {/* 3. Hierarchy 4: Referral code - connected and secondary */}
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

            {/* 4 & 5. Hierarchy 5 & 6: Referral URL Area with Single Primary Copy Action */}
            <div className="mt-2.5 flex flex-col sm:flex-row items-stretch gap-2 bg-slate-950/90 p-1.5 rounded-xl border border-slate-800/90 focus-within:border-blue-500/50 transition-colors">
              <div className="flex-1 flex items-center min-w-0 px-3 py-2">
                <svg className="w-4 h-4 text-slate-400 shrink-0 mr-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                </svg>
                <span className="font-mono text-xs sm:text-sm text-slate-200 truncate select-all">
                  {referralLink}
                </span>
              </div>

              {/* Single Clear Primary CTA */}
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

        {/* ─── 9 & 10. Refined Stat Cards ──────────────────────────────────── */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mt-6">
          {/* Primary Metric: Total Referred Schools */}
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

          {/* Secondary Metric: Referral Status */}
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
                Attribution is linked automatically when an institution signs up.
              </p>
            </div>
            <div className="text-[11px] text-slate-400 pt-3 mt-3 border-t border-slate-100 font-mono">
              Partner ID: {dashboard.publisher_id}
            </div>
          </div>
        </div>

        {/* ─── 11 & 12. Referred Schools Section & Table ───────────────────── */}
        <div className="mt-8 mb-4 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-bold text-slate-900">Referred Schools</h2>
            <p className="text-xs text-slate-500 mt-0.5">
              Schools successfully attributed to your referral link.
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
                When schools use your referral link to sign up, they will appear here automatically with their registration details.
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
                    <th className="px-6 py-3.5">Registration Date</th>
                    <th className="px-6 py-3.5 text-right">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {dashboard.schools.map((school) => (
                    <tr key={school.id || school.school_id} className="hover:bg-slate-50/70 transition-colors">
                      <td className="px-6 py-4">
                        <div className="font-bold text-slate-900">{school.name}</div>
                        {school.contact_email && (
                          <div className="text-xs text-slate-400 mt-0.5">{school.contact_email}</div>
                        )}
                      </td>
                      <td className="px-6 py-4">
                        <code className="px-2 py-0.5 bg-slate-100 text-slate-700 rounded text-xs font-mono font-semibold border border-slate-200/60">
                          {school.code || '—'}
                        </code>
                      </td>
                      <td className="px-6 py-4 text-xs text-slate-500 font-medium">
                        {new Date(school.created_at).toLocaleDateString(undefined, {
                          year: 'numeric',
                          month: 'short',
                          day: 'numeric',
                        })}
                      </td>
                      <td className="px-6 py-4 text-right">
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
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
