import { demoMode } from "./demo-mode";
export type Monitor = {
  type?: "http" | "heartbeat";
  grace_seconds?: number;
  id: string;
  name: string;
  description: string;
  group: string;
  url: string;
  interval_seconds: number;
  timeout_seconds: number;
  expected_status: number;
  contains: string;
  headers?: Record<string, string>;
  failure_threshold: number;
  recovery_threshold: number;
  enabled: boolean;
  public: boolean;
  authorized: boolean;
  maintenance_until: string | null;
  status: string;
  last_check: string | null;
  uptime: number | null;
  latency_ms: number | null;
  p95_ms: number | null;
  samples: number;
};
export type Check = {
  checked_at: string;
  ok: boolean;
  latency_ms: number;
  status_code: number;
  reason: string;
};
export type Incident = {
  id: string;
  monitor_id: string;
  name: string;
  opened_at: string;
  resolved_at: string | null;
};
export type Settings = {
  title: string;
  description: string;
  retention_days: number;
};
export type Channel = { id: string; name: string; kind: string };
export type APIKey = {
  id: string;
  name: string;
  scope: string;
  created_at: string;
};
export type PublicMonitor = Pick<
  Monitor,
  | "id"
  | "name"
  | "group"
  | "status"
  | "uptime"
  | "latency_ms"
  | "last_check"
  | "samples"
>;
export type PublicStatus = {
  title: string;
  description: string;
  status: string;
  updated_at: string;
  monitors: PublicMonitor[];
  incidents: Incident[];
};
export class APIError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  if (demoMode) {
    const { demoAPI } = await import("./demo");
    return demoAPI<T>(path, method, body);
  }
  const r = await fetch("/api/v1" + path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });
  if (!r.ok) {
    const v = await r.json().catch(() => ({ error: "Request failed" }));
    throw new APIError(v.error || "Request failed", r.status);
  }
  return r.status === 204 ? (undefined as T) : r.json();
}
export const percent = (v: number | null) =>
  v === null ? "—" : v.toFixed(2) + "%";
export const ms = (v: number | null) =>
  v === null ? "—" : Math.round(v) + " ms";
export const date = (v: string | null) =>
  v ? new Date(v).toLocaleString() : "No checks yet";
