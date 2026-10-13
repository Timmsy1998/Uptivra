// App Router server component. Set server-side UPTIVRA_ORIGIN; no API key needed.
const states: Record<string, string> = {
  healthy: "All services operational",
  degraded: "Some services degraded",
  down: "Service interruption",
  unknown: "Current health unavailable",
  maintenance: "Scheduled maintenance",
  paused: "Monitoring paused",
};

export default async function ServiceHealth() {
  const origin = process.env.UPTIVRA_ORIGIN;
  if (!origin) return <p role="status">{states.unknown}</p>;
  let status = "unknown";
  try {
    const response = await fetch(new URL("/api/v1/public/status", origin), {
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
      redirect: "error",
    });
    if (response.ok) {
      const data: unknown = await response.json();
      if (data && typeof data === "object" && "status" in data &&
          typeof data.status === "string" && Object.hasOwn(states, data.status)) {
        status = data.status;
      }
    }
  } catch {
    // A failed status request represents missing coverage.
  }
  return <a href={new URL("/status", origin).href} aria-label="View full service status">
    {states[status]}
  </a>;
}
