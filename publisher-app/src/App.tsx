import { useState, useEffect, useCallback } from 'react'

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
    try {
      const res = await fetch('/api/publisher/dashboard', {
        headers: {
          Authorization: `Bearer ${authToken}`,
        },
      })
      const data = await res.json()
      if (res.ok && data.ok) {
        setDashboard(data.data)
      } else if (res.status === 401 || res.status === 403) {
        // Token invalid or suspended
        handleLogout()
        if (data.message) {
          setLoginError(data.message)
        }
      } else {
        setLoginError(data.message || 'Failed to load partner dashboard')
      }
    } catch {
      setLoginError('Network error connecting to partner server')
    } finally {
      setLoading(false)
    }
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

    try {
      const res = await fetch('/api/publisher/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: email.trim().toLowerCase(),
          password,
        }),
      })

      const result = await res.json()
      if (res.ok && result.ok && result.data?.token) {
        const issuedToken = result.data.token
        localStorage.setItem(TOKEN_KEY, issuedToken)
        setToken(issuedToken)
        setEmail('')
        setPassword('')
      } else {
        setLoginError(result.message || 'Invalid email or password.')
      }
    } catch {
      setLoginError('Unable to connect to login server. Please try again.')
    } finally {
      setIsLoggingIn(false)
    }
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
      <div className="min-h-screen flex items-center justify-center bg-slate-50/70 p-4 font-sans">
        <div className="bg-white p-8 sm:p-10 rounded-3xl shadow-xl shadow-slate-200/50 border border-slate-200/80 max-w-md w-full animate-in fade-in zoom-in-95 duration-200">
          <div className="flex justify-center mb-5">
            <img
              src="/favicon.png"
              alt="Eduplexo Logo"
              className="h-16 w-16 rounded-2xl shadow-md p-2.5 bg-gradient-to-tr from-blue-50 to-indigo-50 border border-blue-100 object-contain"
            />
          </div>
          
          <div className="text-center mb-6">
            <h1 className="text-2xl font-black text-slate-900 tracking-tight">Partner Portal</h1>
            <p className="text-slate-500 text-xs mt-1">
              Sign in with your partner credentials to track school referrals.
            </p>
          </div>

          {loginError && (
            <div className="mb-5 p-3.5 bg-rose-50 border border-rose-200 rounded-2xl text-rose-700 text-xs font-medium flex items-start gap-2">
              <span className="font-bold">•</span>
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
                onChange={e => setEmail(e.target.value)}
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
                  onChange={e => setPassword(e.target.value)}
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
    <div className="min-h-screen bg-slate-50/50 font-sans text-slate-900 pb-16">
      {/* Top Navbar */}
      <header className="bg-white border-b border-slate-200 sticky top-0 z-30 shadow-xs">
        <div className="max-w-6xl mx-auto px-4 sm:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <img
              src="/favicon.png"
              alt="Eduplexo"
              className="h-9 w-9 rounded-xl object-contain p-1 bg-blue-50 border border-blue-100 shadow-xs"
            />
            <div>
              <span className="font-extrabold text-sm sm:text-base tracking-tight text-slate-900">
                Eduplexo Partner Portal
              </span>
              <span className="hidden sm:inline-block ml-2 px-2 py-0.5 text-[10px] font-bold bg-blue-50 text-blue-700 rounded-md border border-blue-100">
                Referral Tracking
              </span>
            </div>
          </div>

          <div className="flex items-center gap-4">
            <div className="hidden sm:block text-right">
              <div className="text-xs font-bold text-slate-800">{dashboard.publisher_name}</div>
              <div className="text-[11px] font-mono text-slate-400">Token: {dashboard.referral_token}</div>
            </div>
            <button
              onClick={handleLogout}
              className="px-3.5 py-1.5 text-xs font-semibold text-slate-600 hover:text-slate-900 hover:bg-slate-100 rounded-xl transition-colors border border-slate-200"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-6xl mx-auto px-4 sm:px-8 mt-8">
        {/* Welcome & Referral Link Hero Card */}
        <div className="bg-gradient-to-br from-slate-900 via-slate-800 to-indigo-950 text-white rounded-3xl p-6 sm:p-8 shadow-xl shadow-slate-900/10 mb-8 border border-slate-800">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
            <div>
              <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-blue-500/20 text-blue-300 text-xs font-semibold mb-3 border border-blue-400/20">
                <span>Active Partner Link</span>
              </div>
              <h2 className="text-2xl sm:text-3xl font-black tracking-tight">
                Welcome back, {dashboard.publisher_name}
              </h2>
              <p className="text-slate-300 text-xs sm:text-sm mt-1 max-w-xl">
                Share your unique referral link with school administrators. Any school created through this link is instantly and permanently attributed to your partner account.
              </p>
            </div>

            <div className="flex items-center gap-3">
              <div className="bg-white/10 backdrop-blur-md px-5 py-3 rounded-2xl border border-white/10 text-center">
                <div className="text-xs uppercase tracking-wider text-slate-400 font-semibold">Your Referral Code</div>
                <div className="text-xl font-mono font-black text-amber-300 mt-0.5">{dashboard.referral_token}</div>
              </div>
            </div>
          </div>

          {/* Referral Link Box */}
          <div className="mt-6 pt-6 border-t border-white/10">
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
              Your Shareable Referral URL
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <div className="flex-1 bg-black/40 border border-white/15 rounded-2xl px-4 py-2.5 flex items-center overflow-hidden">
                <input
                  type="text"
                  readOnly
                  value={referralLink}
                  className="w-full bg-transparent text-xs sm:text-sm font-mono text-emerald-300 focus:outline-none select-all"
                />
              </div>
              <button
                onClick={() => copyReferralLink(referralLink)}
                className="px-5 py-2.5 bg-blue-500 hover:bg-blue-600 active:scale-95 text-white font-bold text-xs sm:text-sm rounded-2xl transition-all shadow-md flex items-center justify-center gap-2"
              >
                {copied ? '✓ Copied to Clipboard!' : 'Copy Referral Link'}
              </button>
              <a
                href={referralLink}
                target="_blank"
                rel="noopener noreferrer"
                className="px-4 py-2.5 bg-white/10 hover:bg-white/20 text-white font-medium text-xs sm:text-sm rounded-2xl transition-colors text-center"
              >
                Test Link ↗
              </a>
            </div>
          </div>
        </div>

        {/* Big Metric Card */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-8">
          <div className="bg-white p-6 rounded-3xl border border-slate-200 shadow-sm">
            <div className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-1">
              Total Schools Created
            </div>
            <div className="text-4xl sm:text-5xl font-black text-blue-600 tracking-tight">
              {dashboard.total_referred_schools}
            </div>
            <p className="text-xs text-slate-500 mt-2 font-medium">
              Live count of schools successfully registered via your referral code.
            </p>
          </div>

          <div className="bg-white p-6 rounded-3xl border border-slate-200 shadow-sm flex flex-col justify-between">
            <div>
              <div className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-1">
                Attribution Status
              </div>
              <div className="flex items-center gap-2 mt-1">
                <span className="h-3 w-3 rounded-full bg-emerald-500 animate-pulse" />
                <span className="text-xl font-black text-slate-800">Active & Tracking</span>
              </div>
              <p className="text-xs text-slate-500 mt-2 font-medium">
                Referral attribution is processed automatically upon school signup.
              </p>
            </div>
            <div className="text-[11px] text-slate-400 pt-3 border-t border-slate-100 font-mono">
              Partner ID: {dashboard.publisher_id}
            </div>
          </div>
        </div>

        {/* Referred Schools Audit Table */}
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h3 className="text-lg font-extrabold text-slate-900 tracking-tight">Referred Schools</h3>
            <p className="text-xs text-slate-500">List of schools attributed to your referral link.</p>
          </div>
          <span className="text-xs font-bold bg-white text-slate-700 px-3 py-1 rounded-full border border-slate-200 shadow-xs">
            {dashboard.schools?.length || 0} Registered
          </span>
        </div>

        <div className="bg-white rounded-3xl border border-slate-200 shadow-sm overflow-hidden">
          {!dashboard.schools || dashboard.schools.length === 0 ? (
            <div className="p-16 text-center">
              <div className="w-14 h-14 bg-slate-50 text-slate-400 rounded-2xl flex items-center justify-center mx-auto mb-3 border border-slate-100 text-xl font-bold">
                🏫
              </div>
              <h4 className="font-bold text-slate-800 text-base mb-1">No Schools Registered Yet</h4>
              <p className="text-xs text-slate-500 max-w-sm mx-auto mb-4">
                When schools use your referral link to sign up, they will appear here automatically with their registration details.
              </p>
              <button
                onClick={() => copyReferralLink(referralLink)}
                className="px-4 py-2 bg-blue-50 text-blue-700 hover:bg-blue-100 rounded-xl text-xs font-bold transition-colors inline-flex items-center gap-1.5"
              >
                {copied ? '✓ Link Copied!' : 'Copy Your Referral Link'}
              </button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm whitespace-nowrap">
                <thead className="bg-slate-50/80 text-slate-500 text-[11px] uppercase tracking-wider font-bold border-b border-slate-200">
                  <tr>
                    <th className="px-6 py-4">School Name</th>
                    <th className="px-6 py-4">School Code</th>
                    <th className="px-6 py-4">Registration Date</th>
                    <th className="px-6 py-4 text-right">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {dashboard.schools.map(school => (
                    <tr key={school.id || school.school_id} className="hover:bg-slate-50/70 transition-colors">
                      <td className="px-6 py-4">
                        <div className="font-bold text-slate-900">{school.name}</div>
                      </td>
                      <td className="px-6 py-4">
                        <code className="px-2 py-0.5 bg-slate-100 text-slate-700 rounded text-xs font-mono font-bold">
                          {school.code || '—'}
                        </code>
                      </td>
                      <td className="px-6 py-4 text-xs text-slate-500">
                        {new Date(school.created_at).toLocaleDateString(undefined, {
                          year: 'numeric',
                          month: 'short',
                          day: 'numeric',
                        })}
                      </td>
                      <td className="px-6 py-4 text-right">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-bold border ${
                            school.status === 'active'
                              ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                              : 'bg-amber-50 text-amber-700 border-amber-200'
                          }`}
                        >
                          {(school.status || 'ACTIVE').toUpperCase()}
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
