// API client with interceptors for Bearer auth and error handling

import type { ApiError } from "./types"
import { useAuthStore } from "./auth-store"
import { useUIStore } from "./ui-store"
import { mapRoute, API_BASE_URL } from "./routes"

const API_BASE = API_BASE_URL

export class ApiClient {
  private baseUrl: string
  private getToken: () => string | null
  private onUnauthorized: () => void
  private onLeaderRedirect: (leaderUrl: string) => void

  constructor(
    baseUrl: string = API_BASE,
    getToken: () => string | null,
    onUnauthorized: () => void,
    onLeaderRedirect: (leaderUrl: string) => void,
  ) {
    this.baseUrl = baseUrl
    this.getToken = getToken
    this.onUnauthorized = onUnauthorized
    this.onLeaderRedirect = onLeaderRedirect
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const token = this.getToken()
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers["Authorization"] = `Bearer ${token}`
    }

    // Map route from V1 to V2 if needed
    const mappedEndpoint = mapRoute(endpoint)
    const url = `${this.baseUrl}${mappedEndpoint}`

    try {
      const response = await fetch(url, {
        ...options,
        headers,
        credentials: "include", // Required for cross-site cookies (localhost:3000 -> localhost:8080)
      })

      // Handle 401 Unauthorized
      if (response.status === 401) {
        this.onUnauthorized()
        throw new Error("Unauthorized")
      }

      // Handle 409 Not Leader (follower redirect)
      if (response.status === 409) {
        const leaderUrl = response.headers.get("X-Leader-URL")
        const leader = response.headers.get("X-Leader")
        if (leaderUrl) {
          this.onLeaderRedirect(leaderUrl)
        }
        const error: ApiError = {
          error: "not_leader",
          error_description: `This node is a follower. Leader: ${leader}`,
          status: 409,
        }
        throw error
      }

      // Handle 429 Rate Limited
      if (response.status === 429) {
        const retryAfter = response.headers.get("Retry-After")
        const error: ApiError = {
          error: "rate_limited",
          error_description: `Too many requests. Retry after ${retryAfter}s`,
          status: 429,
        }
        throw error
      }

      // Handle other error statuses
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        const error: ApiError = {
          // Support both OAuth2 style (error) and v2 API style (code)
          error: errorData.code || errorData.error || "server_error",
          error_description: errorData.detail || errorData.message || errorData.error_description || response.statusText,
          status: response.status,
        }
        throw error
      }

      // Handle 204 No Content
      if (response.status === 204) {
        return {} as T
      }

      return await response.json()
    } catch (error) {
      if (error instanceof Error && error.message === "Unauthorized") {
        throw error
      }
      if ((error as ApiError).error) {
        throw error
      }
      throw new Error("Network error")
    }
  }

  async get<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    return this.request<T>(endpoint, { method: "GET", ...options })
  }

  async getWithHeaders<T>(endpoint: string, options: RequestInit = {}): Promise<{ data: T; headers: Headers }> {
    const token = this.getToken()
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers["Authorization"] = `Bearer ${token}`
    }

    // Map route from V1 to V2 if needed
    const mappedEndpoint = mapRoute(endpoint)
    const url = `${this.baseUrl}${mappedEndpoint}`

    try {
      const response = await fetch(url, {
        method: "GET",
        ...options,
        headers,
      })

      if (response.status === 401) {
        this.onUnauthorized()
        throw new Error("Unauthorized")
      }

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        const error: ApiError = {
          // Support both OAuth2 style (error) and v2 API style (code)
          error: errorData.code || errorData.error || "server_error",
          error_description: errorData.detail || errorData.message || errorData.error_description || response.statusText,
          status: response.status,
        }
        throw error
      }

      const data = await response.json()
      return { data, headers: response.headers }
    } catch (error) {
      throw error
    }
  }

  async post<T>(endpoint: string, data?: any, options: RequestInit = {}): Promise<T> {
    return this.request<T>(endpoint, {
      method: "POST",
      body: data ? JSON.stringify(data) : undefined,
      ...options,
    })
  }

  // Form-URL-Encoded POST (e.g., OAuth2 introspect/revoke) with optional custom headers (Basic auth)
  async postForm<T>(
    endpoint: string,
    form: URLSearchParams,
    headers: Record<string, string> = {},
  ): Promise<T> {
    return this.request<T>(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded", ...headers },
      body: form.toString(),
    })
  }

  async put<T>(endpoint: string, data?: any, ifMatch?: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = { ...(options.headers as Record<string, string>) }
    if (ifMatch) {
      headers["If-Match"] = ifMatch
    }
    return this.request<T>(endpoint, {
      method: "PUT",
      body: data ? JSON.stringify(data) : undefined,
      headers,
      ...options,
    })
  }

  async delete<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    return this.request<T>(endpoint, { method: "DELETE", ...options })
  }

  setBaseUrl(url: string) {
    this.baseUrl = url
  }

  getBaseUrl(): string {
    return this.baseUrl
  }
}

export const api = new ApiClient(
  API_BASE,
  () => useAuthStore.getState().token,
  () => {
    ; (useAuthStore.getState() as any).logout?.()
    if (typeof window !== "undefined") {
      window.location.href = "/login"
    }
  },
  (leaderUrl: string) => {
    ; (useUIStore.getState() as any).setLeaderUrl?.(leaderUrl)
  },
)
