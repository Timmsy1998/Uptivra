"use client";
import { useEffect, useState } from "react";
import { ArrowUpRight, RefreshCw } from "lucide-react";
import { api, type PublicStatus, percent, ms, date } from "@/lib/api";
import {
  Brand,
  Theme,
  Status,
  IncidentList,
  ErrorMessage,
  Empty,
} from "@/components/shared";
const summary: Record<string, string> = {
  healthy: "All services operational",
  degraded: "Some services are degraded",
  down: "Some services are unavailable",
  unknown: "Awaiting current service data",
  maintenance: "Scheduled maintenance in progress",
  paused: "Some services are paused",
};
export default function StatusPage() {
  const [data, setData] = useState<PublicStatus | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let alive = true;
    const load = () =>
      api<PublicStatus>("/public/status")
        .then((v) => {
          if (alive) {
            setData(v);
            setError("");
            document.title = v.title + " · Uptivra";
          }
        })
        .catch((e) => {
          if (alive) setError(e.message);
        });
    void load();
    const timer = setInterval(() => {
      if (!document.hidden) void load();
    }, 30000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, []);
  const currentStatus = error ? "unknown" : data?.status || "unknown";
  const groups = [
    ...new Set(data?.monitors.map((m) => m.group || "Services") || []),
  ];
  return (
    <div className="public-page">
      <header>
        <Brand />
        <div className="top-actions">
          <a href="/" className="text-button">
            Workspace
            <ArrowUpRight size={14} />
          </a>
          <Theme />
        </div>
      </header>
      <main>
        <div className="public-title">
          <span className="eyebrow">Live service health</span>
          <h1>{data?.title || "Service status"}</h1>
          <p>{data?.description || "Availability and incident history."}</p>
        </div>
        <ErrorMessage message={error} />
        {data && (
          <>
            <section className={"overall-status " + currentStatus}>
              <span className="status-dot" />
              <div>
                <h2>{summary[currentStatus] || summary.unknown}</h2>
                <p>
                  {error
                    ? "Current health unavailable. Last update " +
                      date(data.updated_at)
                    : "Updated " + date(data.updated_at)}
                </p>
              </div>
              <RefreshCw size={17} />
            </section>
            {groups.length ? (
              groups.map((group) => (
                <section className="public-services panel" key={group}>
                  <div className="section-heading">
                    <h2>{group}</h2>
                    <span className="label">
                      Recorded checks · last 24 hours
                    </span>
                  </div>
                  {data.monitors
                    .filter((m) => (m.group || "Services") === group)
                    .map((m) => (
                      <article key={m.id}>
                        <div>
                          <h3>{m.name}</h3>
                          <span className="label">
                            {m.last_check
                              ? "Last check " + date(m.last_check)
                              : "No checks recorded"}
                          </span>
                        </div>
                        <div>
                          <Status value={error ? "unknown" : m.status} />
                          <span className="mono">
                            {percent(m.uptime)} uptime
                          </span>
                          <span className="label">
                            {ms(m.latency_ms)} mean · {m.samples} samples
                          </span>
                        </div>
                      </article>
                    ))}
                </section>
              ))
            ) : (
              <section className="panel">
                <Empty title="No public services yet">
                  The administrator has not published any monitors.
                </Empty>
              </section>
            )}
            <p className="footnote">
              Availability reflects recorded checks. Missing checks and
              maintenance are unmeasured time; stale services are shown as
              unknown.
            </p>
            <section className="public-history">
              <div className="section-heading">
                <h2>Incident history</h2>
                <span className="label">Recent events</span>
              </div>
              <IncidentList incidents={data.incidents} />
            </section>
          </>
        )}
        {!data && !error && (
          <div className="empty" role="status">
            Loading service health…
          </div>
        )}
      </main>
      <footer>
        <span>
          Powered by{" "}
          <a href="https://github.com/timmsy1998/uptivra">uptivra.</a>
        </span>
        <span>Availability, measured.</span>
      </footer>
    </div>
  );
}
