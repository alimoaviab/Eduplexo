import { useState, useEffect } from 'react'
import { AppIcon } from 'shared/ui/AppIcon'
import { apiRequest } from '@/lib/api'
import { showToast } from '@/utils/toast'

interface CredentialsData {
  id: string
  email: string
  role: string
  first_name?: string
  last_name?: string
  updated_at?: string
}

export function SecurityPage() {
  const [credentials, setCredentials] = useState<CredentialsData | null>(null)
  const [loadingInitial, setLoadingInitial] = useState(true)

  // Email change form state
  const [newEmail, setNewEmail] = useState('')
  const [emailCurrentPassword, setEmailCurrentPassword] = useState('')
  const [showEmailCurrentPass, setShowEmailCurrentPass] = useState(false)
  const [emailLoading, setEmailLoading] = useState(false)
  const [emailError, setEmailError] = useState('')

  // Password change form state
  const [passwordCurrentPassword, setPasswordCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPassCurrent, setShowPassCurrent] = useState(false)
  const [showPassNew, setShowPassNew] = useState(false)
  const [showPassConfirm, setShowPassConfirm] = useState(false)
  const [passwordLoading, setPasswordLoading] = useState(false)
  const [passwordError, setPasswordError] = useState('')

  const fetchCredentials = async () => {
    try {
      const res = await apiRequest<CredentialsData>('/api/super-admin/credentials')
      if (res.ok && res.data) {
        setCredentials(res.data)
      } else {
        // Fallback from session if endpoint isn't ready
        const stored = sessionStorage.getItem('sa_user')
        if (stored) {
          const parsed = JSON.parse(stored)
          setCredentials({
            id: parsed.id,
            email: parsed.email,
            role: parsed.role,
          })
        }
      }
    } catch {
      // Fallback from session
      const stored = sessionStorage.getItem('sa_user')
      if (stored) {
        try {
          const parsed = JSON.parse(stored)
          setCredentials({
            id: parsed.id,
            email: parsed.email,
            role: parsed.role,
          })
        } catch {}
      }
    } finally {
      setLoadingInitial(false)
    }
  }

  useEffect(() => {
    void fetchCredentials()
  }, [])

  const handleEmailSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setEmailError('')

    const cleanEmail = newEmail.trim().toLowerCase()
    if (!cleanEmail || !cleanEmail.includes('@')) {
      setEmailError('Please enter a valid email address.')
      return
    }
    if (cleanEmail === credentials?.email?.toLowerCase()) {
      setEmailError('New email must be different from current email.')
      return
    }
    if (!emailCurrentPassword) {
      setEmailError('Current password is required to verify your identity.')
      return
    }

    setEmailLoading(true)
    const res = await apiRequest<{ success: boolean; message: string; email: string }>('/api/super-admin/credentials', {
      method: 'POST',
      body: JSON.stringify({
        current_password: emailCurrentPassword,
        new_email: cleanEmail,
      }),
    })
    setEmailLoading(false)

    if (!res.ok || !res.data?.success) {
      setEmailError(res.message || 'Failed to update email address.')
      return
    }

    showToast(res.data.message || 'Email updated successfully!', 'success')
    setCredentials((prev) => prev ? { ...prev, email: cleanEmail } : null)

    // Update stored session
    const stored = sessionStorage.getItem('sa_user')
    if (stored) {
      try {
        const parsed = JSON.parse(stored)
        parsed.email = cleanEmail
        sessionStorage.setItem('sa_user', JSON.stringify(parsed))
      } catch {}
    }

    setNewEmail('')
    setEmailCurrentPassword('')
  }

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setPasswordError('')

    if (!passwordCurrentPassword) {
      setPasswordError('Current password is required.')
      return
    }
    if (!newPassword || newPassword.length < 8) {
      setPasswordError('New password must be at least 8 characters long.')
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError('New password and confirmation do not match.')
      return
    }
    if (newPassword === passwordCurrentPassword) {
      setPasswordError('New password must be different from your current password.')
      return
    }

    setPasswordLoading(true)
    const res = await apiRequest<{ success: boolean; message: string }>('/api/super-admin/credentials', {
      method: 'POST',
      body: JSON.stringify({
        current_password: passwordCurrentPassword,
        new_password: newPassword,
      }),
    })
    setPasswordLoading(false)

    if (!res.ok || !res.data?.success) {
      setPasswordError(res.message || 'Failed to update password.')
      return
    }

    showToast(res.data.message || 'Password updated successfully!', 'success')
    setPasswordCurrentPassword('')
    setNewPassword('')
    setConfirmPassword('')
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6 pb-12">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black text-slate-900 tracking-tight flex items-center gap-2.5">
            <span className="p-2 rounded-xl bg-blue-50 text-blue-600 border border-blue-100/60 inline-flex">
              <AppIcon name="KeyRound" size={22} />
            </span>
            Security & Login Credentials
          </h1>
          <p className="text-sm text-slate-500 mt-1">
            Manage and update your Super Admin login email and password for the Eduplexo Platform.
          </p>
        </div>
      </div>

      {/* Account Overview Card */}
      <div className="bg-white rounded-2xl border border-slate-200 p-6 shadow-xs">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3.5">
            <div className="h-12 w-12 rounded-2xl bg-gradient-to-tr from-blue-600 to-indigo-600 flex items-center justify-center text-white font-black text-lg shadow-md shadow-blue-500/20">
              <AppIcon name="Shield" size={24} />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-base font-bold text-slate-900">
                  {credentials?.email || 'Loading...'}
                </span>
                <span className="px-2 py-0.5 rounded-full text-[10px] font-black uppercase tracking-wider bg-blue-100 text-blue-700">
                  Super Admin
                </span>
              </div>
              <p className="text-xs text-slate-400 mt-0.5">
                Primary Platform Administrator • Full Access Control
              </p>
            </div>
          </div>
          <div className="text-xs text-slate-500 bg-slate-50 border border-slate-100 rounded-xl px-4 py-2.5 sm:text-right">
            <span className="font-semibold text-slate-700 block">Persistence Status</span>
            <span className="text-emerald-600 font-medium flex items-center gap-1 sm:justify-end">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              Direct Database & Memory Sync Active
            </span>
          </div>
        </div>

        <div className="mt-5 pt-5 border-t border-slate-100 flex items-start gap-2.5 text-xs text-slate-600 bg-blue-50/60 p-3.5 rounded-xl border border-blue-100/60">
          <AppIcon name="Info" size={16} className="text-blue-600 shrink-0 mt-0.5" />
          <span>
            Any email or password changes made here take effect immediately for your next login and will persist permanently across server restarts and deployments.
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Change Email Card */}
        <div className="bg-white rounded-2xl border border-slate-200 p-6 shadow-xs flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-2.5 mb-1.5">
              <div className="p-1.5 rounded-lg bg-indigo-50 text-indigo-600">
                <AppIcon name="Mail" size={18} />
              </div>
              <h2 className="text-base font-bold text-slate-900">Change Login Email</h2>
            </div>
            <p className="text-xs text-slate-500 mb-5">
              Update the email address used to sign in to this Super Admin control panel.
            </p>

            {emailError && (
              <div className="p-3 mb-4 rounded-xl bg-red-50 border border-red-100 text-xs text-red-700 font-medium flex items-start gap-2">
                <AppIcon name="AlertCircle" size={15} className="shrink-0 mt-0.5" />
                <span>{emailError}</span>
              </div>
            )}

            <form onSubmit={handleEmailSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  Current Email
                </label>
                <input
                  type="text"
                  disabled
                  value={credentials?.email || ''}
                  className="w-full h-10 px-3.5 rounded-lg border border-slate-200 bg-slate-50 text-xs text-slate-500 font-medium cursor-not-allowed select-all"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  New Email Address
                </label>
                <input
                  type="email"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  placeholder="admin@eduplexo.com"
                  required
                  className="w-full h-10 px-3.5 rounded-lg border border-slate-200 text-xs outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all font-medium"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  Current Password (for authorization)
                </label>
                <div className="relative">
                  <input
                    type={showEmailCurrentPass ? 'text' : 'password'}
                    value={emailCurrentPassword}
                    onChange={(e) => setEmailCurrentPassword(e.target.value)}
                    placeholder="Enter current password"
                    required
                    className="w-full h-10 pl-3.5 pr-10 rounded-lg border border-slate-200 text-xs outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all"
                  />
                  <button
                    type="button"
                    onClick={() => setShowEmailCurrentPass(!showEmailCurrentPass)}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 p-1"
                  >
                    <AppIcon name={showEmailCurrentPass ? 'EyeOff' : 'Eye'} size={16} />
                  </button>
                </div>
              </div>

              <div className="pt-2">
                <button
                  type="submit"
                  disabled={emailLoading || !newEmail || !emailCurrentPassword}
                  className="w-full h-10 bg-blue-600 text-white text-xs font-bold rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 shadow-sm"
                >
                  {emailLoading ? (
                    <>
                      <span className="h-3.5 w-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      Updating Email...
                    </>
                  ) : (
                    <>
                      <AppIcon name="Save" size={15} />
                      Save New Email
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>

        {/* Change Password Card */}
        <div className="bg-white rounded-2xl border border-slate-200 p-6 shadow-xs flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-2.5 mb-1.5">
              <div className="p-1.5 rounded-lg bg-emerald-50 text-emerald-600">
                <AppIcon name="Lock" size={18} />
              </div>
              <h2 className="text-base font-bold text-slate-900">Change Login Password</h2>
            </div>
            <p className="text-xs text-slate-500 mb-5">
              Update your Super Admin password. Must be at least 8 characters long.
            </p>

            {passwordError && (
              <div className="p-3 mb-4 rounded-xl bg-red-50 border border-red-100 text-xs text-red-700 font-medium flex items-start gap-2">
                <AppIcon name="AlertCircle" size={15} className="shrink-0 mt-0.5" />
                <span>{passwordError}</span>
              </div>
            )}

            <form onSubmit={handlePasswordSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  Current Password
                </label>
                <div className="relative">
                  <input
                    type={showPassCurrent ? 'text' : 'password'}
                    value={passwordCurrentPassword}
                    onChange={(e) => setPasswordCurrentPassword(e.target.value)}
                    placeholder="Enter current password"
                    required
                    className="w-full h-10 pl-3.5 pr-10 rounded-lg border border-slate-200 text-xs outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassCurrent(!showPassCurrent)}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 p-1"
                  >
                    <AppIcon name={showPassCurrent ? 'EyeOff' : 'Eye'} size={16} />
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  New Password
                </label>
                <div className="relative">
                  <input
                    type={showPassNew ? 'text' : 'password'}
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="At least 8 characters"
                    required
                    minLength={8}
                    className="w-full h-10 pl-3.5 pr-10 rounded-lg border border-slate-200 text-xs outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassNew(!showPassNew)}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 p-1"
                  >
                    <AppIcon name={showPassNew ? 'EyeOff' : 'Eye'} size={16} />
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  Confirm New Password
                </label>
                <div className="relative">
                  <input
                    type={showPassConfirm ? 'text' : 'password'}
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Re-enter new password"
                    required
                    minLength={8}
                    className="w-full h-10 pl-3.5 pr-10 rounded-lg border border-slate-200 text-xs outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-all"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassConfirm(!showPassConfirm)}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 p-1"
                  >
                    <AppIcon name={showPassConfirm ? 'EyeOff' : 'Eye'} size={16} />
                  </button>
                </div>
                {confirmPassword && newPassword !== confirmPassword && (
                  <p className="text-[11px] text-red-500 font-medium mt-1">
                    Passwords do not match.
                  </p>
                )}
                {confirmPassword && newPassword === confirmPassword && (
                  <p className="text-[11px] text-emerald-600 font-medium mt-1 flex items-center gap-1">
                    <AppIcon name="Check" size={12} />
                    Passwords match.
                  </p>
                )}
              </div>

              <div className="pt-2">
                <button
                  type="submit"
                  disabled={passwordLoading || !passwordCurrentPassword || !newPassword || newPassword !== confirmPassword}
                  className="w-full h-10 bg-emerald-600 text-white text-xs font-bold rounded-lg hover:bg-emerald-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 shadow-sm"
                >
                  {passwordLoading ? (
                    <>
                      <span className="h-3.5 w-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      Updating Password...
                    </>
                  ) : (
                    <>
                      <AppIcon name="KeyRound" size={15} />
                      Update Password
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}

export default SecurityPage
