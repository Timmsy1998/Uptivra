"use client";
import { demoMode } from "@/lib/demo-mode";
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
  const heartbeat = monitor.type === "heartbeat";
  const [credential, setCredential] = useState<{
    token: string;
    path: string;
  } | null>(null);
  const [generating, setGenerating] = useState(false);
  const [tokenVisible, setTokenVisible] = useState(false);
  const [copyNotice, setCopyNotice] = useState("");
  useEffect(() => {
    setCredential(null);
    setTokenVisible(false);
    setCopyNotice("");
  }, [monitor.id]);
  async function generateToken() {
    if (
      !window.confirm(
        "Generate a new heartbeat token? Any previous token will stop working.",
      )
    )
      return;
    setGenerating(true);
    setCredential(null);
    setTokenVisible(false);
    setCopyNotice("");
    try {
      setCredential(
        await api("/monitors/" + monitor.id + "/heartbeat-token", "POST"),
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to generate token");
    } finally {
      setGenerating(false);
    }
  }
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
      {heartbeat && (
        <section className="panel">
          <h2>Heartbeat connection</h2>
          <p>
            Send an authenticated POST every {monitor.interval_seconds} seconds.
            A signal is late after{" "}
            {monitor.interval_seconds + (monitor.grace_seconds || 0)} seconds.
            The scheduler evaluates signals every {monitor.interval_seconds}{" "}
            seconds; failure and recovery thresholds apply.
          </p>
          <button
            className="button"
            disabled={generating}
            onClick={generateToken}
          >
            {generating ? "Generating…" : "Generate / replace heartbeat token"}
          </button>
          {credential && (
            <div className="form-grid">
              <label className="wide">
                Heartbeat URL
                <input
                  readOnly
                  value={window.location.origin + credential.path}
                />
              </label>
              <label className="wide">
                Bearer token (save now; shown once)
                <input
                  className="mono"
                  readOnly
                  type={tokenVisible ? "text" : "password"}
                  autoComplete="off"
                  spellCheck={false}
                  value={credential.token}
                  onFocus={(e) => e.currentTarget.select()}
                />
              </label>
              <div className="wide token-actions">
                <button
                  className="button"
                  type="button"
                  onClick={() => setTokenVisible((visible) => !visible)}
                >
                  {tokenVisible ? "Hide token" : "Show token"}
                </button>
                <button
                  className="button"
                  type="button"
                  onClick={async () => {
                    setCopyNotice("");
                    try {
                      await navigator.clipboard.writeText(credential.token);
                      setCopyNotice("Token copied to clipboard.");
                    } catch {
                      setTokenVisible(true);
                      setCopyNotice(
                        "Clipboard unavailable. Select the visible token and copy it manually.",
                      );
                    }
                  }}
                >
                  Copy token
                </button>
              </div>
              {copyNotice && (
                <p className="wide" role="status">
                  {copyNotice}
                </p>
              )}
              <small>
                Copy the token into your bot’s environment. Leaving this monitor
                clears it from the screen.
              </small>
            </div>
          )}
        </section>
      )}
      {!heartbeat && (
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
              {demoMode
                ? "Live checks are disabled in this demo. No measurements will be collected."
                : `The scheduler checks this endpoint every ${monitor.interval_seconds} seconds when enabled.`}
            </Empty>
          )}
          <p className="footnote">
            Each bar is a recorded check. Gaps between checks have no measured
            availability.
          </p>
        </section>
      )}
      <section className="panel">
        <div className="section-heading">
          <h2>{heartbeat ? "Heartbeat evaluations" : "Recent checks"}</h2>
          <span className="label">
            {monitor.interval_seconds}s interval ·{" "}
            {heartbeat
              ? `${monitor.grace_seconds || 0}s grace`
              : `${monitor.timeout_seconds}s timeout`}
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
                  <td className="mono">
                    {ms(heartbeat ? null : c.latency_ms)}
                  </td>
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
