import type {
  AdminSettings,
  AdminSettingsUpdate,
  LoginResponse,
  Streamer,
  Submission,
  SubmissionPayload,
  SuccessPayload,
  YouTubeMonitorEvent
} from "./types";

const DEFAULT_DEV_BASE = "http://localhost:8880";

const API_BASE = resolveBase();

function resolveBase(): string {
  const envBase = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.trim();
  if (envBase) {
    return trimTrailingSlash(envBase);
  }
  if (import.meta.env.DEV) {
    return DEFAULT_DEV_BASE;
  }
  return "";
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function buildURL(path: string): string {
  if (!API_BASE) {
    return path.startsWith("/") ? path : `/${path}`;
  }
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${API_BASE}${normalizedPath}`;
}

type RequestOptions = Omit<RequestInit, "headers"> & {
  headers?: HeadersInit;
};

const jsonHeaders = {
  "Content-Type": "application/json"
};

async function request<T>(
  path: string,
  options: RequestOptions = {},
  adminToken?: string | null
): Promise<T> {
  const headers = new Headers(options.headers);

  if (adminToken) {
    headers.set("Authorization", `Bearer ${adminToken}`);
  }

  const hasBody = options.body !== undefined;
  const isJSONBody =
    hasBody &&
    !(options.body instanceof FormData) &&
    !(headers.get("Content-Type") ?? "").includes("multipart/form-data");

  if (isJSONBody && !headers.has("Content-Type")) {
    Object.entries(jsonHeaders).forEach(([key, value]) => headers.set(key, value));
  }

  const method = (options.method ?? "GET").toUpperCase();
  const requestInit: RequestOptions = {
    cache: method === "GET" && options.cache === undefined ? "no-store" : options.cache,
    ...options,
    method,
    headers
  };

  const response = await fetch(buildURL(path), requestInit);

  const contentType = response.headers.get("content-type") ?? "";
  const isJSON = contentType.includes("application/json");
  const payload = isJSON ? await response.json() : await response.text();

  if (!response.ok) {
    if (isJSON && payload && typeof payload.message === "string") {
      throw new Error(payload.message);
    }
    throw new Error(
      `Request to ${path} failed with ${response.status} ${response.statusText}`
    );
  }

  return (payload as T) ?? ({} as T);
}

export async function getStreamers(): Promise<Streamer[]> {
  return request<Streamer[]>("/api/streamers", { method: "GET" });
}

export async function loginAdmin(email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>("/api/admin/login", {
    method: "POST",
    body: JSON.stringify({ email, password })
  });
}

export async function submitStreamer(payload: SubmissionPayload): Promise<SuccessPayload> {
  return request<SuccessPayload>("/api/submit-streamer", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getAdminStreamers(token: string): Promise<Streamer[]> {
  return request<Streamer[]>("/api/admin/streamers", { method: "GET" }, token);
}

export async function createAdminStreamer(
  token: string,
  payload: SubmissionPayload
): Promise<Streamer> {
  return request<Streamer>(
    "/api/admin/streamers",
    {
      method: "POST",
      body: JSON.stringify(payload)
    },
    token
  );
}

export async function updateAdminStreamer(
  token: string,
  id: string,
  payload: SubmissionPayload
): Promise<Streamer> {
  return request<Streamer>(
    `/api/admin/streamers/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload)
    },
    token
  );
}

export async function deleteAdminStreamer(token: string, id: string): Promise<void> {
  await request<void>(
    `/api/admin/streamers/${encodeURIComponent(id)}`,
    { method: "DELETE" },
    token
  );
}

export async function getSubmissions(token: string): Promise<Submission[]> {
  return request<Submission[]>("/api/admin/submissions", { method: "GET" }, token);
}

export async function moderateSubmission(
  token: string,
  action: "approve" | "reject",
  id: string
): Promise<SuccessPayload> {
  return request<SuccessPayload>(
    "/api/admin/submissions",
    {
      method: "POST",
      body: JSON.stringify({ action, id })
    },
    token
  );
}

export async function getAdminSettings(token: string): Promise<AdminSettings> {
  return request<AdminSettings>("/api/admin/settings", { method: "GET" }, token);
}

export async function updateAdminSettings(
  token: string,
  payload: AdminSettingsUpdate
): Promise<SuccessPayload> {
  return request<SuccessPayload>(
    "/api/admin/settings",
    {
      method: "PUT",
      body: JSON.stringify(payload)
    },
    token
  );
}

export async function getAdminYouTubeMonitor(token: string): Promise<YouTubeMonitorEvent[]> {
  const response = await request<{ events?: YouTubeMonitorEvent[] }>(
    "/api/admin/monitor/youtube",
    { method: "GET" },
    token
  );
  return response.events ?? [];
}
