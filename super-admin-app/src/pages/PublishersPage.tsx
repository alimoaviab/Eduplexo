import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiRequest } from '@/lib/api'
import { showToast } from '@/utils/toast'

interface Publisher {
  id: string
  name: string
  status: string
  created_at: string
}

export function PublishersPage() {
  const [publishers, setPublishers] = useState<Publisher[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [newName, setNewName] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const loadPublishers = async () => {
    setLoading(true)
    const res = await apiRequest('/api/referral/publishers')
    if (res.ok && res.data) {
      setPublishers(res.data)
    }
    setLoading(false)
  }

  useEffect(() => { loadPublishers() }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    setIsSubmitting(true)
    const res = await apiRequest('/api/referral/publishers', {
      method: 'POST',
      body: JSON.stringify({ name: newName }),
    })
    setIsSubmitting(false)
    if (res.ok) {
      showToast('Publisher created successfully', 'success')
      setShowModal(false)
      setNewName('')
      loadPublishers()
    } else {
      showToast(res.message || 'Failed to create publisher', 'error')
    }
  }

  return (
    <div className="p-8 max-w-7xl mx-auto min-h-screen">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 flex items-center gap-2">
            <AppIcon name="users" className="h-6 w-6 text-blue-600" />
            Publishers
          </h1>
          <p className="text-slate-500 text-sm mt-1">Manage referral partners and agencies</p>
        </div>
        <button 
          onClick={() => setShowModal(true)}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors flex items-center gap-2"
        >
          <AppIcon name="plus" className="h-4 w-4" />
          Add Publisher
        </button>
      </div>

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-12 text-center text-slate-400">Loading publishers...</div>
        ) : publishers.length === 0 ? (
          <div className="p-16 text-center">
            <div className="bg-slate-50 w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4 border border-slate-100">
              <AppIcon name="users" className="h-8 w-8 text-slate-400" />
            </div>
            <h3 className="text-slate-900 font-medium mb-1">No Publishers</h3>
            <p className="text-slate-500 text-sm max-w-sm mx-auto">Get started by creating your first publisher to generate referral links.</p>
          </div>
        ) : (
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-slate-50 text-slate-500 text-[11px] uppercase tracking-wider font-semibold border-b border-slate-200">
              <tr>
                <th className="px-6 py-4">Name</th>
                <th className="px-6 py-4">ID</th>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {publishers.map(pub => (
                <tr key={pub.id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="px-6 py-4 font-medium text-slate-900">{pub.name}</td>
                  <td className="px-6 py-4 text-slate-500 font-mono text-xs">{pub.id}</td>
                  <td className="px-6 py-4">
                    <span className="text-[10px] font-bold px-2.5 py-1 rounded-full border bg-green-50 text-green-700 border-green-100">
                      {pub.status.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-right">
                    <Link to={`/publishers/${pub.id}`} className="text-blue-600 hover:text-blue-800 font-medium text-xs">
                      Manage →
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-sm">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            <div className="p-6">
              <h2 className="text-xl font-bold text-slate-900 mb-1">New Publisher</h2>
              <p className="text-slate-500 text-sm mb-6">Create a new partner for referral generation.</p>
              
              <form onSubmit={handleCreate} className="space-y-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1.5 uppercase tracking-wider">
                    Publisher Name
                  </label>
                  <input
                    type="text"
                    required
                    value={newName}
                    onChange={e => setNewName(e.target.value)}
                    className="w-full px-3 py-2 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all text-sm"
                    placeholder="e.g. EduAgency Ltd"
                  />
                </div>
                
                <div className="flex gap-3 pt-4 border-t border-slate-100">
                  <button
                    type="button"
                    onClick={() => setShowModal(false)}
                    className="flex-1 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmitting || !newName.trim()}
                    className="flex-1 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-lg transition-colors"
                  >
                    {isSubmitting ? 'Creating...' : 'Create Publisher'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
