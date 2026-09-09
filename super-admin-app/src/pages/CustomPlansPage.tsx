import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiRequest } from '@/lib/api'
import { showToast } from '@/utils/toast'
import { AppIcon } from 'shared/ui/AppIcon'

/* ────────────────────────────────────────────────────────────────────────
   Types (mirror backend-go/internal/domain/subscription/custom_plan.go)
   ──────────────────────────────────────────────────────────────────────── */

interface OwnerSearchResult {
  owner_id: string
  name: string
  email: string
  phone: string
  school_count: number
  schools: string[]
  plan_name: string
  plan_status: string
  phase: string
  student_limit: number
  students_used: number
  status: string
  end_date?: string
  has_custom_plan: boolean
  custom_plan_name?: string
}

interface CustomPlanContract {
  id: string
  name: string
  student_limit: number
  price: number
  currency: string
  duration_days: number
  description: string
  notes: string
  is_active: boolean
  effective_until?: string
  created_by: string
  created_at: string
  updated_at: string
  status: 'available' | 'current' | 'scheduled' | 'ending' | 'ended'
  subscription_id?: string
  subscription_end?: string
  subscription_start?: string
}

interface OwnerCustomPlanDetail {
  owner: {
    owner_id: string
    name: string
    email: string
    phone: string
    schools: { school_id: string; name: string }[]
  }
  phase: string
  current_subscription?: {
    id: string
    plan_name: string
    plan_id?: string
    student_limit: number
    price: number
    start_date: string
    end_date: string
    status: string
    is_trial: boolean
    grace_ends_at?: string
  }
  students_used: number
  students_limit: number
  custom_plans: CustomPlanContract[]
}

interface PlanFormState {
  name: string
  student_limit: string
  price: string
  currency: string
  duration_days: string
  description: string
  notes: string
  effective_from: string // '' = apply immediately
}

const EMPTY_FORM: PlanFormState = {
  name: '',
  student_limit: '',
  price: '',
  currency: 'PKR',
  duration_days: '30',
  description: '',
  notes: '',
  effective_from: '',
}

function errText(r: { message?: string; error?: any }): string {
  if (typeof r.error === 'object' && r.error && r.error.message) return r.error.message
  if (r.message) return r.message
  return 'Request failed. Please try again.'
}

function fmtMoney(n: number, cur = 'PKR') {
  return `${cur} ${(n || 0).toLocaleString()}`
}

