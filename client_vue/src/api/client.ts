export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly body: unknown,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type Query = Record<string, string | number | boolean | undefined | null>

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  query?: Query
  signal?: AbortSignal
  timeoutMs?: number
}

const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api/v1'

function buildUrl(path: string, query?: Query): string {
  if (!query) return BASE + path
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(query))
    if (v !== undefined && v !== null) params.set(k, String(v))
  const qs = params.toString()
  return qs ? `${BASE}${path}?${qs}` : BASE + path
}

async function safeJson(res: Response): Promise<unknown> {
  try {
    return await res.json()
  } catch {
    return null
  }
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, query, signal, timeoutMs = 10_000 } = opts

  const signals: AbortSignal[] = [AbortSignal.timeout(timeoutMs)]
  if (signal) signals.push(signal)

  const res = await fetch(buildUrl(path, query), {
    method,
    signal: AbortSignal.any(signals),
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (!res.ok) {
    throw new ApiError(res.status, await safeJson(res), `${method} ${path} → ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'GET' }),
  post: <T>(path: string, body?: unknown, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'POST', body }),
  patch: <T>(path: string, body?: unknown, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'PATCH', body }),
  delete: <T>(path: string, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'DELETE' }),
}
