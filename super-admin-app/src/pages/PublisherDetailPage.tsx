import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { apiRequest } from '@/lib/api'
import { CreateReferralModal } from '@/components/referral/CreateReferralModal'

interface Token {
  id: string
  status: string
  plan_name_snapshot: string
  monthly_price_snapshot: number
  expires_at: string | null
  used_at: string | null
  created_at: string
}

export function PublisherDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [tokens, setTokens] = useState<Token[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)

  const loadTokens = async () => {
    setLoading(true)
    const res = await apiRequest(`/api/referral/publishers/${id}/tokens`)
    if (res.ok && res.data) {
      setTokens(res.data)
    }
    setLoading(false)
  }

  useEffect(() => { loadTokens() }, [id])

  const statusBadge = (status: string) => {
    switch (status) {
      case 'UNUSED': return 'bg-amber-50 text-amber-700 border-amber-100'
      case 'USED': return 'bg-green-50 text-green-700 border-green-100'
      case 'REVOKED': return 'bg-red-50 text-red-700 border-red-100'
      default: return 'bg-slate-50 text-slate-700 border-slate-100'
    }
  }

  return (
    <div className="p-8 max-w-7xl mx-auto min-h-screen">
      <div className="mb-6">
        <Link to="/publishers" className="text-sm font-medium text-slate-500 hover:text-slate-900 flex items-center gap-1 w-fit transition-colors">
          <AppIcon name="arrow-left" className="h-4 w-4" />
          Back to Publishers
        </Link>
      </div>

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900">
            Publisher Details
          </h1>
          <p className="text-slate-500 text-sm mt-1 font-mono">{id}</p>
        </div>
        <button 
          onClick={() => setShowModal(true)}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors flex items-center gap-2"
        >
          <AppIcon name="plus" className="h-4 w-4" />
          Generate New Link
        </button>
      </div>

      <h3 className="text-lg font-bold text-slate-900 mb-4">Referral Tokens</h3>
      
      <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-12 text-center text-slate-400">Loading tokens...</div>
        ) : tokens.length === 0 ? (
          <div className="p-16 text-center">
            <div className="bg-slate-50 w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4 border border-slate-100">
              <AppIcon name="link" className="h-8 w-8 text-slate-400" />
            </div>
            <h3 className="text-slate-900 font-medium mb-1">No Links Generated</h3>
            <p className="text-slate-500 text-sm max-w-sm mx-auto">Generate a referral link to share with schools.</p>
          </div>
        ) : (
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-slate-50 text-slate-500 text-[11px] uppercase tracking-wider font-semibold border-b border-slate-200">
              <tr>
                <th className="px-6 py-4">ID</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4">Locked Plan</th>
                <th className="px-6 py-4 text-right">Locked Price</th>
                <th className="px-6 py-4">Created At</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {tokens.map(tok => (
                <tr key={tok.id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="px-6 py-4 font-mono text-slate-500 text-xs">{tok.id}</td>
                  <td className="px-6 py-4">
                    <span className={`text-[10px] font-bold px-2.5 py-1 rounded-full border ${statusBadge(tok.status)}`}>
                      {tok.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 font-medium text-slate-900">{tok.plan_name_snapshot}</td>
                  <td className="px-6 py-4 text-right font-mono text-slate-700">Rs. {tok.monthly_price_snapshot.toLocaleString()}</td>
                  <td className="px-6 py-4 text-slate-500 text-xs">{new Date(tok.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {showModal && (
        <CreateReferralModal 
          publisherId={id!} 
          onClose={() => setShowModal(false)}
          onSuccess={() => loadTokens()}
        />
      )}
    </div>
  )
}
