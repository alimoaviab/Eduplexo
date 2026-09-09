// Partner Portal API Client

// Base URL for the backend API.
//
// Development: Vite proxy handles /api/* → localhost:8080 (see vite.config.ts)
// Production (Vercel): vercel.json rewrites /api/* → https://api.eduplexo.com/api/*
//
// If VITE_API_URL is explicitly set (e.g. in Vercel environment variables or .env),
// it will prefix API calls directly.
const API_BASE_URL = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '')

export function resolveUrl(url: string): string {
  if (/^https?:\/\//.test(url)) return url
  if (!API_BASE_URL) return url
  if (url.startsWith('/')) return `${API_BASE_URL}${url}`
  return `${API_BASE_URL}/${url}`
}

export interface ApiResponse<T = any> {
  ok: boolean
  data?: T
  message?: string
  status?: number
}

export async function apiRequest<T = any>(
  endpoint: string,
  options: RequestInit = {}
): Promise<ApiResponse<T>> {
  const targetUrl = resolveUrl(endpoint)

  // Attach publisher bearer token automatically if present
  const token = typeof window !== 'undefined' ? localStorage.getItem('eduplexo_publisher_token') : null
  const authHeaders: Record<string, string> = {}
  if (token) {
    authHeaders['Authorization'] = `Bearer ${token}`
  }

  try {
    const res = await fetch(targetUrl, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders,
        ...(options.headers || {}),
      },
    })

    const contentType = res.headers.get('content-type') || ''
    let body: any = null

    if (contentType.includes('application/json')) {
      try {
        body = await res.json()
      } catch {
        body = null
      }
    } else {
      const rawText = await res.text()
      if (res.status === 404) {
        return {
          ok: false,
          status: 404,
          message:
            'API endpoint not found (404). Please ensure the Vercel rewrite or VITE_API_URL points to https://api.eduplexo.com.',
        }
      }
      return {
        ok: false,
        status: res.status,
        message: `Server returned unexpected response (${res.status}): ${rawText.slice(0, 120)}`,
      }
    }

    if (res.ok && body?.ok) {
      return {
        ok: true,
        data: body.data,
        message: body.message,
        status: res.status,
      }
    }

    return {
      ok: false,
      status: res.status,
      message: body?.message || `Request failed with status ${res.status}`,
    }
  } catch (err: any) {
    return {
      ok: false,
      message:
        err?.message ||
        'Unable to connect to the partner server. Please check your internet connection and try again.',
    }
  }
}
