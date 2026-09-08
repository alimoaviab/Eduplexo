import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { apiRequest } from '@/lib/api'
import { showToast } from '@/utils/toast'

interface Plan {
  id: string
  name: string
  display_name: string
  price: number
  currency: string
}

interface CreateReferralModalProps {
  publisherId: string
  onClose: () => void
  onSuccess: () => void
}

export function CreateReferralModal({ publisherId, onClose, onSuccess }: CreateReferralModalProps) {
  const [plans, setPlans] = useState<Plan[]>([])
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const [customPrice, setCustomPrice] = useState<number | ''>('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [generatedLink, setGeneratedLink] = useState('')
  
  useEffect(() => {
    const loadPlans = async () => {
      // Use the super-admin plans endpoint
      const res = await apiRequest('/api/super-admin/plans')
      if (res.ok && res.data) {
        setPlans(res.data)
        if (res.data.length > 0) setSelectedPlanId(res.data[0].id)
      }
    }
    loadPlans()
  }, [])

  const selectedPlan = plans.find(p => p.id === selectedPlanId)

  // Sync custom price when plan changes
  useEffect(() => {
    if (selectedPlan && customPrice === '') {
      setCustomPrice(selectedPlan.price)
    }
  }, [selectedPlanId, selectedPlan, customPrice])

  const handleGenerate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedPlan || customPrice === '') return
    setIsSubmitting(true)
    const res = await apiRequest('/api/referral/generate', {
      method: 'POST',
      body: JSON.stringify({
        PublisherID: publisherId,
        PlanID: selectedPlan.id,
        PlanNameSnapshot: selectedPlan.display_name,
        MonthlyPriceSnapshot: Number(customPrice),
        Currency: selectedPlan.currency || 'PKR',
        BillingPeriod: 'monthly'
      })
    })
    setIsSubmitting(false)
    if (res.ok && res.data?.raw_token) {
      setGeneratedLink(`https://app.eduplexo.com/invite/${res.data.raw_token}`)
      onSuccess()
    } else {
      showToast(res.message || 'Failed to generate token', 'error')
    }
  }

  const copyLink = () => {
    navigator.clipboard.writeText(generatedLink)
    showToast('Link copied to clipboard!', 'success')
  }

  if (generatedLink) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-sm">
        <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg overflow-hidden animate-in fade-in zoom-in-95 duration-200">
          <div className="p-8 text-center">
            <div className="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-green-100 mb-4">
              <AppIcon name="check" className="h-6 w-6 text-green-600" />
            </div>
            <h2 className="text-2xl font-bold text-slate-900 mb-2">Referral Link Generated!</h2>
            <p className="text-slate-500 mb-6 text-sm">
              This link is a one-time use secret. Make sure to copy it now, as it will not be shown again.
            </p>
            
            <div className="bg-slate-50 border border-slate-200 rounded-lg p-4 flex items-center gap-3 mb-6">
              <input 
                type="text" 
                readOnly 
                value={generatedLink}
                className="flex-1 bg-transparent border-none text-slate-600 font-mono text-sm focus:outline-none"
              />
              <button 
                onClick={copyLink}
                className="p-2 bg-blue-100 hover:bg-blue-200 text-blue-700 rounded transition-colors"
                title="Copy Link"
              >
                <AppIcon name="copy" className="h-4 w-4" />
              </button>
            </div>

            <button
              onClick={onClose}
              className="w-full bg-slate-900 hover:bg-slate-800 text-white font-medium py-3 rounded-lg transition-colors"
            >
              Done
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-sm">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        <div className="p-6">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-xl font-bold text-slate-900">Generate Referral Link</h2>
            <button onClick={onClose} className="text-slate-400 hover:text-slate-600">
              <AppIcon name="x" className="h-5 w-5" />
            </button>
          </div>
          
          <form onSubmit={handleGenerate} className="space-y-5">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1.5 uppercase tracking-wider">
                Select Base Plan
              </label>
              <select
                value={selectedPlanId}
                onChange={e => {
                  setSelectedPlanId(e.target.value)
                  const p = plans.find(x => x.id === e.target.value)
                  if (p) setCustomPrice(p.price)
                }}
                className="w-full px-3 py-2 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 text-sm"
              >
                <option value="" disabled>Select a plan...</option>
                {plans.map(p => (
                  <option key={p.id} value={p.id}>{p.display_name} - Rs. {p.price.toLocaleString()}/mo</option>
                ))}
              </select>
            </div>
            
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1.5 uppercase tracking-wider">
                Override Monthly Price (Rs.)
              </label>
              <input
                type="number"
                min="0"
                required
                value={customPrice}
                onChange={e => setCustomPrice(e.target.value === '' ? '' : Number(e.target.value))}
                className="w-full px-3 py-2 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 text-sm font-mono"
              />
              <p className="text-xs text-slate-500 mt-1">
                The school will be permanently locked into this price.
              </p>
            </div>
            
            <div className="flex gap-3 pt-4 border-t border-slate-100">
              <button
                type="button"
                onClick={onClose}
                className="flex-1 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isSubmitting || !selectedPlanId || customPrice === ''}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-lg transition-colors"
              >
                <AppIcon name="link" className="h-4 w-4" />
                {isSubmitting ? 'Generating...' : 'Generate Secret Link'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}
