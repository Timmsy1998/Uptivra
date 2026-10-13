import {
  APIError,
  type Monitor,
  type Settings,
  type PublicStatus,
} from "./api";

const storageKey = "uptivra.demo.v1";
const lifetime = 2 * 60 * 60 * 1000;
type Workspace = { expires: number; monitors: Monitor[]; settings: Settings };
const fresh = (): Workspace => ({
  expires: Date.now() + lifetime,
  monitors: [],
  settings: {
    title: "Demo service status",
    description: "Local preview in this browser tab",
    retention_days: 30,
  },
});

function read(): Workspace {
  try {
    const raw = sessionStorage.getItem(storageKey);
    if (raw) {
      const value = JSON.parse(raw) as Workspace;
      if (
        Number.isFinite(value.expires) &&
        value.expires > Date.now() &&
        value.expires <= Date.now() + lifetime &&
        Array.isArray(value.monitors) &&
        value.monitors.length <= 50 &&
        value.settings
      )
        return value;
    }
    sessionStorage.removeItem(storageKey);
    return fresh();
  } catch {
    try {
      sessionStorage.removeItem(storageKey);
      return fresh();
    } catch {
      throw new APIError(
        "Demo storage is unavailable. Allow browser session storage to use this demo.",
        503,
      );
    }
  }
}

export function resetDemo() {
  sessionStorage.removeItem(storageKey);
  window.location.assign("/");
}

function status(m: Monitor): Monitor {
  return {
    ...m,
    status: !m.enabled
      ? "paused"
      : m.maintenance_until &&
          new Date(m.maintenance_until).getTime() > Date.now()
        ? "maintenance"
        : "unknown",
    last_check: null,
    uptime: null,
    latency_ms: null,
    p95_ms: null,
    samples: 0,
  };
}

// This is a local UI workspace, not a replacement monitoring engine or auth API.
export async function demoAPI<T>(
  path: string,
  method: string,
  body?: unknown,
): Promise<T> {
  const state = read();
  state.monitors = state.monitors.map(status);
  let result: unknown;
  if (path === "/auth/session" && method === "GET") result = {};
  else if (path === "/monitors" && method === "GET") result = state.monitors;
  else if (path === "/incidents" && method === "GET") result = [];
  else if (path === "/settings" && method === "GET") result = state.settings;
  else if (path === "/settings" && method === "PUT") {
    const value = body as Settings;
    state.settings = {
      title: value.title,
      description: value.description,
      retention_days: value.retention_days,
    };
  } else if (path === "/public/status" && method === "GET") {
    result = {
      title: state.settings.title,
      description: state.settings.description,
      status: "unknown",
      updated_at: new Date().toISOString(),
      incidents: [],
      monitors: state.monitors
        .filter((m) => m.public)
        .map(
          ({
            id,
            name,
            group,
            status,
            uptime,
            latency_ms,
            last_check,
            samples,
          }) => ({
            id,
            name,
            group,
            status,
            uptime,
            latency_ms,
            last_check,
            samples,
          }),
        ),
    } satisfies PublicStatus;
  } else if (/^\/monitors\/[^/]+\/checks$/.test(path) && method === "GET")
    result = [];
  else if (
    (path === "/monitors" && method === "POST") ||
    (/^\/monitors\/[^/]+$/.test(path) && method === "PUT")
  ) {
    const value = body as Monitor;
    const id = method === "POST" ? crypto.randomUUID() : path.split("/")[2];
    if (method === "PUT" && !state.monitors.some((m) => m.id === id))
      throw new APIError("Monitor not found", 404);
    if (method === "POST" && state.monitors.length >= 50)
      throw new APIError(
        "Demo limit: 50 monitors. Delete a monitor or reset the demo.",
        400,
      );
    const url = new URL(value.url);
    if (
      !["http:", "https:"].includes(url.protocol) ||
      url.username ||
      url.password
    )
      throw new APIError(
        "Use an HTTP(S) URL without credentials. No requests will be sent.",
        400,
      );
    if (Object.keys(value.headers || {}).length)
      throw new APIError(
        "Request headers are disabled in the demo. Do not enter secrets.",
        400,
      );
    const monitor = status({
      id,
      name: value.name,
      description: value.description,
      group: value.group,
      url: value.url,
      interval_seconds: value.interval_seconds,
      timeout_seconds: value.timeout_seconds,
      expected_status: value.expected_status,
      contains: value.contains,
      failure_threshold: value.failure_threshold,
      recovery_threshold: value.recovery_threshold,
      enabled: value.enabled,
      public: value.public,
      authorized: value.authorized,
      maintenance_until: value.maintenance_until,
    } as Monitor);
    state.monitors = [...state.monitors.filter((m) => m.id !== id), monitor];
    result = monitor;
  } else if (/^\/monitors\/[^/]+$/.test(path) && method === "DELETE") {
    state.monitors = state.monitors.filter((m) => m.id !== path.split("/")[2]);
  } else
    throw new APIError(
      "This feature requires the full backend and is unavailable in demo mode.",
      403,
    );
  try {
    sessionStorage.setItem(storageKey, JSON.stringify(state));
  } catch {
    throw new APIError(
      "Unable to save the demo in this tab. Free browser storage or reset the demo.",
      503,
    );
  }
  return structuredClone(result) as T;
}
