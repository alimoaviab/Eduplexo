// Super Admin API client

// Base URL for the backend API.
//
// Development: Vite proxy handles /api/* → localhost:8080 (see vite.config.ts)
// Production (Vercel): vercel.json rewrites /api/* → https://api.eduplexo.com/api/*
//
// In BOTH cases, we use relative paths ("/api/...") so the platform's
// proxy/rewrite layer handles routing. Only set VITE_API_URL if you need
// to bypass the proxy and hit the backend directly (e.g. mobile app).
const API_BASE_URL = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '')

function resolveUrl(url: string): string {
  // Absolute URLs pass through untouched.
  if (/^https?:\/\//.test(url)) return url
  // No base URL → use relative path (works with Vite proxy + Vercel rewrites).
  if (!API_BASE_URL) return url
  // Base URL set → prefix it.
  if (url.startsWith('/')) return API_BASE_URL + url
  return `${API_BASE_URL}/${url}`
}

export function clearStoredSession() {
  sessionStorage.removeItem('sa_user')
  localStorage.removeItem('sa_token')
  localStorage.removeItem('sa_user')
}

export async function apiRequest<T = any>(
  url: string,
  options: RequestInit = {}
): Promise<{ ok: boolean; data?: T; message?: string; error?: any }> {
  const token = typeof window !== 'undefined' ? localStorage.getItem('sa_token') : null
  const authHeaders: Record<string, string> = {
    'X-App': 'super-admin',
  }
  if (token) {
    authHeaders['Authorization'] = `Bearer ${token}`
  }

  try {
    const res = await fetch(resolveUrl(url), {
      ...options,
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders,
        ...(options.headers || {}),
      },
    })

    const text = await res.text()
    let payload: any = null
    if (text) {
      try {
        payload = JSON.parse(text)
      } catch {
        payload = null
      }
    }

    if (res.status === 401) {
      const onLogin = window.location.pathname === '/login'
      if (!onLogin) {
        clearStoredSession()
        window.location.replace('/login')
      }
      return {
        ok: false,
        message: payload?.message || 'Invalid credentials',
        error: payload?.error,
      }
    }

    if (res.ok) {
      if (payload?.ok !== undefined) return payload
      return { ok: true, data: payload }
    }

    return {
      ok: false,
      message: payload?.message || payload?.error?.message || 'Request failed',
      error: payload?.error,
    }
  } catch {
    return { ok: false, message: 'Network error' }
  }
}
