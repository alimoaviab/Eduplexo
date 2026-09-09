import { AppIcon } from "shared/ui/AppIcon";
import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { apiRequest } from '@/lib/api'

interface DashboardData {
  schools: {
    total: number
    active: number
    pending: number
    suspended: number
    expired: number
    trial: number
    paid: number
    new_this_month: number
    new_last_month: number
    growth_rate: number
  }
  revenue: {
    total: number
    monthly: number
    mrr: number
    arr: number
    collected: number
    pending: number
    collection_rate: number
    renewals_due: number
  }
  subscriptions: {
    active: number
    expired: number
    churn_rate: number
  }
  platform: {
    total_users: number
    admin_users: number
    total_expenses: number
    net_revenue: number
    expense_breakdown: Record<string, number>
  }
  monthly_growth: { month: string; schools: number; revenue: number }[]
  plan_distribution: Record<string, number>
  recent_schools: { _id: string; name: string; plan: string; status: string; revenue: number; expiry: string; created_at: string }[]
  recent_payments: { school: string; amount: number; plan: string; status: string; date: string }[]
  activities: { type: string; message: string; timestamp: string }[]
}

function formatCurrency(amount: number): string {
  if (amount >= 1000000) return `Rs ${(amount / 1000000).toFixed(1)}M`
  if (amount >= 1000) return `Rs ${(amount / 1000).toFixed(1)}K`
  return `Rs ${amount.toLocaleString()}`
}

