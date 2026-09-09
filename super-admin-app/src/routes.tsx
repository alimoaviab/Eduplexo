import { lazy, Suspense, type ComponentType } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'

function lazyPage(importFn: () => Promise<any>, exportName: string) {
  const LazyComponent = lazy(async () => {
    const mod = await importFn()
    return { default: (mod as Record<string, ComponentType>)[exportName] }
  })

  return (
    <Suspense fallback={<div className="p-8 text-sm text-slate-500">Loading...</div>}>
      <LazyComponent />
    </Suspense>
  )
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <Layout />,
    children: [
      { path: '/', element: <Navigate to="/dashboard" replace /> },
      { path: '/dashboard', element: lazyPage(() => import('./pages/DashboardPage'), 'DashboardPage') },
      { path: '/schools', element: lazyPage(() => import('./pages/SchoolsPage'), 'SchoolsPage') },
      { path: '/schools/:id', element: lazyPage(() => import('./pages/SchoolDetailPage'), 'SchoolDetailPage') },
      { path: '/users', element: lazyPage(() => import('./pages/UsersPage'), 'UsersPage') },
      { path: '/payments', element: lazyPage(() => import('./pages/PaymentsPage'), 'PaymentsPage') },
      { path: '/subscriptions', element: lazyPage(() => import('./pages/SubscriptionsPage'), 'SubscriptionsPage') },
      { path: '/custom-plans', element: lazyPage(() => import('./pages/CustomPlansPage'), 'CustomPlansPage') },
      { path: '/publishers', element: lazyPage(() => import('./pages/PublishersPage'), 'PublishersPage') },
      { path: '/publishers/:id', element: lazyPage(() => import('./pages/PublisherDetailPage'), 'PublisherDetailPage') },
      { path: '/settings', element: lazyPage(() => import('./pages/SettingsPage'), 'SettingsPage') },
      { path: '/security', element: lazyPage(() => import('./pages/SecurityPage'), 'SecurityPage') },
      { path: '/packages', element: <Navigate to="/dashboard" replace /> },
      { path: '/ai-usage', element: <Navigate to="/dashboard" replace /> },
      { path: '/moderation', element: <Navigate to="/dashboard" replace /> },
      { path: '/question-bank', element: <Navigate to="/dashboard" replace /> },
      { path: '/hierarchy', element: <Navigate to="/dashboard" replace /> },
      { path: '/csv-imports', element: <Navigate to="/dashboard" replace /> },
    ],
  },
])