function fmtDate(v?: string): string {
  if (!v) return '—'
  const d = new Date(v)
  return isNaN(d.getTime()) ? '—' : d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

const PHASE_LABEL: Record<string, { label: string; cls: string }> = {
  trial_active: { label: 'Trial Active', cls: 'bg-blue-50 text-blue-700' },
  trial_expiring: { label: 'Trial Ending', cls: 'bg-amber-50 text-amber-700' },
  active: { label: 'Active', cls: 'bg-emerald-50 text-emerald-700' },
  expiring: { label: 'Expiring', cls: 'bg-amber-50 text-amber-700' },
  grace: { label: 'Grace', cls: 'bg-orange-50 text-orange-700' },
  expired: { label: 'Expired', cls: 'bg-rose-50 text-rose-700' },
  suspended: { label: 'Suspended', cls: 'bg-rose-100 text-rose-700' },
  scheduled: { label: 'Scheduled', cls: 'bg-violet-50 text-violet-700' },
}

/* ────────────────────────────────────────────────────────────────────────
   Modal shell (Tailwind overlay — no browser-native dialogs)
   ──────────────────────────────────────────────────────────────────────── */

function Modal({
  title,
  subtitle,
  onClose,
  children,
  footer,
}: {
  title: string
  subtitle?: string
  onClose: () => void
  children: React.ReactNode
  footer?: React.ReactNode
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-sm">
      <div className="w-full max-w-lg bg-white rounded-2xl shadow-2xl overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-100 flex items-start justify-between gap-3">
          <div>
            <h3 className="text-base font-black text-slate-900 tracking-tight">{title}</h3>
            {subtitle && <p className="text-xs text-slate-500 font-medium mt-0.5">{subtitle}</p>}
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-500 flex items-center justify-center shrink-0"
          >
            <AppIcon name="X" size={16} />
          </button>
        </div>
        <div className="px-5 py-4 max-h-[65vh] overflow-y-auto">{children}</div>
        {footer && <div className="px-5 py-3.5 border-t border-slate-100 bg-slate-50/70 flex justify-end gap-2">{footer}</div>}
      </div>
    </div>
  )
}

/* ──────────────────────────────────────────────────────────────────────── */

export function CustomPlansPage() {
  const [searchParams] = useSearchParams()
  const [query, setQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const [owners, setOwners] = useState<OwnerSearchResult[]>([])
  const [searched, setSearched] = useState(false)
  const [selected, setSelected] = useState<OwnerCustomPlanDetail | null>(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [activeTab, setActiveTab] = useState<'plans' | 'create'>('plans')

  // Create / edit
  const [formOpen, setFormOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<PlanFormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  // End (transition) modal
  const [endTarget, setEndTarget] = useState<CustomPlanContract | null>(null)
  const [endDays, setEndDays] = useState('3')
  const [endReason, setEndReason] = useState('')
  const [ending, setEnding] = useState(false)

  // Activate confirm modal
  const [activateTarget, setActivateTarget] = useState<CustomPlanContract | null>(null)
  const [activating, setActivating] = useState(false)

  const loadOwner = useCallback(async (ownerId: string) => {
    setLoadingDetail(true)
    setSelected(null)
    const res = await apiRequest<OwnerCustomPlanDetail>(`/api/super-admin/owners/${encodeURIComponent(ownerId)}/custom-plans`)
    setLoadingDetail(false)
    if (res.ok && res.data) {
      setSelected(res.data)
      setActiveTab('plans')
    } else {
      showToast(errText(res), 'error')
    }
  }, [])

  // Deep link: /custom-plans?owner=<owner_id>
  useEffect(() => {
    const oid = searchParams.get('owner')
    if (oid) loadOwner(oid)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams])

  const runSearch = useCallback(async (customQuery?: string) => {
    const q = typeof customQuery === 'string' ? customQuery.trim() : query.trim()
    setSearching(true)
    setSearched(true)
    const res = await apiRequest<{ items: OwnerSearchResult[] }>(
      `/api/super-admin/owners/search${q ? `?q=${encodeURIComponent(q)}` : ''}`
    )
    setSearching(false)
    if (res.ok && res.data?.items) {
      setOwners(res.data.items)
      if (res.data.items.length === 0 && q) showToast('No owners matched your search.', 'info')
    } else {
      showToast(errText(res), 'error')
    }
  }, [query])

  // Initial load: fetch all registered owners by default
  useEffect(() => {
    void runSearch('')
  }, [])

  const openCreate = () => {
    setEditId(null)
    setForm(EMPTY_FORM)
    setFormOpen(true)
  }

  const openEdit = (p: CustomPlanContract) => {
    setEditId(p.id)
    setForm({
      name: p.name,
      student_limit: String(p.student_limit),
      price: String(p.price),
      currency: p.currency || 'PKR',
      duration_days: String(p.duration_days || 30),
      description: p.description || '',
      notes: p.notes || '',
      effective_from: '',
    })
    setFormOpen(true)
  }

  const savePlan = async () => {
    if (!selected) return
    const ownerId = selected.owner.owner_id
    const limit = parseInt(form.student_limit, 10)
    const price = parseInt(form.price, 10)
    const days = parseInt(form.duration_days, 10)
    if (!form.name.trim() || isNaN(limit) || limit < 1 || isNaN(price) || price < 0 || isNaN(days) || days < 1) {
      showToast('Name, a positive capacity, a non-negative price and duration are required.', 'error')
      return
    }
    const body = {
      name: form.name.trim(),
      student_limit: limit,
      price,
      currency: form.currency || 'PKR',
      duration_days: days,
      description: form.description.trim(),
      notes: form.notes.trim(),
      effective_from: editId ? undefined : form.effective_from.trim(),
    }
    setSaving(true)
    const res = editId
      ? await apiRequest(`/api/super-admin/owners/${encodeURIComponent(ownerId)}/custom-plans/${encodeURIComponent(editId)}`, {
          method: 'PATCH',
          body: JSON.stringify(body),
        })
      : await apiRequest(`/api/super-admin/owners/${encodeURIComponent(ownerId)}/custom-plans`, {
          method: 'POST',
          body: JSON.stringify(body),
        })
    setSaving(false)
    if (res.ok) {
      showToast(res.data?.message || (editId ? 'Custom plan updated.' : 'Custom plan created.'), 'success')
      setFormOpen(false)
      await loadOwner(ownerId)
    } else {
      showToast(errText(res), 'error')
    }
  }

  const doActivate = async () => {
    if (!selected || !activateTarget) return
    setActivating(true)
    const res = await apiRequest(
      `/api/super-admin/owners/${encodeURIComponent(selected.owner.owner_id)}/custom-plans/${encodeURIComponent(activateTarget.id)}/activate`,
      { method: 'POST' }
    )
    setActivating(false)
    setActivateTarget(null)
    if (res.ok) {
      showToast(res.data?.message || 'Custom plan activated.', 'success')
      await loadOwner(selected.owner.owner_id)
    } else {
      showToast(errText(res), 'error')
    }
  }

  const doEnd = async () => {
    if (!selected || !endTarget) return
    const days = parseInt(endDays, 10)
    if (isNaN(days) || days < 0 || days > 60) {
      showToast('Transition days must be between 0 and 60.', 'error')
      return
    }
    setEnding(true)
    const res = await apiRequest(
      `/api/super-admin/owners/${encodeURIComponent(selected.owner.owner_id)}/custom-plans/${encodeURIComponent(endTarget.id)}/end`,
      { method: 'POST', body: JSON.stringify({ transition_days: days, reason: endReason.trim() }) }
    )
    setEnding(false)
    setEndTarget(null)
    if (res.ok) {
      showToast(res.data?.message || 'Custom plan ended.', 'success')
      await loadOwner(selected.owner.owner_id)
    } else {
      showToast(errText(res), 'error')
    }
  }

  const clearSelection = () => {
    setSelected(null)
    setOwners([])
    setSearched(false)
    setQuery('')
  }

  const selectedOwner = selected?.owner
  const current = selected?.current_subscription

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <span className="px-2 py-0.5 rounded-full text-[10px] font-black uppercase tracking-wider bg-violet-100 text-violet-700">
              Owner-Specific Contracts
            </span>
            <span className="text-xs font-semibold text-slate-400">· Super Admin Only</span>
          </div>
          <h1 className="text-2xl font-black text-slate-900 tracking-tight mt-1">Custom Plans</h1>
          <p className="text-sm text-slate-500 font-medium mt-0.5">
            Negotiate private plans for one Owner — capacity, price and billing period. Each contract is visible only to
            its assigned Owner.
          </p>
        </div>
        {selected && (
          <button
            onClick={clearSelection}
            className="self-start sm:self-center px-3.5 py-2 rounded-xl bg-white border border-slate-200 text-xs font-bold text-slate-600 hover:bg-slate-50 transition flex items-center gap-1.5"
          >
            <AppIcon name="ArrowLeft" size={14} />
            Back to search
          </button>
        )}
      </div>

      {!selected ? (
        /* ── Search Owners ──────────────────────────────────────────── */
        <div className="bg-white rounded-2xl border border-slate-200/90 shadow-sm">
          <div className="px-5 py-4 border-b border-slate-100">
            <h3 className="text-sm font-extrabold text-slate-900 tracking-tight">Search Owners</h3>
            <p className="text-[11px] text-slate-500 font-medium">
              Search backend data by owner name, email, phone, institution name or campus code.
            </p>
          </div>
          <div className="p-5">
            <div className="flex flex-col sm:flex-row gap-2">
              <div className="relative flex-1">
                <div className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">
                  <AppIcon name="Search" size={16} />
                </div>
                <input
                  value={query}
                  onChange={(e) => {
                    const val = e.target.value
                    setQuery(val)
                    if (!val.trim()) {
                      void runSearch('')
                    }
                  }}
                  onKeyDown={(e) => e.key === 'Enter' && void runSearch()}
                  placeholder="Type owner name, email, phone, school or code…"
                  className="w-full pl-9 pr-3 py-2.5 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500 focus:border-transparent"
                />
              </div>
              <button
                onClick={() => void runSearch()}
                disabled={searching}
                className="px-5 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-700 text-white text-xs font-bold transition disabled:opacity-50 flex items-center justify-center gap-1.5"
              >
                {searching ? (
                  <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                ) : (
                  <AppIcon name="Search" size={14} />
                )}
                Search
              </button>
            </div>

            {searched && owners.length === 0 && !searching && (
              <div className="mt-6 text-center py-10 text-slate-400">
                <AppIcon name="Users" size={28} className="mx-auto mb-2 opacity-50" />
                <p className="text-sm font-semibold">No owners found.</p>
                <p className="text-xs">Try a different name, email or institution keyword.</p>
              </div>
            )}

            {owners.length > 0 && (
              <div className="mt-5 overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-slate-50 text-[10px] font-black text-slate-400 uppercase tracking-wider">
                    <tr>
                      <th className="py-2.5 px-3 font-bold">Owner</th>
                      <th className="py-2.5 px-3 font-bold">Institution</th>
                      <th className="py-2.5 px-3 font-bold">Current Plan</th>
                      <th className="py-2.5 px-3 font-bold">Usage</th>
                      <th className="py-2.5 px-3 font-bold">Status</th>
                      <th className="py-2.5 px-3 font-bold text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {owners.map((o) => {
                      const ph = PHASE_LABEL[o.phase || o.plan_status] || {
                        label: (o.plan_status || '—').replace(/_/g, ' '),
                        cls: 'bg-slate-100 text-slate-600',
                      }
                      return (
                        <tr key={o.owner_id} className="hover:bg-violet-50/40 transition-colors">
                          <td className="py-3 px-3">
                            <p className="font-bold text-slate-900">{o.name || '—'}</p>
                            <p className="text-[10px] text-slate-500">{o.email}</p>
                            {o.phone && <p className="text-[10px] text-slate-400">{o.phone}</p>}
                          </td>
                          <td className="py-3 px-3">
                            <p className="font-semibold text-slate-800">{o.schools?.[0] || 'No school yet'}</p>
                            {o.school_count > 1 && (
                              <p className="text-[10px] text-slate-400">{o.school_count} campuses total</p>
                            )}
                          </td>
                          <td className="py-3 px-3">
                            <p className="font-bold text-slate-900 capitalize">{o.plan_name || 'none'}</p>
                            {o.custom_plan_name && (
                              <span className="inline-block mt-1 px-1.5 py-0.5 rounded bg-violet-50 text-violet-700 text-[9px] font-bold">
                                Custom: {o.custom_plan_name}
                              </span>
                            )}
                          </td>
                          <td className="py-3 px-3 tabular-nums">
                            {o.students_used ?? 0} / {o.student_limit || 0}
                          </td>
                          <td className="py-3 px-3">
                            <span className={`px-2 py-0.5 rounded-full text-[9px] font-bold ${ph.cls}`}>{ph.label}</span>
                          </td>
                          <td className="py-3 px-3 text-right">
                            <button
                              onClick={() => loadOwner(o.owner_id)}
                              className="px-3 py-1.5 rounded-lg bg-violet-600 hover:bg-violet-700 text-white text-[11px] font-bold transition"
                            >
                              Manage Custom Plan
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
        </div>
      ) : (
        /* ── Owner Custom Plan Manager ──────────────────────────────── */
        <div className="space-y-5">
          {/* Owner header */}
          <div className="bg-white rounded-2xl border border-slate-200/90 shadow-sm overflow-hidden">
            <div className="p-5 flex flex-col sm:flex-row sm:items-center gap-4">
              <div className="w-12 h-12 rounded-2xl bg-violet-600 text-white flex items-center justify-center font-black text-lg shrink-0">
                {(selectedOwner?.name || 'O').substring(0, 1).toUpperCase()}
              </div>
              <div className="flex-1 min-w-0">
                <h2 className="text-lg font-black text-slate-900 tracking-tight">{selectedOwner?.name}</h2>
                <p className="text-xs text-slate-500 font-medium">
                  {selectedOwner?.email}
                  {selectedOwner?.phone ? ` · ${selectedOwner.phone}` : ''}
                </p>
                <p className="text-[11px] text-slate-400 mt-0.5">
                  {selectedOwner?.schools?.length ? selectedOwner.schools.map((s) => s.name || s.school_id).join(' · ') : 'No school rows'}
                </p>
              </div>
              <div className="flex items-center gap-2 flex-wrap shrink-0">
                <button
                  onClick={openCreate}
                  className="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-white text-xs font-bold transition flex items-center gap-1.5 shadow-sm"
                >
                  <AppIcon name="Plus" size={14} />
                  Create Custom Plan
                </button>
              </div>
            </div>

            {/* Summary strip */}
            <div className="grid grid-cols-2 sm:grid-cols-4 divide-x divide-slate-100 border-t border-slate-100 bg-slate-50/50">
              <div className="px-5 py-3">
                <p className="text-[10px] font-black text-slate-400 uppercase tracking-wider">Current Plan</p>
                <p className="text-sm font-black text-slate-900 capitalize truncate mt-0.5">
                  {current ? current.plan_name : 'None'}
                </p>
                {current && (
                  <span
                    className={`mt-1 inline-block px-2 py-0.5 rounded-full text-[9px] font-bold ${
                      (PHASE_LABEL[selected?.phase || ''] || { cls: 'bg-slate-100 text-slate-600' }).cls
                    }`}
                  >
                    {(PHASE_LABEL[selected?.phase || ''] || { label: selected?.phase || '—' }).label}
                  </span>
                )}
              </div>
              <div className="px-5 py-3">
                <p className="text-[10px] font-black text-slate-400 uppercase tracking-wider">Students</p>
                <p className="text-sm font-black text-slate-900 tabular-nums mt-0.5">
                  {selected?.students_used ?? 0} / {selected?.students_limit || 0}
                </p>
                <p className="text-[10px] text-slate-400 font-medium mt-0.5">
                  {(selected?.students_limit ?? 0) > 0
                    ? `${Math.min(100, Math.round(((selected?.students_used ?? 0) / (selected?.students_limit || 1)) * 100))}% used`
                    : 'aggregated across campuses'}
                </p>
              </div>
              <div className="px-5 py-3">
                <p className="text-[10px] font-black text-slate-400 uppercase tracking-wider">Renews / Ends</p>
                <p className="text-sm font-black text-slate-900 mt-0.5 tabular-nums">{fmtDate(current?.end_date)}</p>
                <p className="text-[10px] text-slate-400 font-medium mt-0.5 capitalize">{(current?.status || 'no active period').replace(/_/g, ' ')}</p>
              </div>
              <div className="px-5 py-3">
                <p className="text-[10px] font-black text-slate-400 uppercase tracking-wider">Custom Contracts</p>
                <p className="text-sm font-black text-slate-900 mt-0.5">{selected?.custom_plans?.length ?? 0}</p>
                <p className="text-[10px] text-slate-400 font-medium mt-0.5">private agreements</p>
              </div>
            </div>
          </div>

          {/* Contracts */}
          <div className="bg-white rounded-2xl border border-slate-200/90 shadow-sm">
            <div className="px-5 py-4 border-b border-slate-100 flex items-center justify-between gap-3">
              <div>
                <h3 className="text-sm font-extrabold text-slate-900 tracking-tight">Custom Plan Contracts</h3>
                <p className="text-[11px] text-slate-500 font-medium">
                  Only this Owner can see these negotiated plans. History is never deleted.
                </p>
              </div>
              <button
                onClick={() => setActiveTab(activeTab === 'plans' ? 'create' : 'plans')}
                className="px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 text-[11px] font-bold transition"
              >
                {activeTab === 'plans' ? 'Usage Guide' : 'View Contracts'}
              </button>
            </div>

            {loadingDetail ? (
              <div className="py-16 flex items-center justify-center text-slate-400">
                <span className="w-6 h-6 border-2 border-violet-200 border-t-violet-600 rounded-full animate-spin" />
              </div>
            ) : (selected?.custom_plans?.length ?? 0) === 0 ? (
              <div className="py-14 text-center">
                <AppIcon name="FileText" size={30} className="mx-auto text-slate-300 mb-2" />
                <p className="text-sm font-bold text-slate-600">No custom plans yet</p>
                <p className="text-xs text-slate-400 mt-1">
                  Use “Create Custom Plan” to negotiate a private plan for this Owner.
                </p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-slate-50 text-[10px] font-black text-slate-400 uppercase tracking-wider">
                    <tr>
                      <th className="py-2.5 px-5 font-bold">Plan</th>
                      <th className="py-2.5 px-3 font-bold">Terms</th>
                      <th className="py-2.5 px-3 font-bold">Binding Period</th>
                      <th className="py-2.5 px-3 font-bold">Status</th>
                      <th className="py-2.5 px-3 font-bold text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {selected?.custom_plans?.map((p) => (
                      <tr key={p.id} className="hover:bg-violet-50/40 transition-colors align-top">
                        <td className="py-3.5 px-5">
                          <p className="font-black text-slate-900">{p.name}</p>
                          <p className="text-[10px] text-slate-400 mt-0.5">{p.id}</p>
                          {p.description && <p className="text-[11px] text-slate-500 mt-1 max-w-[260px]">{p.description}</p>}
                          {p.notes && <p className="text-[10px] text-slate-400 italic mt-0.5 max-w-[260px]">{p.notes}</p>}
                        </td>
                        <td className="py-3.5 px-3">
                          <p className="font-bold text-slate-900 tabular-nums">{fmtMoney(p.price, p.currency)}</p>
                          <p className="text-[11px] text-slate-500">/ {p.duration_days || 30} days</p>
                          <p className="text-[11px] font-semibold text-slate-700 mt-0.5">
                            {p.student_limit.toLocaleString()} students
                          </p>
                        </td>
                        <td className="py-3.5 px-3">
                          {p.subscription_start || p.subscription_end ? (
                            <>
                              <p className="text-[11px] font-semibold text-slate-700">
                                {fmtDate(p.subscription_start)} → {fmtDate(p.subscription_end)}
                              </p>
                            </>
                          ) : (
                            <p className="text-[11px] text-slate-400">Not yet bound</p>
                          )}
                        </td>
                        <td className="py-3.5 px-3">
                          <StatusChip status={p.status} />
                          {!p.is_active && p.status === 'current' && (
                            <p className="text-[9px] text-amber-600 font-bold mt-1">Transition window running</p>
                          )}
                        </td>
                        <td className="py-3.5 px-3">
                          <div className="flex justify-end gap-1.5 flex-wrap">
                            {p.status === 'current' && (
                              <button
                                onClick={() => openEdit(p)}
                                className="px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 text-[10px] font-bold transition"
                              >
                                Edit
                              </button>
                            )}
                            {(p.status === 'scheduled' || p.status === 'ending' || p.status === 'available' || p.status === 'ended') && (
                              <button
                                onClick={() => setActivateTarget(p)}
                                disabled={activating}
                                className="px-2.5 py-1.5 rounded-lg bg-violet-600 hover:bg-violet-700 text-white text-[10px] font-bold transition disabled:opacity-50"
                              >
                                {p.status === 'ended' ? 'Reactivate' : p.status === 'scheduled' ? 'Activate Now' : 'Activate'}
                              </button>
                            )}
                            {(p.status === 'current' || p.status === 'scheduled' || p.status === 'ending' || p.status === 'available') && (
                              <button
                                onClick={() => {
                                  setEndTarget(p)
                                  setEndDays('3')
                                  setEndReason('')
                                }}
                                className="px-2.5 py-1.5 rounded-lg bg-rose-50 hover:bg-rose-100 text-rose-600 text-[10px] font-bold transition"
                              >
                                End
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ── Create / Edit form modal ─────────────────────────────────── */}
      {formOpen && selected && (
        <Modal
          title={editId ? 'Edit Custom Plan' : 'Create Custom Plan'}
          subtitle={`${selected.owner.name} · ${selected.owner.email}`}
          onClose={() => setFormOpen(false)}
          footer={
            <>
              <button
                onClick={() => setFormOpen(false)}
                className="px-4 py-2 rounded-xl bg-white border border-slate-200 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
              >
                Cancel
              </button>
              <button
                onClick={savePlan}
                disabled={saving}
                className="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-white text-xs font-bold transition disabled:opacity-50 flex items-center gap-1.5"
              >
                {saving && <span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
                {editId ? 'Save Changes' : form.effective_from.trim() ? 'Create & Schedule' : 'Create & Activate'}
              </button>
            </>
          }
        >
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label="Plan Name *" className="sm:col-span-2">
              <input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. Custom Enterprise"
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
              />
            </Field>
            <Field label="Student Capacity *">
              <input
                type="number"
                min={1}
                value={form.student_limit}
                onChange={(e) => setForm({ ...form, student_limit: e.target.value })}
                placeholder="e.g. 1600"
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
              />
            </Field>
            <Field label="Monthly Price *">
              <div className="flex gap-2">
                <input
                  type="number"
                  min={0}
                  value={form.price}
                  onChange={(e) => setForm({ ...form, price: e.target.value })}
                  placeholder="24000"
                  className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
                />
                <select
                  value={form.currency}
                  onChange={(e) => setForm({ ...form, currency: e.target.value })}
                  className="px-2 py-2 rounded-xl border border-slate-200 text-sm font-bold bg-white"
                >
                  <option>PKR</option>
                  <option>USD</option>
                </select>
              </div>
            </Field>
            <Field label="Billing Duration (days) *">
              <input
                type="number"
                min={1}
                max={730}
                value={form.duration_days}
                onChange={(e) => setForm({ ...form, duration_days: e.target.value })}
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
              />
            </Field>
            <Field label={editId ? 'Current values are kept when empty' : 'Effective From (optional)'}>
              {editId ? (
                <p className="text-xs text-slate-500 bg-slate-50 border border-slate-200 rounded-xl px-3 py-2">
                  Renegotiated terms apply to the current bound period. Historical billing is never rewritten.
                </p>
              ) : (
                <>
                  <input
                    type="datetime-local"
                    value={form.effective_from}
                    onChange={(e) => setForm({ ...form, effective_from: e.target.value })}
                    className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
                  />
                  <p className="text-[10px] text-slate-400 mt-1">
                    Leave empty to activate immediately (replaces the current plan safely). Pick a future date to
                    schedule activation at the period boundary.
                  </p>
                </>
              )}
            </Field>
            <Field label="Customer-facing Description" className="sm:col-span-2">
              <input
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="Shown to the Owner on the plan card"
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
              />
            </Field>
            <Field label="Internal Admin Notes" className="sm:col-span-2">
              <textarea
                rows={2}
                value={form.notes}
                onChange={(e) => setForm({ ...form, notes: e.target.value })}
                placeholder="Negotiation context — never shown to the Owner"
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500 resize-none"
              />
            </Field>
          </div>
        </Modal>
      )}

      {/* ── Activate confirm ─────────────────────────────────────────── */}
      {activateTarget && selected && (
        <Modal
          title={activateTarget.status === 'ended' ? 'Reactivate Custom Plan?' : 'Activate Custom Plan Now?'}
          subtitle={activateTarget.name}
          onClose={() => setActivateTarget(null)}
          footer={
            <>
              <button
                onClick={() => setActivateTarget(null)}
                className="px-4 py-2 rounded-xl bg-white border border-slate-200 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
              >
                Cancel
              </button>
              <button
                onClick={doActivate}
                disabled={activating}
                className="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-white text-xs font-bold transition disabled:opacity-50 flex items-center gap-1.5"
              >
                {activating && <span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
                Confirm Activation
              </button>
            </>
          }
        >
          <p className="text-xs text-slate-600 leading-relaxed">
            This makes <strong className="text-slate-900">{activateTarget.name}</strong> the Owner's current plan with a
            capacity of{' '}
            <strong className="text-slate-900">{activateTarget.student_limit.toLocaleString()} students</strong> at{' '}
            <strong className="text-slate-900">{fmtMoney(activateTarget.price, activateTarget.currency)}</strong>. Any
            live standard or trial period is ended at this moment (history is preserved) and the aggregate student limit
            is enforced immediately.
          </p>
          {(selected?.students_used ?? 0) > activateTarget.student_limit && (
            <p className="mt-3 text-xs font-bold text-rose-600 bg-rose-50 border border-rose-200 rounded-lg px-3 py-2">
              Warning: the Owner currently has {selected?.students_used} active students, which exceeds this plan's{' '}
              {activateTarget.student_limit} capacity. Activation will be rejected by the backend until capacity is
              raised or enrollment reduced.
            </p>
          )}
        </Modal>
      )}

      {/* ── End (transition) modal ───────────────────────────────────── */}
      {endTarget && selected && (
        <Modal
          title="End Custom Plan Agreement"
          subtitle={endTarget.name}
          onClose={() => setEndTarget(null)}
          footer={
            <>
              <button
                onClick={() => setEndTarget(null)}
                className="px-4 py-2 rounded-xl bg-white border border-slate-200 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
              >
                Cancel
              </button>
              <button
                onClick={doEnd}
                disabled={ending}
                className="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-700 text-white text-xs font-bold transition disabled:opacity-50 flex items-center gap-1.5"
              >
                {ending && <span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
                End Custom Plan
              </button>
            </>
          }
        >
          <p className="text-xs text-slate-600 leading-relaxed">
            The agreement is retired (never deleted) and the Owner keeps access for the transition period below. After
            it ends, the subscription follows the standard expiry → grace → suspension policy unless a replacement plan
            is assigned.
          </p>
          <div className="mt-4 space-y-3">
            <Field label="Transition / Grace Days (0 – 60)">
              <input
                type="number"
                min={0}
                max={60}
                value={endDays}
                onChange={(e) => setEndDays(e.target.value)}
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
              />
              <p className="text-[10px] text-slate-400 mt-1">0 ends access immediately. Suggested: 3–8 days.</p>
            </Field>
            <Field label="Reason (internal, stored in notes)">
              <textarea
                rows={2}
                value={endReason}
                onChange={(e) => setEndReason(e.target.value)}
                placeholder="e.g. Contract renegotiated, owner moving to Premium"
                className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500 resize-none"
              />
            </Field>
          </div>
        </Modal>
      )}
    </div>
  )
}

function Field({
  label,
  children,
  className = '',
}: {
  label: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <label className={`block ${className}`}>
      <span className="block text-[11px] font-bold text-slate-600 uppercase tracking-wider mb-1.5">{label}</span>
      {children}
    </label>
  )
}

const STATUS_META: Record<string, { label: string; cls: string }> = {
  current: { label: 'Current', cls: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
  scheduled: { label: 'Scheduled', cls: 'bg-violet-50 text-violet-700 border-violet-200' },
  ending: { label: 'Ending', cls: 'bg-amber-50 text-amber-700 border-amber-200' },
  ended: { label: 'Ended', cls: 'bg-slate-100 text-slate-500 border-slate-200' },
  available: { label: 'Available', cls: 'bg-blue-50 text-blue-700 border-blue-200' },
}

function StatusChip({ status }: { status: string }) {
  const m = STATUS_META[status] || { label: status, cls: 'bg-slate-100 text-slate-600 border-slate-200' }
  return (
    <span className={`inline-block px-2 py-0.5 rounded-full text-[9px] font-black border ${m.cls}`}>{m.label}</span>
  )
}