export function DashboardPage() {
  const navigate = useNavigate()
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [pendingPaymentsCount, setPendingPaymentsCount] = useState(0)
  const [pendingPaymentsList, setPendingPaymentsList] = useState<any[]>([])

  useEffect(() => {
    loadDashboard()
    loadPendingPayments()
  }, [])

  async function loadDashboard() {
    setLoading(true)
    const result = await apiRequest<DashboardData>('/api/super-admin/dashboard')
    if (result.ok && result.data) {
      setData(result.data)
    }
    setLoading(false)
  }

  async function loadPendingPayments() {
    const res = await apiRequest('/api/admin/payments/pending')
    if (res.ok && res.data) {
      const items = Array.isArray(res.data) 
        ? res.data 
        : (res.data as any).items || (res.data as any).data || []
      setPendingPaymentsCount(items.length)
      setPendingPaymentsList(items.slice(0, 5))
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-slate-200 border-t-blue-600" />
      </div>
    )
  }

  if (!data) return null

  const { schools, revenue, subscriptions, platform, recent_schools } = data

  return (
    <div className="space-y-6">
      {/* ── Page Header ─────────────────────────────────────────────────── */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-slate-900">Platform Overview</h1>
          <p className="text-xs text-slate-500 mt-1">Executive overview of institutional subscribers, licenses, and revenue.</p>
        </div>
        <div className="flex items-center gap-2.5">
          <button
            onClick={() => {
              loadDashboard()
              loadPendingPayments()
            }}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-slate-200 bg-white text-xs font-bold text-slate-700 hover:bg-slate-50 shadow-sm transition-all"
          >
            <AppIcon name="RefreshCw" size={13} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {/* ── 4 Executive SaaS KPI Cards (De-cluttered & High-Impact) ───────── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* 1. MRR */}
        <div className="bg-white rounded-2xl border border-slate-200/90 p-5 shadow-sm relative overflow-hidden">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-bold text-slate-500 uppercase tracking-wider">Monthly Recurring Revenue</span>
            <div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center border border-emerald-100">
              <AppIcon name="TrendingUp" size={16} />
            </div>
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-black text-slate-900">{formatCurrency(revenue.mrr)}</span>
            <span className="text-[11px] font-bold text-emerald-600">ARR {formatCurrency(revenue.arr)}</span>
          </div>
          <p className="text-[11px] text-slate-400 mt-2">Active recurring SaaS subscriptions</p>
        </div>

        {/* 2. Total Campuses */}
        <div className="bg-white rounded-2xl border border-slate-200/90 p-5 shadow-sm relative overflow-hidden">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-bold text-slate-500 uppercase tracking-wider">Institutions / Campuses</span>
            <div className="w-8 h-8 rounded-lg bg-blue-50 text-blue-600 flex items-center justify-center border border-blue-100">
              <AppIcon name="Building2" size={16} />
            </div>
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-black text-slate-900">{schools.total}</span>
            <span className="text-[11px] font-bold text-blue-600">{schools.active} Active</span>
          </div>
          <p className="text-[11px] text-slate-400 mt-2">
            {platform.total_users} users registered across all institutions
          </p>
        </div>

        {/* 3. Pending Payments (Clickable Callout) */}
        <Link
          to="/payments"
          className={`rounded-2xl border p-5 shadow-sm transition-all block relative overflow-hidden ${
            pendingPaymentsCount > 0
              ? "bg-amber-50/50 border-amber-300 hover:border-amber-400 hover:shadow-md"
              : "bg-white border-slate-200/90 hover:border-slate-300"
          }`}
        >
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-bold text-slate-700 uppercase tracking-wider">Payment Verification</span>
            <div className={`w-8 h-8 rounded-lg flex items-center justify-center border ${
              pendingPaymentsCount > 0
                ? "bg-amber-100 text-amber-700 border-amber-200 animate-pulse"
                : "bg-slate-100 text-slate-600 border-slate-200"
            }`}>
              <AppIcon name="CreditCard" size={16} />
            </div>
          </div>
          <div className="flex items-baseline gap-2">
            <span className={`text-2xl font-black ${pendingPaymentsCount > 0 ? "text-amber-700" : "text-slate-900"}`}>
              {pendingPaymentsCount}
            </span>
            {pendingPaymentsCount > 0 ? (
              <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-amber-200/60 text-amber-800">
                Action Required
              </span>
            ) : (
              <span className="text-[11px] font-semibold text-slate-400">All settled</span>
            )}
          </div>
          <p className="text-[11px] text-slate-500 mt-2 flex items-center gap-1 font-medium">
            <span>Review payment proofs</span>
            <AppIcon name="ChevronRight" size={12} />
          </p>
        </Link>

        {/* 4. Active Free Trials */}
        <div className="bg-white rounded-2xl border border-slate-200/90 p-5 shadow-sm relative overflow-hidden">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-bold text-slate-500 uppercase tracking-wider">Free Trials</span>
            <div className="w-8 h-8 rounded-lg bg-indigo-50 text-indigo-600 flex items-center justify-center border border-indigo-100">
              <AppIcon name="Award" size={16} />
            </div>
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-black text-slate-900">{schools.trial || subscriptions.active}</span>
            <span className="text-[11px] font-bold text-indigo-600">Free Trial Accounts</span>
          </div>
          <p className="text-[11px] text-slate-400 mt-2">
            {subscriptions.active} licenses actively running
          </p>
        </div>
      </div>

      {/* ── Pending Payments Action Banner (if any) ─────────────────────────── */}
      {pendingPaymentsCount > 0 && (
        <div className="bg-gradient-to-r from-amber-500/10 via-amber-500/5 to-transparent border border-amber-300 rounded-2xl p-5 shadow-sm flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-start gap-3.5">
            <div className="w-10 h-10 rounded-xl bg-amber-500 text-white flex items-center justify-center shrink-0 shadow-md shadow-amber-500/30">
              <AppIcon name="AlertCircle" size={20} />
            </div>
            <div>
              <h3 className="text-sm font-bold text-slate-900">
                {pendingPaymentsCount} Payment Proof{pendingPaymentsCount > 1 ? "s" : ""} Awaiting Verification
              </h3>
              <p className="text-xs text-slate-600 mt-0.5">
                School owners have uploaded bank deposit slips and mobile wallet screenshots. Review proofs to activate their subscription plans.
              </p>
            </div>
          </div>
          <Link
            to="/payments"
            className="px-4 py-2 bg-amber-500 hover:bg-amber-600 text-white font-bold text-xs rounded-xl transition-all shadow-md shadow-amber-500/20 shrink-0 inline-flex items-center gap-1.5 self-start md:self-auto"
          >
            <span>Open Payments Queue</span>
            <AppIcon name="ArrowRight" size={13} />
          </Link>
        </div>
      )}

      {/* ── 2-Column Section: Institutional Activity & Quick Tools ─────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Recent Schools (2 Cols) */}
        <div className="lg:col-span-2 bg-white rounded-2xl border border-slate-200/90 shadow-sm overflow-hidden flex flex-col">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
            <div>
              <h3 className="text-sm font-bold text-slate-900">Registered Institutions</h3>
              <p className="text-[11px] text-slate-400">Schools and campuses on the EduPlexo network</p>
            </div>
            <Link
              to="/schools"
              className="text-xs font-bold text-blue-600 hover:text-blue-700 flex items-center gap-1"
            >
              <span>View All</span>
              <AppIcon name="ChevronRight" size={12} />
            </Link>
          </div>

          <div className="p-0 flex-1 overflow-x-auto">
            {!recent_schools || recent_schools.length === 0 ? (
              <div className="p-12 text-center text-xs text-slate-400 font-medium">
                No campuses registered yet.
              </div>
            ) : (
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-50 text-slate-500 font-bold text-[10px] uppercase tracking-wider">
                  <tr>
                    <th className="px-6 py-3">School Name</th>
                    <th className="px-6 py-3">Plan</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3 text-right">Registered</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {recent_schools.slice(0, 6).map((s) => (
                    <tr key={s._id || s.name} className="hover:bg-slate-50/70 transition-colors">
                      <td className="px-6 py-3.5">
                        <span className="font-bold text-slate-900 block text-sm">{s.name}</span>
                      </td>
                      <td className="px-6 py-3.5">
                        <span className="px-2.5 py-0.5 rounded-full bg-blue-50 text-blue-700 font-bold text-[10px] border border-blue-100">
                          {s.plan || "Free Trial"}
                        </span>
                      </td>
                      <td className="px-6 py-3.5">
                        <span className={`px-2 py-0.5 rounded-full font-bold text-[10px] ${
                          s.status === 'active' 
                            ? 'bg-emerald-50 text-emerald-700 border border-emerald-200' 
                            : 'bg-amber-50 text-amber-700 border border-amber-200'
                        }`}>
                          {s.status}
                        </span>
                      </td>
                      <td className="px-6 py-3.5 text-right font-medium text-slate-400">
                        {s.created_at ? new Date(s.created_at).toLocaleDateString() : "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* Right: Quick Action Hub (1 Col) */}
        <div className="space-y-4">
          <div className="bg-white rounded-2xl border border-slate-200/90 p-5 shadow-sm space-y-3">
            <h3 className="text-sm font-bold text-slate-900 border-b border-slate-100 pb-2">
              Management Modules
            </h3>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <Link
                to="/schools"
                className="p-3 bg-slate-50 hover:bg-blue-50 hover:text-blue-700 border border-slate-100 rounded-xl font-bold text-slate-700 transition-colors flex flex-col items-center justify-center gap-1.5 text-center"
              >
                <AppIcon name="Building2" size={18} className="text-blue-600" />
                <span>Schools</span>
              </Link>
              <Link
                to="/payments"
                className="p-3 bg-slate-50 hover:bg-emerald-50 hover:text-emerald-700 border border-slate-100 rounded-xl font-bold text-slate-700 transition-colors flex flex-col items-center justify-center gap-1.5 text-center relative"
              >
                <AppIcon name="CreditCard" size={18} className="text-emerald-600" />
                <span>Payments</span>
                {pendingPaymentsCount > 0 && (
                  <span className="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-amber-500 animate-ping" />
                )}
              </Link>
              <Link
                to="/subscriptions"
                className="p-3 bg-slate-50 hover:bg-indigo-50 hover:text-indigo-700 border border-slate-100 rounded-xl font-bold text-slate-700 transition-colors flex flex-col items-center justify-center gap-1.5 text-center"
              >
                <AppIcon name="Award" size={18} className="text-indigo-600" />
                <span>Subscriptions</span>
              </Link>
              <Link
                to="/security"
                className="p-3 bg-slate-50 hover:bg-amber-50 hover:text-amber-700 border border-slate-100 rounded-xl font-bold text-slate-700 transition-colors flex flex-col items-center justify-center gap-1.5 text-center"
              >
                <AppIcon name="KeyRound" size={18} className="text-amber-600" />
                <span>Security</span>
              </Link>
              <Link
                to="/users"
                className="p-3 bg-slate-50 hover:bg-rose-50 hover:text-rose-700 border border-slate-100 rounded-xl font-bold text-slate-700 transition-colors flex flex-col items-center justify-center gap-1.5 text-center"
              >
                <AppIcon name="Users" size={18} className="text-rose-600" />
                <span>Users</span>
              </Link>
            </div>
          </div>

          {/* Platform Support Info Card */}
          <div className="bg-slate-900 text-white rounded-2xl p-5 shadow-md space-y-2">
            <div className="flex items-center gap-2">
              <AppIcon name="PhoneCall" size={16} className="text-blue-400" />
              <span className="text-xs font-bold text-slate-300">Super Admin Billing Desk</span>
            </div>
            <p className="text-sm font-black text-white select-all">+92 306 4944326</p>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Official WhatsApp and mobile line displayed to school owners during manual payment transfers and verification.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
