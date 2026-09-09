import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
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
  updated_at: string
}

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

export function PublisherDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [publisher, setPublisher] = useState<Publisher | null>(null)
  const [schools, setSchools] = useState<ReferredSchool[]>([])
  const [loading, setLoading] = useState(true)

  // Reset password modal state
  const [showResetPasswordModal, setShowResetPasswordModal] = useState(false)
  const [newPassword, setNewPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [isResetting, setIsResetting] = useState(false)

  // Action busy state
  const [actionLoading, setActionLoading] = useState(false)

  const loadData = async () => {
    if (!id) return
    setLoading(true)
    const res = await apiRequest<{ publisher: Publisher; schools: ReferredSchool[] }>(
      `/api/super-admin/publishers/${id}`
    )
    if (res.ok && res.data) {
      setPublisher(res.data.publisher)
      setSchools(res.data.schools || [])
    } else {
      showToast(res.message || 'Failed to load publisher details', 'error')
    }
    setLoading(false)
  }

  useEffect(() => {
    loadData()
  }, [id])

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text)
    showToast(`${label} copied to clipboard!`, 'success')
  }

  const handleToggleStatus = async () => {
    if (!publisher) return
    const isSuspending = publisher.status === 'active'
    const endpoint = isSuspending
      ? `/api/super-admin/publishers/${publisher.id}/suspend`
      : `/api/super-admin/publishers/${publisher.id}/reactivate`

    setActionLoading(true)
    const res = await apiRequest(endpoint, { method: 'POST' })
    setActionLoading(false)

    if (res.ok) {
      showToast(`Publisher ${isSuspending ? 'suspended' : 'reactivated'} successfully`, 'success')
      setPublisher(prev => (prev ? { ...prev, status: isSuspending ? 'suspended' : 'active' } : null))
    } else {
      showToast(res.message || 'Action failed', 'error')
    }
  }

  const handleDelete = async () => {
    if (!publisher) return
    if (
      !window.confirm(
        `Are you sure you want to delete publisher "${publisher.name}"? Referred schools will remain intact.`
      )
    ) {
      return
    }

    setActionLoading(true)
    const res = await apiRequest(`/api/super-admin/publishers/${publisher.id}`, { method: 'DELETE' })
    setActionLoading(false)

    if (res.ok) {
      showToast('Publisher deleted successfully', 'success')
      navigate('/publishers', { replace: true })
    } else {
      showToast(res.message || 'Failed to delete publisher', 'error')
    }
  }

  const generateRandomPassword = () => {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%^&*'
    let pwd = ''
    for (let i = 0; i < 12; i++) {
      pwd += chars.charAt(Math.floor(Math.random() * chars.length))
    }
    setNewPassword(pwd)
  }

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!publisher || !newPassword) return

    if (newPassword.length < 8) {
      showToast('Password must be at least 8 characters', 'error')
      return
    }

    setIsResetting(true)
    const res = await apiRequest(`/api/super-admin/publishers/${publisher.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ password: newPassword }),
    })
    setIsResetting(false)

    if (res.ok) {
      showToast('Password updated successfully', 'success')
      setShowResetPasswordModal(false)
    } else {
      showToast(res.message || 'Failed to update password', 'error')
    }
  }

  if (loading) {
    return (
      <div className="p-8 max-w-7xl mx-auto min-h-screen flex flex-col items-center justify-center gap-3 text-slate-400">
        <div className="h-7 w-7 border-2 border-blue-200 border-t-blue-600 rounded-full animate-spin" />
        <span className="text-sm font-medium">Loading publisher details...</span>
      </div>
    )
  }

  if (!publisher) {
    return (
      <div className="p-8 max-w-7xl mx-auto min-h-screen text-center py-24">
        <div className="bg-slate-50 w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-slate-100">
          <AppIcon name="AlertCircle" className="h-8 w-8 text-slate-400" />
        </div>
        <h2 className="text-xl font-bold text-slate-900 mb-2">Publisher Not Found</h2>
        <p className="text-slate-500 text-sm mb-6">The requested publisher could not be loaded or was removed.</p>
        <Link
          to="/publishers"
          className="px-4 py-2 bg-blue-600 text-white rounded-xl text-sm font-semibold hover:bg-blue-700 transition-colors"
        >
          Back to Publishers
        </Link>
      </div>
    )
  }

  const refUrl = publisher.referral_url || `https://app.eduplexo.com/auth/register?ref=${publisher.referral_token}`

  return (
    <div className="p-8 max-w-7xl mx-auto min-h-screen">
      {/* Back Link */}
      <div className="mb-6">
        <Link
          to="/publishers"
          className="text-sm font-medium text-slate-500 hover:text-slate-900 inline-flex items-center gap-1.5 transition-colors"
        >
          <AppIcon name="ArrowLeft" className="h-4 w-4" />
          Back to Publishers
        </Link>
      </div>

      {/* Main Partner Header Card */}
      <div className="bg-white rounded-3xl border border-slate-200 shadow-sm p-6 sm:p-8 mb-8">
        <div className="flex flex-col lg:flex-row justify-between items-start lg:items-center gap-6">
          <div>
            <div className="flex items-center gap-3 mb-2 flex-wrap">
              <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-slate-900">
                {publisher.name}
              </h1>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-bold border ${
                  publisher.status === 'active'
                    ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                    : 'bg-rose-50 text-rose-700 border-rose-200'
                }`}
              >
                {publisher.status.toUpperCase()}
              </span>
            </div>
            <div className="flex items-center gap-4 text-sm text-slate-500 flex-wrap">
              <span>
                <strong className="text-slate-700 font-medium">Email:</strong> {publisher.email}
              </span>
              <span>•</span>
              <span>
                <strong className="text-slate-700 font-medium">Partner ID:</strong>{' '}
                <code className="text-xs font-mono text-slate-600">{publisher.id}</code>
              </span>
              <span>•</span>
              <span>
                <strong className="text-slate-700 font-medium">Joined:</strong>{' '}
                {new Date(publisher.created_at).toLocaleDateString()}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-2.5 flex-wrap">
            <button
              onClick={() => {
                generateRandomPassword()
                setShowPassword(false)
                setShowResetPasswordModal(true)
              }}
              className="px-3.5 py-2 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-xl transition-colors inline-flex items-center gap-1.5"
            >
              <AppIcon name="Key" className="h-3.5 w-3.5" />
              Reset Password
            </button>
            <button
              disabled={actionLoading}
              onClick={handleToggleStatus}
              className={`px-3.5 py-2 text-xs font-semibold rounded-xl transition-colors border ${
                publisher.status === 'active'
                  ? 'text-amber-700 border-amber-200 bg-amber-50 hover:bg-amber-100'
                  : 'text-emerald-700 border-emerald-200 bg-emerald-50 hover:bg-emerald-100'
              }`}
            >
              {publisher.status === 'active' ? 'Suspend Partner' : 'Reactivate Partner'}
            </button>
            <button
              disabled={actionLoading}
              onClick={handleDelete}
              className="p-2 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded-xl transition-colors"
              title="Delete partner"
            >
              <AppIcon name="Trash2" className="h-4 w-4" />
            </button>
          </div>
        </div>

        {/* Hero Referral Link Section */}
        <div className="mt-6 pt-6 border-t border-slate-100">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
            Referral Tracking Link
          </div>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
            <div className="flex-1 bg-slate-50 border border-slate-200 rounded-xl px-3.5 py-2.5 flex items-center gap-2 overflow-hidden">
              <AppIcon name="Link2" className="h-4 w-4 text-blue-600 flex-shrink-0" />
              <input
                type="text"
                readOnly
                value={refUrl}
                className="w-full bg-transparent text-xs font-mono text-slate-700 focus:outline-none select-all"
              />
            </div>
            <button
              onClick={() => copyToClipboard(refUrl, 'Referral link')}
              className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2.5 rounded-xl text-xs font-semibold transition-all inline-flex items-center justify-center gap-1.5 shadow-sm active:scale-95"
            >
              <AppIcon name="Copy" className="h-3.5 w-3.5" />
              Copy Link
            </button>
            <a
              href={refUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="bg-slate-100 hover:bg-slate-200 text-slate-700 px-4 py-2.5 rounded-xl text-xs font-semibold transition-colors inline-flex items-center justify-center gap-1.5"
            >
              <AppIcon name="ExternalLink" className="h-3.5 w-3.5" />
              Test Link
            </a>
          </div>
        </div>
      </div>

      {/* Metrics Row */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
            Referred Schools Created
          </div>
          <div className="text-3xl font-extrabold text-blue-600">{schools.length}</div>
        </div>
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
            Referral Token
          </div>
          <div className="text-xl font-mono font-bold text-slate-800">{publisher.referral_token}</div>
        </div>
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
            Publisher Portal Login
          </div>
          <div className="text-sm font-semibold text-slate-700">publisher.eduplexo.com</div>
        </div>
      </div>

      {/* Referred Schools Section */}
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-slate-900">Schools Referred by this Partner</h2>
          <p className="text-slate-500 text-xs mt-0.5">
            Real-time audit list of schools registered through referral token {publisher.referral_token}.
          </p>
        </div>
        <span className="text-xs font-semibold bg-slate-100 text-slate-700 px-3 py-1 rounded-full border border-slate-200">
          {schools.length} {schools.length === 1 ? 'School' : 'Schools'}
        </span>
      </div>

      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        {schools.length === 0 ? (
          <div className="p-16 text-center">
            <div className="bg-slate-50 w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-slate-100">
              <AppIcon name="Building2" className="h-8 w-8 text-slate-400" />
            </div>
            <h3 className="text-slate-900 font-bold text-base mb-1">No Schools Registered Yet</h3>
            <p className="text-slate-500 text-sm max-w-md mx-auto">
              When a new school creates an account using this partner's referral link, it will automatically appear here.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-slate-50 text-slate-500 text-[11px] uppercase tracking-wider font-semibold border-b border-slate-200">
                <tr>
                  <th className="px-6 py-4">School Name & Code</th>
                  <th className="px-6 py-4">Contact Email</th>
                  <th className="px-6 py-4">Contact Phone</th>
                  <th className="px-6 py-4">Registration Date</th>
                  <th className="px-6 py-4">School Status</th>
                  <th className="px-6 py-4 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {schools.map(school => (
                  <tr key={school.id || school.school_id} className="hover:bg-slate-50/70 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-semibold text-slate-900">{school.name}</div>
                      <div className="text-xs font-mono text-slate-500">Code: {school.code || '—'}</div>
                    </td>
                    <td className="px-6 py-4 text-slate-600 text-xs">
                      {school.contact_email || '—'}
                    </td>
                    <td className="px-6 py-4 text-slate-600 text-xs">
                      {school.contact_phone || '—'}
                    </td>
                    <td className="px-6 py-4 text-slate-500 text-xs">
                      {new Date(school.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4">
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
                    <td className="px-6 py-4 text-right">
                      <Link
                        to={`/schools/${school.school_id || school.id}`}
                        className="text-blue-600 hover:text-blue-800 text-xs font-semibold"
                      >
                        Manage School →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Reset Password Modal */}
      {showResetPasswordModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-white rounded-3xl shadow-2xl w-full max-w-md overflow-hidden border border-slate-100 p-7">
            <div className="flex justify-between items-center mb-5">
              <div>
                <h2 className="text-lg font-bold text-slate-900">Reset Partner Password</h2>
                <p className="text-xs text-slate-500">Set a new login password for {publisher.name}.</p>
              </div>
              <button
                onClick={() => setShowResetPasswordModal(false)}
                className="p-1 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
              >
                <AppIcon name="X" className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleResetPassword} className="space-y-4">
              <div>
                <div className="flex justify-between items-center mb-1.5">
                  <label className="text-xs font-semibold text-slate-700">New Password</label>
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
                    value={newPassword}
                    onChange={e => setNewPassword(e.target.value)}
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
                  onClick={() => setShowResetPasswordModal(false)}
                  className="flex-1 py-2.5 px-4 text-sm font-semibold text-slate-600 hover:bg-slate-100 rounded-xl transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isResetting || !newPassword}
                  className="flex-1 py-2.5 px-4 text-sm font-semibold text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-xl transition-all shadow-sm"
                >
                  {isResetting ? 'Saving...' : 'Update Password'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
