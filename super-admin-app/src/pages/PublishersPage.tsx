import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { apiRequest } from '@/lib/api'
import { showToast } from '@/utils/toast'

interface Publisher {
  id: string
  name: string
  email: string
  referral_token: string
  referral_url: string
  status: 'active' | 'suspended' | 'deleted'
  referred_schools_count: number
  created_at: string
}

interface CreatedPublisherResult {
  id: string
  name: string
  email: string
  password: string
  referral_token: string
  referral_url: string
}

export function PublishersPage() {
  const [publishers, setPublishers] = useState<Publisher[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')

  // Create Modal State
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [createdResult, setCreatedResult] = useState<CreatedPublisherResult | null>(null)

  // Status toggle state
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)

  const loadPublishers = async () => {
    setLoading(true)
    const res = await apiRequest<Publisher[]>('/api/super-admin/publishers')
    if (res.ok && res.data) {
      setPublishers(res.data)
    } else {
      showToast(res.message || 'Failed to load publishers', 'error')
    }
    setLoading(false)
  }

  useEffect(() => {
    loadPublishers()
  }, [])

  const generateRandomPassword = () => {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%^&*'
    let pwd = ''
    for (let i = 0; i < 12; i++) {
      pwd += chars.charAt(Math.floor(Math.random() * chars.length))
    }
    setPassword(pwd)
  }

  const handleOpenCreateModal = () => {
    setName('')
    setEmail('')
    generateRandomPassword()
    setShowPassword(false)
    setCreatedResult(null)
    setShowCreateModal(true)
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !email.trim() || !password) {
      showToast('All fields are required', 'error')
      return
    }

    if (password.length < 8) {
      showToast('Password must be at least 8 characters', 'error')
      return
    }

    setIsSubmitting(true)
    const res = await apiRequest<Publisher>('/api/super-admin/publishers', {
      method: 'POST',
      body: JSON.stringify({
        name: name.trim(),
        email: email.trim().toLowerCase(),
        password: password,
      }),
    })
    setIsSubmitting(false)

    if (res.ok && res.data) {
      showToast('Publisher partner created successfully', 'success')
      setCreatedResult({
        id: res.data.id,
        name: res.data.name,
        email: res.data.email,
        password: password,
        referral_token: res.data.referral_token,
        referral_url: res.data.referral_url || `https://app.eduplexo.com/auth/register?ref=${res.data.referral_token}`,
      })
      loadPublishers()
    } else {
      showToast(res.message || 'Failed to create publisher', 'error')
    }
  }

  const handleToggleStatus = async (pub: Publisher) => {
    const isSuspending = pub.status === 'active'
    const endpoint = isSuspending
      ? `/api/super-admin/publishers/${pub.id}/suspend`
      : `/api/super-admin/publishers/${pub.id}/reactivate`

    setActionLoadingId(pub.id)
    const res = await apiRequest(endpoint, { method: 'POST' })
    setActionLoadingId(null)

    if (res.ok) {
      showToast(`Publisher ${isSuspending ? 'suspended' : 'reactivated'} successfully`, 'success')
      setPublishers(prev =>
        prev.map(p => (p.id === pub.id ? { ...p, status: isSuspending ? 'suspended' : 'active' } : p))
      )
    } else {
      showToast(res.message || 'Action failed', 'error')
    }
  }

  const handleDelete = async (pub: Publisher) => {
    if (!window.confirm(`Are you sure you want to delete publisher "${pub.name}"? Referred schools will be preserved.`)) {
      return
    }

    setActionLoadingId(pub.id)
    const res = await apiRequest(`/api/super-admin/publishers/${pub.id}`, { method: 'DELETE' })
    setActionLoadingId(null)

    if (res.ok) {
      showToast('Publisher deleted successfully', 'success')
      setPublishers(prev => prev.filter(p => p.id !== pub.id))
    } else {
      showToast(res.message || 'Failed to delete publisher', 'error')
    }
  }

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text)
    showToast(`${label} copied to clipboard!`, 'success')
  }

  const filteredPublishers = useMemo(() => {
    if (!searchQuery.trim()) return publishers
    const q = searchQuery.toLowerCase()
    return publishers.filter(
      p =>
        p.name.toLowerCase().includes(q) ||
        p.email.toLowerCase().includes(q) ||
        p.referral_token.toLowerCase().includes(q)
    )
  }, [publishers, searchQuery])

  const stats = useMemo(() => {
    const total = publishers.length
    const active = publishers.filter(p => p.status === 'active').length
    const totalReferred = publishers.reduce((acc, p) => acc + (p.referred_schools_count || 0), 0)
    return { total, active, totalReferred }
  }, [publishers])

  return (
    <div className="p-8 max-w-7xl mx-auto min-h-screen">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 flex items-center gap-2">
            <AppIcon name="Link2" className="h-6 w-6 text-blue-600" />
            Publishers & Referral Tracking
          </h1>
          <p className="text-slate-500 text-sm mt-1">
            Manage partner credentials and track schools created through referral links.
          </p>
        </div>
        <button
          onClick={handleOpenCreateModal}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2.5 rounded-xl text-sm font-semibold transition-all shadow-sm flex items-center gap-2 active:scale-95"
        >
          <AppIcon name="Plus" className="h-4 w-4" />
          Add Publisher
        </button>
      </div>

      {/* Metrics Row */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
            Total Partners
          </div>
          <div className="text-2xl font-extrabold text-slate-900">{stats.total}</div>
        </div>
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
            Active Partners
          </div>
          <div className="text-2xl font-extrabold text-emerald-600">{stats.active}</div>
        </div>
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
            Total Referred Schools
          </div>
          <div className="text-2xl font-extrabold text-blue-600">{stats.totalReferred}</div>
        </div>
      </div>

      {/* Search Bar */}
      <div className="mb-6 flex items-center gap-3">
        <div className="relative flex-1 max-w-md">
          <AppIcon name="Search" className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            placeholder="Search by name, email, or referral code..."
            className="w-full pl-10 pr-4 py-2 bg-white border border-slate-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all shadow-sm"
          />
        </div>
        {searchQuery && (
          <button
            onClick={() => setSearchQuery('')}
            className="text-xs text-slate-500 hover:text-slate-700 font-medium px-2 py-1"
          >
            Clear
          </button>
        )}
      </div>

      {/* Publishers Table */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-16 text-center text-slate-400 flex flex-col items-center justify-center gap-3">
            <div className="h-7 w-7 border-2 border-blue-200 border-t-blue-600 rounded-full animate-spin" />
            <span className="text-sm font-medium">Loading publishers...</span>
          </div>
        ) : filteredPublishers.length === 0 ? (
          <div className="p-16 text-center">
            <div className="bg-slate-50 w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-slate-100">
              <AppIcon name="Link2" className="h-8 w-8 text-slate-400" />
            </div>
            <h3 className="text-slate-900 font-bold text-base mb-1">
              {searchQuery ? 'No matching publishers found' : 'No Publishers Created Yet'}
            </h3>
            <p className="text-slate-500 text-sm max-w-sm mx-auto mb-5">
              {searchQuery
                ? 'Try refining your search query.'
                : 'Create your first publisher partner to generate unique referral links.'}
            </p>
            {!searchQuery && (
              <button
                onClick={handleOpenCreateModal}
                className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-xl text-sm font-semibold transition-all inline-flex items-center gap-2 shadow-sm"
              >
                <AppIcon name="Plus" className="h-4 w-4" />
                Add Publisher
              </button>
            )}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-slate-50 text-slate-500 text-[11px] uppercase tracking-wider font-semibold border-b border-slate-200">
                <tr>
                  <th className="px-6 py-4">Partner Details</th>
                  <th className="px-6 py-4">Referral Code</th>
                  <th className="px-6 py-4">Referral Link</th>
                  <th className="px-6 py-4 text-center">Referred Schools</th>
                  <th className="px-6 py-4">Status</th>
                  <th className="px-6 py-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {filteredPublishers.map(pub => {
                  const refUrl = pub.referral_url || `https://app.eduplexo.com/auth/register?ref=${pub.referral_token}`
                  const isActionBusy = actionLoadingId === pub.id

                  return (
                    <tr key={pub.id} className="hover:bg-slate-50/70 transition-colors">
                      <td className="px-6 py-4">
                        <div className="font-semibold text-slate-900">{pub.name}</div>
                        <div className="text-xs text-slate-500 font-normal">{pub.email}</div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-slate-100 rounded-lg border border-slate-200">
                          <code className="text-xs font-mono font-bold text-slate-800">{pub.referral_token}</code>
                          <button
                            onClick={() => copyToClipboard(pub.referral_token, 'Referral code')}
                            className="text-slate-400 hover:text-slate-700 p-0.5 rounded transition-colors"
                            title="Copy code"
                          >
                            <AppIcon name="Copy" className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <button
                          onClick={() => copyToClipboard(refUrl, 'Referral link')}
                          className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-blue-600 hover:text-blue-800 bg-blue-50/70 hover:bg-blue-100/70 rounded-lg border border-blue-100 transition-colors"
                        >
                          <AppIcon name="Link" className="h-3.5 w-3.5" />
                          <span>Copy Link</span>
                        </button>
                      </td>
                      <td className="px-6 py-4 text-center">
                        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-bold bg-blue-50 text-blue-700 border border-blue-100">
                          {pub.referred_schools_count || 0}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-bold border ${
                            pub.status === 'active'
                              ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                              : 'bg-rose-50 text-rose-700 border-rose-200'
                          }`}
                        >
                          {pub.status.toUpperCase()}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <Link
                            to={`/publishers/${pub.id}`}
                            className="px-3 py-1.5 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg transition-colors"
                          >
                            View Schools →
                          </Link>
                          <button
                            disabled={isActionBusy}
                            onClick={() => handleToggleStatus(pub)}
                            className={`px-2.5 py-1.5 text-xs font-semibold rounded-lg transition-colors border ${
                              pub.status === 'active'
                                ? 'text-amber-700 border-amber-200 bg-amber-50 hover:bg-amber-100'
                                : 'text-emerald-700 border-emerald-200 bg-emerald-50 hover:bg-emerald-100'
                            }`}
                          >
                            {pub.status === 'active' ? 'Suspend' : 'Reactivate'}
                          </button>
                          <button
                            disabled={isActionBusy}
                            onClick={() => handleDelete(pub)}
                            className="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors"
                            title="Delete publisher"
                          >
                            <AppIcon name="Trash2" className="h-4 w-4" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Create Publisher Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-white rounded-3xl shadow-2xl w-full max-w-md overflow-hidden border border-slate-100">
            {createdResult ? (
              // Success / Copy Credentials View
              <div className="p-7">
                <div className="w-12 h-12 rounded-2xl bg-emerald-100 text-emerald-600 flex items-center justify-center mx-auto mb-4">
                  <AppIcon name="CheckCircle" className="h-6 w-6" />
                </div>
                <h2 className="text-xl font-bold text-center text-slate-900 mb-1">
                  Publisher Created!
                </h2>
                <p className="text-xs text-center text-slate-500 mb-6">
                  Partner account is ready. Copy the credentials and referral link below to share with them.
                </p>

                <div className="space-y-3 bg-slate-50 p-4 rounded-2xl border border-slate-200 text-xs mb-6">
                  <div>
                    <span className="text-slate-400 font-medium">Partner Name:</span>
                    <div className="font-semibold text-slate-800 text-sm mt-0.5">{createdResult.name}</div>
                  </div>
                  <div>
                    <span className="text-slate-400 font-medium">Login Email:</span>
                    <div className="font-semibold text-slate-800 text-sm mt-0.5">{createdResult.email}</div>
                  </div>
                  <div>
                    <span className="text-slate-400 font-medium">Password:</span>
                    <div className="font-mono font-bold text-slate-800 text-sm mt-0.5 bg-white px-2 py-1 rounded border border-slate-200 inline-block">
                      {createdResult.password}
                    </div>
                  </div>
                  <div>
                    <span className="text-slate-400 font-medium">Referral Code:</span>
                    <div className="font-mono font-bold text-blue-700 text-sm mt-0.5">{createdResult.referral_token}</div>
                  </div>
                  <div>
                    <span className="text-slate-400 font-medium">Referral Link:</span>
                    <div className="font-mono text-slate-600 text-[11px] mt-0.5 break-all bg-white p-2 rounded border border-slate-200">
                      {createdResult.referral_url}
                    </div>
                  </div>
                </div>

                <div className="flex flex-col gap-2">
                  <button
                    onClick={() => {
                      const text = `Eduplexo Publisher Partner Access:\n\nPortal: https://publisher.eduplexo.com\nEmail: ${createdResult.email}\nPassword: ${createdResult.password}\n\nYour Referral Link:\n${createdResult.referral_url}\nReferral Code: ${createdResult.referral_token}`
                      copyToClipboard(text, 'Credentials & link')
                    }}
                    className="w-full py-2.5 px-4 bg-blue-600 hover:bg-blue-700 text-white rounded-xl text-sm font-semibold transition-all flex items-center justify-center gap-2 shadow-sm"
                  >
                    <AppIcon name="Copy" className="h-4 w-4" />
                    Copy All Credentials & Link
                  </button>
                  <button
                    onClick={() => setShowCreateModal(false)}
                    className="w-full py-2.5 px-4 bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-xl text-sm font-semibold transition-colors"
                  >
                    Done
                  </button>
                </div>
              </div>
            ) : (
              // Form View
              <div className="p-7">
                <div className="flex justify-between items-center mb-5">
                  <div>
                    <h2 className="text-lg font-bold text-slate-900">Add Publisher Partner</h2>
                    <p className="text-xs text-slate-500">Create login credentials and generate a unique referral link.</p>
                  </div>
                  <button
                    onClick={() => setShowCreateModal(false)}
                    className="p-1 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
                  >
                    <AppIcon name="X" className="h-5 w-5" />
                  </button>
                </div>

                <form onSubmit={handleCreate} className="space-y-4">
                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                      Partner / Agency Name
                    </label>
                    <input
                      type="text"
                      required
                      value={name}
                      onChange={e => setName(e.target.value)}
                      placeholder="e.g. EduGrowth Media"
                      className="w-full px-3.5 py-2.5 border border-slate-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                      Login Email Address
                    </label>
                    <input
                      type="email"
                      required
                      value={email}
                      onChange={e => setEmail(e.target.value)}
                      placeholder="partner@example.com"
                      className="w-full px-3.5 py-2.5 border border-slate-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all"
                    />
                  </div>

                  <div>
                    <div className="flex justify-between items-center mb-1.5">
                      <label className="text-xs font-semibold text-slate-700">
                        Password
                      </label>
                      <button
                        type="button"
                        onClick={generateRandomPassword}
                        className="text-xs font-semibold text-blue-600 hover:text-blue-800"
                      >
                        Auto-generate
                      </button>
                    </div>
                    <div className="relative">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        required
                        minLength={8}
                        value={password}
                        onChange={e => setPassword(e.target.value)}
                        placeholder="At least 8 characters"
                        className="w-full pl-3.5 pr-10 py-2.5 border border-slate-200 rounded-xl text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all"
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
                      >
                        <AppIcon name={showPassword ? 'EyeOff' : 'Eye'} className="h-4 w-4" />
                      </button>
                    </div>
                  </div>

                  <div className="flex gap-3 pt-3">
                    <button
                      type="button"
                      onClick={() => setShowCreateModal(false)}
                      className="flex-1 py-2.5 px-4 text-sm font-semibold text-slate-600 hover:bg-slate-100 rounded-xl transition-colors"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={isSubmitting}
                      className="flex-1 py-2.5 px-4 text-sm font-semibold text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-xl transition-all shadow-sm"
                    >
                      {isSubmitting ? 'Creating...' : 'Create Partner'}
                    </button>
                  </div>
                </form>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
