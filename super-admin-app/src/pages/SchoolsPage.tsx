import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiRequest } from '@/lib/api'
import { showToast } from '@/utils/toast'

interface School {
  _id: string
  school_id: string
  name: string
  code: string
  status: string
  owner_email: string
  student_count: number
  teacher_count: number
  class_count: number
  plan: string
  revenue: number
  expiry: string
  created_at: string
  subscription_status?: string
  is_trial?: boolean
  days_remaining?: number
  grace_ends_at?: string
}

function formatCurrency(amount: number): string {
  return `Rs ${amount.toLocaleString()}`
}

export function SchoolsPage() {
  const [schools, setSchools] = useState<School[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState('all')
  const [search, setSearch] = useState('')
  const [actionModal, setActionModal] = useState<{ schoolId: string; name: string; type: 'suspend' | 'delete' } | null>(null)
  const [isProcessing, setIsProcessing] = useState(false)

  const loadSchools = async () => {
    setLoading(true)
    const params = new URLSearchParams()
    if (statusFilter !== 'all') params.set('status', statusFilter)
    if (search) params.set('search', search)
    const res = await apiRequest(`/api/super-admin/schools?${params}`)
    if (res.ok && res.data) {
      const items = Array.isArray(res.data) 
        ? res.data 
        : (res.data as any).items || (res.data as any).data || []
      setSchools(items)
    }
    setLoading(false)
  }

  useEffect(() => { loadSchools() }, [statusFilter])

  const handleAction = async (schoolId: string, action: 'approve' | 'suspend' | 'renew' | 'reactivate') => {
    const endpoint = action === 'renew' ? 'approve' : action
    setIsProcessing(true)
    const res = await apiRequest(`/api/super-admin/schools/${schoolId}/${endpoint}`, {
      method: 'POST',
      body: JSON.stringify({ reason: action === 'suspend' ? 'Super Admin manual suspension' : 'Super Admin manual action' }),
    })
    setIsProcessing(false)
    if (res.ok) {
      showToast(`School ${action === 'suspend' ? 'suspended' : action === 'reactivate' ? 'reactivated' : action} successfully.`, 'success')
      setActionModal(null)
      loadSchools()
    } else {
      showToast(res.message || `Failed to ${action} school.`, 'error')
    }
  }

  const handleDelete = async (schoolId: string) => {
    setIsProcessing(true)
    const res = await apiRequest(`/api/super-admin/schools/${schoolId}`, { method: 'DELETE' })
    setIsProcessing(false)
    if (res.ok) {
      showToast('School deleted successfully.', 'success')
      setActionModal(null)
      loadSchools()
    } else {
      showToast(res.message || 'Failed to delete school.', 'error')
    }
  }

  const statusBadge = (status: string) => {
    const map: Record<string, string> = {
      active: 'bg-blue-50 text-blue-700 border-blue-100',
      suspended: 'bg-red-50 text-red-700 border-red-100',
      pending: 'bg-amber-50 text-amber-700 border-amber-100',
      expired: 'bg-slate-50 text-slate-600 border-slate-100',
    }
    return `text-[9px] font-bold px-2 py-0.5 rounded-full border ${map[status] || map.expired}`
  }

  const planBadge = (plan: string) => {
    const isFree = !plan || plan === 'Free'
    return `text-[9px] font-bold px-2 py-0.5 rounded-full ${isFree ? 'bg-slate-50 text-slate-500' : 'bg-blue-50 text-blue-700'}`
  }

  const initials = (name: string) => name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Registered Schools & Institutions</h1>
          <p className="text-xs text-slate-500 mt-0.5">{schools.length} registered schools & subscriber accounts</p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 bg-white p-3 rounded-xl border border-slate-200">
        <div className="relative flex-1 max-w-sm">
          <AppIcon name="Search" size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && loadSchools()}
            placeholder="Search schools..."
            className="w-full h-8 pl-9 pr-3 rounded-lg border border-slate-200 text-[12px] outline-none focus:border-blue-500"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="h-8 px-3 rounded-lg border border-slate-200 text-[12px] font-semibold text-slate-600 outline-none"
        >
          <option value="all">All Status</option>
          <option value="active">Active</option>
          <option value="pending">Pending</option>
          <option value="suspended">Suspended</option>
          <option value="expired">Expired</option>
        </select>
        <button onClick={loadSchools} className="h-8 px-3 rounded-lg border border-slate-200 text-[10px] font-bold text-slate-600 hover:bg-slate-50 transition-colors">
          Refresh
        </button>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-slate-200 overflow-hidden shadow-sm">
        {loading ? (
          <div className="p-12 text-center text-sm text-slate-400">Loading registered schools...</div>
        ) : schools.length === 0 ? (
          <div className="p-16 text-center">
            <AppIcon name="Users" size={36} className="text-slate-200 mb-3" />
            <p className="text-sm font-medium text-slate-500">No registered schools found</p>
          </div>
        ) : (
          <table className="w-full">
            <thead className="bg-slate-50 border-b border-slate-100">
              <tr>
                <th className="text-left px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">School & Administrator</th>
                <th className="text-left px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Plan</th>
                <th className="text-left px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">License / Trial Remaining</th>
                <th className="text-center px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Status</th>
                <th className="text-right px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Revenue</th>
                <th className="text-left px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Joined Date</th>
                <th className="text-right px-4 py-3 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-50">
              {schools.map((school) => {
                // Prefer the backend-computed days_remaining; fall back to expiry math.
                const days =
                  typeof school.days_remaining === 'number'
                    ? school.days_remaining
                    : Math.ceil(((school.expiry ? new Date(school.expiry).getTime() : 0) - Date.now()) / (1000 * 60 * 60 * 24))
                const isPaidPlan = school.plan && !school.plan.toLowerCase().includes('trial') && school.plan !== 'Free' && school.plan !== 'Free Trial'
                const subStatus = school.subscription_status || ''
                const isSuspended = subStatus === 'suspended'
                const isTrial = Boolean(school.is_trial) || (school.plan || '').toLowerCase().includes('trial')
                return (
                  <tr key={school._id} className="hover:bg-blue-50/30 transition-colors">
                    <td className="px-4 py-3">
                      <Link to={`/schools/${school._id}`} className="flex items-center gap-3 group">
                        <div className="h-9 w-9 rounded-xl bg-blue-100 text-blue-700 flex items-center justify-center text-[12px] font-black shrink-0 shadow-sm">
                          {initials(school.name)}
                        </div>
                        <div>
                          <p className="text-[13px] font-bold text-slate-900 group-hover:text-blue-600 transition-colors">
                            {school.name}
                          </p>
                          <p className="text-[11px] text-slate-500 font-medium">
                            {school.owner_email || school.code}
                          </p>
                        </div>
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`text-[10px] font-extrabold px-2.5 py-1 rounded-full border ${
                        isPaidPlan
                          ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                          : 'bg-indigo-50 text-indigo-700 border-indigo-200'
                      }`}>
                        {school.plan || 'Free Trial'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      {isSuspended ? (
                        <span className="text-[10px] font-bold text-rose-700 bg-rose-100 px-2 py-0.5 rounded border border-rose-300">
                          Suspended
                        </span>
                      ) : school.expiry ? (
                        <div>
                          {days <= 0 ? (
                            <span className="text-[10px] font-bold text-rose-600 bg-rose-50 px-2 py-0.5 rounded border border-rose-200">
                              Expired
                            </span>
                          ) : isPaidPlan ? (
                            <span className="text-[10px] font-bold text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
                              {days} days remaining
                            </span>
                          ) : (
                            <span className="text-[10px] font-bold text-indigo-700 bg-indigo-50 px-2 py-0.5 rounded border border-indigo-200">
                              {days} {days === 1 ? 'day' : 'days'} trial left
                            </span>
                          )}
                          <p className="text-[10px] text-slate-400 mt-0.5">
                            {subStatus === 'expired' && school.grace_ends_at
                              ? `Suspends ${new Date(school.grace_ends_at).toLocaleDateString()}`
                              : `Expires ${new Date(school.expiry).toLocaleDateString()}`}
                          </p>
                        </div>
                      ) : (
                        <span className="text-[10px] text-slate-400">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-center">
                      <span className={statusBadge(school.status)}>{school.status}</span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <span className="text-[12px] font-black text-slate-900">{formatCurrency(school.revenue || 0)}</span>
                    </td>
                    <td className="px-4 py-3 text-slate-500 text-[11px] font-medium">
                      {new Date(school.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        <Link to={`/schools/${school._id}`} className="h-7 px-3 text-[11px] font-bold text-blue-700 bg-blue-50 border border-blue-100 rounded-xl hover:bg-blue-100 transition-colors flex items-center justify-center shadow-sm">
                          View
                        </Link>
                        {school.status === 'pending' && (
                          <button onClick={() => handleAction(school._id, 'approve')} className="h-7 px-2.5 text-[10px] font-bold text-emerald-700 bg-emerald-50 border border-emerald-100 rounded-xl hover:bg-emerald-100 transition-colors">
                            Approve
                          </button>
                        )}
                        {school.status === 'active' && (
                          <button
                            onClick={() => setActionModal({ schoolId: school._id, name: school.name, type: 'suspend' })}
                            className="h-7 px-2.5 text-[10px] font-bold text-red-700 bg-red-50 border border-red-100 rounded-xl hover:bg-red-100 transition-colors"
                          >
                            Suspend
                          </button>
                        )}
                        {school.status === 'suspended' && (
                          <button
                            onClick={() => handleAction(school._id, 'reactivate')}
                            disabled={isProcessing}
                            className="h-7 px-2.5 text-[10px] font-bold text-emerald-700 bg-emerald-50 border border-emerald-100 rounded-xl hover:bg-emerald-100 transition-colors"
                          >
                            Reactivate
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Confirmation Modal */}
      {actionModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div className="bg-white rounded-2xl border border-slate-200 shadow-2xl p-6 max-w-md w-full space-y-4 animate-fade-in-up">
            <div className="flex items-center gap-3">
              <div className={`h-10 w-10 rounded-xl flex items-center justify-center ${
                actionModal.type === 'suspend' ? 'bg-amber-100 text-amber-700' : 'bg-red-100 text-red-700'
              }`}>
                <AppIcon name={actionModal.type === 'suspend' ? 'AlertTriangle' : 'Trash2'} size={20} />
              </div>
              <div>
                <h3 className="text-base font-bold text-slate-900">
                  {actionModal.type === 'suspend' ? 'Suspend Institution' : 'Delete Institution'}
                </h3>
                <p className="text-xs text-slate-500">{actionModal.name}</p>
              </div>
            </div>

            <p className="text-xs text-slate-600 leading-relaxed">
              {actionModal.type === 'suspend'
                ? 'Suspending this institution will immediately block access for all teachers, admins, and students. The owner will be restricted to the billing renewal portal until reactivated.'
                : 'Are you sure you want to permanently delete this institution? All data across campuses and students will be permanently removed. This cannot be undone.'}
            </p>

            <div className="flex items-center justify-end gap-2.5 pt-2">
              <button
                type="button"
                onClick={() => setActionModal(null)}
                disabled={isProcessing}
                className="px-4 py-2 text-xs font-bold text-slate-600 bg-slate-100 hover:bg-slate-200 rounded-xl transition"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={isProcessing}
                onClick={() => {
                  if (actionModal.type === 'suspend') {
                    handleAction(actionModal.schoolId, 'suspend')
                  } else {
                    handleDelete(actionModal.schoolId)
                  }
                }}
                className={`px-4 py-2 text-xs font-bold text-white rounded-xl shadow-sm transition ${
                  actionModal.type === 'suspend'
                    ? 'bg-amber-600 hover:bg-amber-700'
                    : 'bg-red-600 hover:bg-red-700'
                }`}
              >
                {isProcessing ? 'Processing…' : actionModal.type === 'suspend' ? 'Confirm Suspension' : 'Permanently Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
