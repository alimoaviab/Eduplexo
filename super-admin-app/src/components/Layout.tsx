import { AppIcon } from "shared/ui/AppIcon";
import { useEffect, useState } from 'react'
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom'
import { apiRequest, clearStoredSession } from '@/lib/api'
import { ToastHost } from '@/utils/toast'

const navItems = [
	{ label: 'Dashboard', href: '/dashboard', icon: 'LayoutDashboard' },
	{ label: 'Owners & Schools', href: '/schools', icon: 'Building2' },
	{ label: 'Partners', href: '/publishers', icon: 'Link2' },
	{ label: 'Payments', href: '/payments', icon: 'CreditCard' },
	{ label: 'Subscriptions', href: '/subscriptions', icon: 'Award' },
	{ label: 'Custom Plans', href: '/custom-plans', icon: 'FileText' },
	{ label: 'Users', href: '/users', icon: 'Users' },
	{ label: 'Settings', href: '/settings', icon: 'Settings' },
	{ label: 'Change Credentials', href: '/security', icon: 'KeyRound' },
]

interface SAUser {
  id: string
  email: string
  role: string
  school_id: string
}

export function Layout() {
  const location = useLocation()
  const navigate = useNavigate()
  const [user, setUser] = useState<SAUser | null>(null)

  useEffect(() => {
    const hydrate = async () => {
      const userJson = sessionStorage.getItem('sa_user')
      if (userJson) {
        try {
          const parsed = JSON.parse(userJson) as SAUser
          if (parsed.role === 'super_admin') {
            setUser(parsed)
            return
          }
        } catch {
          clearStoredSession()
        }
      }

      const res = await apiRequest<{
        user_id: string
        email: string
        role: string
        school_id: string
      }>('/api/auth/session')
      if (!res.ok || res.data?.role !== 'super_admin') {
        clearStoredSession()
        navigate('/login', { replace: true })
        return
      }

      const sessionUser = {
        id: res.data.user_id,
        email: res.data.email,
        role: res.data.role,
        school_id: res.data.school_id,
      }
      sessionStorage.setItem('sa_user', JSON.stringify(sessionUser))
      setUser(sessionUser)
    }
    void hydrate()
  }, [navigate])

  const handleLogout = () => {
    void apiRequest('/api/auth/logout', { method: 'POST' })
    clearStoredSession()
    navigate('/login', { replace: true })
  }

  if (!user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50">
        <div className="h-8 w-8 border-2 border-blue-200 border-t-blue-600 rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="flex h-screen bg-slate-50">
      {/* Sidebar */}
      <aside className="w-64 bg-white border-r border-slate-200 flex flex-col">
        <div className="h-14 flex items-center gap-2.5 px-5 border-b border-slate-100">
          <div className="h-8 w-8 rounded-lg overflow-hidden shadow-xs border border-slate-200 bg-white flex items-center justify-center p-0.5 shrink-0">
            <img src="/logo.jpeg" alt="Eduplexo" className="h-full w-full object-contain rounded-md" />
          </div>
          <div>
            <span className="text-sm font-extrabold text-slate-900 block leading-none">Eduplexo</span>
            <span className="text-[10px] text-slate-400 font-medium">Super Admin</span>
          </div>
        </div>

        <nav className="flex-1 p-3 space-y-1">
          {navItems.map((item) => {
            const isActive = location.pathname === item.href
            return (
              <Link
                key={item.href}
                to={item.href}
                className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all ${
                  isActive
                    ? 'bg-blue-600 text-white shadow-sm shadow-blue-600/20'
                    : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                }`}
              >
                <AppIcon name={item.icon} size={18} />
                {item.label}
              </Link>
            )
          })}
        </nav>

        <div className="p-3 border-t border-slate-100 space-y-2">
          {/* User info */}
          <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-slate-50/60">
            <div className="h-8 w-8 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 text-xs font-bold flex-shrink-0">
              {(user.email || 'U').substring(0, 2).toUpperCase()}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-xs font-semibold text-slate-900 truncate">{user.email}</p>
              <p className="text-[10px] text-slate-400 capitalize">{user.role.replace('_', ' ')}</p>
            </div>
          </div>

          <button
            onClick={handleLogout}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium text-slate-500 hover:bg-red-50 hover:text-red-600 transition-all w-full"
          >
            <AppIcon name="LogOut" size={18} />
            Logout
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-y-auto">
        <div className="p-6">
          <Outlet />
        </div>
      </main>
      <ToastHost />
    </div>
  )
}
