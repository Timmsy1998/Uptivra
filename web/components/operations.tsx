"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { demoMode } from "@/lib/demo-mode";
import { Empty, ErrorMessage } from "./shared";
type Delivery = {
  id: string;
  channel_name: string;
  state: string;
  event: string;
  attempts: number;
  created_at: string | null;
  last_attempt_at: string | null;
  next_attempt: string;
  outcome: string;
};
type Health = {
  last_run: {
    started_at: string;
    finished_at: string | null;
    success: boolean | null;
    claimed: number;
    recorded: number;
    delivered: number;
  } | null;
  overdue_monitors: number;
  pending_deliveries: number;
  failed_deliveries: number;
  oldest_pending_at: string | null;
  observed_at: string;
};
const date = (s: string | null) =>
  s ? new Date(s).toLocaleString() : "Not recorded";
export default function Operations() {
  const [health, setHealth] = useState<Health | null>(null);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [state, setState] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  useEffect(() => {
    if (demoMode) return;
    let active = true;
    async function load() {
      try {
        const [h, d] = await Promise.all([
          api<Health>("/scheduler/health"),
          api<Delivery[]>("/deliveries?state=" + state),
        ]);
        if (active) {
          setHealth(h);
          setDeliveries(d);
          setError("");
        }
      } catch (e) {
        if (active)
          setError(
            e instanceof Error ? e.message : "Unable to load operations",
          );
      } finally {
        if (active) setLoading(false);
      }
    }
    setLoading(true);
    void load();
    const timer = setInterval(load, 30000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [state]);
  if (demoMode)
    return (
      <section className="panel">
        <h1>Delivery & scheduler</h1>
        <p className="notice">
          Live scheduler and notification diagnostics require the full backend.
        </p>
      </section>
    );
  const run = health?.last_run;
  return (
    <>
      <div className="page-heading">
        <div>
          <h1>Delivery & scheduler</h1>
          <p>
            Monitoring infrastructure and the latest 100 notifications.
            Refreshes every 30 seconds.
          </p>
        </div>
      </div>
      <ErrorMessage message={error} />
      {error && health && (
        <p className="notice">
          Showing previously loaded data from {date(health.observed_at)}.
        </p>
      )}
      <section className="panel">
        <div className="section-heading">
          <h2>Scheduler health</h2>
        </div>
        {health ? (
          <>
            <p>
              <strong>
                {!run
                  ? "No scheduler run recorded"
                  : !run.finished_at
                    ? "Run unfinished"
                    : run.success
                      ? "Last run completed"
                      : "Last run failed"}
              </strong>
              {run && (
                <>
                  {" "}
                  · Started {date(run.started_at)} · Finished{" "}
                  {date(run.finished_at)}
                </>
              )}
            </p>
            {run && Date.now() - Date.parse(run.started_at) > 180000 && (
              <p className="notice">
                No recent scheduler run. Check your cron configuration and
                hosting logs.
              </p>
            )}
            {run && (
              <p>
                {run.claimed} monitors claimed · {run.recorded} checks recorded
                · {run.delivered} notifications delivered
              </p>
            )}
            <div className="operations-counts">
              <p>
                <strong>{health.overdue_monitors}</strong> overdue monitors
              </p>
              <p>
                <strong>{health.pending_deliveries}</strong> pending
                notifications
              </p>
              <p>
                <strong>{health.failed_deliveries}</strong> failed notifications
              </p>
            </div>
            <p>
              Overdue means more than 90 seconds past the next check, excluding
              paused monitors and active maintenance. Oldest pending
              notification: {date(health.oldest_pending_at)}.
            </p>
            <small>
              Observed {date(health.observed_at)}. An unfinished run can
              indicate interruption or timeout. Counts include retained records
              only.
            </small>
          </>
        ) : (
          <p role="status">
            {loading
              ? "Loading scheduler health…"
              : "Scheduler health unavailable."}
          </p>
        )}
      </section>
      <section className="panel">
        <div className="section-heading">
          <h2>Alert deliveries</h2>
          <label>
            Delivery state
            <select value={state} onChange={(e) => setState(e.target.value)}>
              <option value="">All states</option>
              <option value="pending">Pending</option>
              <option value="delivered">Delivered</option>
              <option value="failed">Failed</option>
            </select>
          </label>
        </div>
        {loading ? (
          <p role="status">Loading deliveries…</p>
        ) : deliveries.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Channel / event</th>
                  <th>State</th>
                  <th>Attempts</th>
                  <th>Last attempt</th>
                  <th>Outcome</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {deliveries.map((d) => (
                  <tr key={d.id}>
                    <td>
                      {d.channel_name}
                      <small className="delivery-event">{d.event}</small>
                    </td>
                    <td>{d.state}</td>
                    <td>{d.attempts} / 8</td>
                    <td>
                      {date(d.last_attempt_at)}
                      {d.state === "pending" && (
                        <small className="delivery-event">
                          Next: {date(d.next_attempt)}
                        </small>
                      )}
                    </td>
                    <td>{d.outcome || "Awaiting scheduler"}</td>
                    <td>
                      {d.state === "failed" && (
                        <button
                          className="button"
                          disabled={!!busy}
                          onClick={async () => {
                            setBusy(d.id);
                            setError("");
                            try {
                              await api(
                                "/deliveries/" + d.id + "/retry",
                                "POST",
                              );
                              setDeliveries(
                                await api<Delivery[]>(
                                  "/deliveries?state=" + state,
                                ),
                              );
                              setHealth(await api<Health>("/scheduler/health"));
                            } catch (e) {
                              setError(
                                e instanceof Error ? e.message : "Retry failed",
                              );
                            } finally {
                              setBusy("");
                            }
                          }}
                        >
                          {busy === d.id ? "Queueing…" : "Retry"}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty title="No matching deliveries">
            Test a channel or wait for an incident notification.
          </Empty>
        )}
        <p>
          Tests and retries are queued for the next scheduler run. Outcomes
          contain safe HTTP codes or failure categories; destinations, secrets
          and response bodies are hidden.
        </p>
      </section>
    </>
  );
}
