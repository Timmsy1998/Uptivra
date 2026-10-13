"use client";
import { useEffect, useState } from "react";
import { ArrowLeft, Settings2 } from "lucide-react";
import { api, type Monitor, type Check, date, ms, percent } from "@/lib/api";
import { Empty, ErrorMessage, Status } from "./shared";
export default function MonitorDetail({
  monitor,
  onBack,
  onEdit,
}: {
  monitor: Monitor;
  onBack: () => void;
  onEdit: () => void;
}) {
  const [checks, setChecks] = useState<Check[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api<Check[]>("/monitors/" + monitor.id + "/checks")
      .then(setChecks)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [monitor.id, monitor.last_check]);
  const chronological = [...checks].reverse();
  const max = Math.max(1, ...checks.map((c) => c.latency_ms));
  return (
    <>
      <button className="text-button" onClick={onBack}>
        <ArrowLeft size={15} />
        All monitors
      </button>
      <div className="page-heading">
        <div>
          <span className="eyebrow">{monitor.group || "Ungrouped"}</span>
          <h1>{monitor.name}</h1>
          <p>{monitor.description || monitor.url}</p>
        </div>
        <button className="button" onClick={onEdit}>
          <Settings2 size={15} />
          Edit monitor
        </button>
      </div>
      <ErrorMessage message={error} />
      <div className="stats">
        <div>
          <span>Current health</span>
          <Status value={monitor.status} />
        </div>
        <div>
          <span>Measured uptime · 24h</span>
          <strong>{percent(monitor.uptime)}</strong>
        </div>
        <div>
          <span>Mean response · 24h</span>
          <strong>{ms(monitor.latency_ms)}</strong>
        </div>
        <div>
          <span>95th percentile · 24h</span>
          <strong>{ms(monitor.p95_ms)}</strong>
        </div>
      </div>
      <section className="panel">
        <div className="section-heading">
          <h2>Response time</h2>
          <span className="label">
            Last {checks.length} checks · milliseconds
          </span>
        </div>
        {checks.length ? (
          <>
            <div
              className="latency-chart"
              role="img"
              aria-label="Response time of recent checks, ordered oldest to newest"
            >
              {chronological.map((c) => (
                <div
                  key={c.checked_at}
                  className={c.ok ? "ok" : "fail"}
                  style={{
                    height: Math.max(2, (c.latency_ms / max) * 100) + "%",
                  }}
                  title={
                    date(c.checked_at) +
                    " · " +
                    ms(c.latency_ms) +
                    (c.ok ? " · healthy" : " · " + c.reason)
                  }
                />
              ))}
            </div>
            <div className="chart-axis">
              <span>{date(chronological[0].checked_at)}</span>
              <span>{date(checks[0].checked_at)}</span>
            </div>
          </>
        ) : (
          <Empty
            title={loading ? "Loading checks…" : "Awaiting the first check"}
          >
            The scheduler checks this endpoint every {monitor.interval_seconds}{" "}
            seconds when enabled.
          </Empty>
        )}
        <p className="footnote">
          Each bar is a recorded check. Gaps between checks have no measured
          availability.
        </p>
      </section>
      <section className="panel">
        <div className="section-heading">
          <h2>Recent checks</h2>
          <span className="label">
            {monitor.interval_seconds}s interval · {monitor.timeout_seconds}s
            timeout
          </span>
        </div>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Checked at</th>
                <th>Result</th>
                <th>Response time</th>
                <th>HTTP</th>
                <th>Reason</th>
              </tr>
            </thead>
            <tbody>
              {checks.map((c) => (
                <tr key={c.checked_at}>
                  <td className="mono">{date(c.checked_at)}</td>
                  <td>
                    <Status value={c.ok ? "healthy" : "down"} />
                  </td>
                  <td className="mono">{ms(c.latency_ms)}</td>
                  <td className="mono">{c.status_code || "—"}</td>
                  <td>{c.reason || "Expected response"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}
