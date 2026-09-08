import { useState, useEffect } from 'react'

export function App() {
  const [publisherId, setPublisherId] = useState('')
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [tokens, setTokens] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault()
    if (publisherId) {
      setIsLoggedIn(true)
      loadTokens(publisherId)
    }
  }

  const loadTokens = async (id: string) => {
    setLoading(true)
    try {
      // Fetching from super admin endpoint temporarily since we don't have publisher-specific auth logic
      const res = await fetch(`/api/referral/publishers/${id}/tokens`)
      const data = await res.json()
      if (data.ok) setTokens(data.data || [])
    } catch (e) {}
    setLoading(false)
  }

  if (!isLoggedIn) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-slate-200 max-w-sm w-full">
          <div className="h-12 w-12 bg-blue-100 text-blue-600 rounded-xl flex items-center justify-center mx-auto mb-4 font-bold text-xl">
            P
          </div>
          <h1 className="text-xl font-bold text-center text-slate-900 mb-6">Publisher Portal</h1>
          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1.5">Publisher ID</label>
              <input
                type="text"
                value={publisherId}
                onChange={e => setPublisherId(e.target.value)}
                className="w-full px-4 py-2 rounded-lg border border-slate-200 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 font-mono text-sm"
                placeholder="pub_..."
                required
              />
            </div>
            <button type="submit" className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 rounded-lg transition-colors">
              Login
            </button>
          </form>
        </div>
      </div>
    )
  }

  const activeSchools = tokens.filter(t => t.status === 'USED').length
  const pendingTokens = tokens.filter(t => t.status === 'UNUSED').length

  return (
    <div className="min-h-screen p-8 max-w-6xl mx-auto">
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Partner Dashboard</h1>
          <p className="text-slate-500 text-sm">Welcome back. ID: <span className="font-mono text-xs">{publisherId}</span></p>
        </div>
        <button onClick={() => setIsLoggedIn(false)} className="text-slate-500 hover:text-slate-900 text-sm font-medium">
          Sign out
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div className="bg-white p-6 rounded-2xl shadow-sm border border-slate-200">
          <div className="text-sm font-medium text-slate-500 mb-2">Active Schools</div>
          <div className="text-3xl font-bold text-slate-900">{activeSchools}</div>
        </div>
        <div className="bg-white p-6 rounded-2xl shadow-sm border border-slate-200">
          <div className="text-sm font-medium text-slate-500 mb-2">Pending Links</div>
          <div className="text-3xl font-bold text-slate-900">{pendingTokens}</div>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-200">
          <h2 className="font-bold text-slate-900">Referral History</h2>
        </div>
        {loading ? (
          <div className="p-12 text-center text-slate-500">Loading your history...</div>
        ) : tokens.length === 0 ? (
          <div className="p-12 text-center text-slate-500">No links generated yet.</div>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-50 text-slate-500 text-[11px] uppercase tracking-wider font-semibold border-b border-slate-200">
              <tr>
                <th className="px-6 py-3">Date Generated</th>
                <th className="px-6 py-3">Locked Plan</th>
                <th className="px-6 py-3">Status</th>
                <th className="px-6 py-3 text-right">Redeemed At</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {tokens.map((t, i) => (
                <tr key={i} className="hover:bg-slate-50/50">
                  <td className="px-6 py-3 text-slate-500 text-xs">{new Date(t.created_at).toLocaleDateString()}</td>
                  <td className="px-6 py-3 font-medium text-slate-900">{t.plan_name_snapshot}</td>
                  <td className="px-6 py-3">
                    <span className={`text-[10px] font-bold px-2 py-0.5 rounded-full border ${
                      t.status === 'USED' ? 'bg-green-50 text-green-700 border-green-200' : 'bg-amber-50 text-amber-700 border-amber-200'
                    }`}>
                      {t.status}
                    </span>
                  </td>
                  <td className="px-6 py-3 text-right text-slate-500 text-xs">
                    {t.used_at ? new Date(t.used_at).toLocaleDateString() : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
