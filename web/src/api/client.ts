import type { ApiError } from "./types";

const API_BASE = "/api";

class ApiClient {
  private getToken(): string | null {
    try {
      const stored = localStorage.getItem("auth-storage");
      if (stored) {
        const parsed = JSON.parse(stored);
        return parsed?.state?.token ?? null;
      }
    } catch {
      // ignore parse errors
    }
    return null;
  }

  private async request<T>(
    path: string,
    options: RequestInit = {},
  ): Promise<T> {
    const token = this.getToken();
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((options.headers as Record<string, string>) ?? {}),
    };

    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      let errorBody: ApiError;
      try {
        errorBody = await response.json();
      } catch {
        errorBody = {
          error: "unknown",
          message: response.statusText,
          statusCode: response.status,
        };
      }

      if (response.status === 401) {
        localStorage.removeItem("auth-storage");
        window.location.href = "/login";
      }

      throw errorBody;
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return response.json();
  }

  async get<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: "GET" });
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "POST",
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "PUT",
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "PATCH",
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: "DELETE" });
  }
}

export const apiClient = new ApiClient();
