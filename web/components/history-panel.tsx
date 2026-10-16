"use client";
import { useEffect, useState } from "react";
import { api, date, percent, type Availability } from "@/lib/api";
import { demoMode } from "@/lib/demo-mode";
import { ErrorMessage } from "./shared";

function duration(seconds: number) {
  const minutes = Math.floor(seconds / 60);
  if (minutes < 1) return Math.floor(seconds) + "s";
  if (minutes < 60) return minutes + "m";
  const hours = Math.floor(minutes / 60);
  return (
    (hours >= 24 ? Math.floor(hours / 24) + "d " : "") +
    (hours % 24) +
    "h " +
    (minutes % 60) +
    "m"
  );
}
export default function HistoryPanel({
  monitorId,
  publicView = false,
}: {
  monitorId: string;
  publicView?: boolean;
}) {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<Availability | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    setData(null);
    setError("");
    if (demoMode) return;
    let alive = true;
    const load = () =>
      api<Availability>(
        (publicView ? "/public" : "") +
          "/monitors/" +
          monitorId +
          "/availability?days=" +
          days,
      )
        .then((v) => {
          if (alive) {
            setData(v);
            setError("");
          }
        })
        .catch((e) => {
          if (alive) setError(e.message);
        });
    void load();
    const timer = setInterval(() => {
      if (!document.hidden) void load();
    }, 60000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [monitorId, days, publicView]);
  return (
    <section className="panel" aria-label="Availability history">
      <div className="section-heading">
        <h2>Availability history</h2>
        <label>
          History period{" "}
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
          >
            {[7, 30, 90].map((d) => (
              <option key={d} value={d}>
                {d} days
              </option>
            ))}
          </select>
        </label>
      </div>
      <ErrorMessage message={error} />
      {demoMode ? (
        <p>
          Historical coverage is available when connected to a monitoring
          database.
        </p>
      ) : !data ? (
        <p role="status">
          {error ? "History unavailable." : "Loading availability…"}
        </p>
      ) : (
        <>
          <p className="label">
            {error ? "Stale history · " : "As of "}
            {date(data.to)} · UTC calendar days, including today
          </p>
          <div className="stats">
            <div>
              <span>Measured availability</span>
              <strong>{percent(data.uptime)}</strong>
            </div>
            <div>
              <span>Monitoring coverage</span>
              <strong>
                {percent(
                  data.period_seconds
                    ? (data.covered_seconds / data.period_seconds) * 100
                    : null,
                )}
              </strong>
            </div>
            <div>
              <span>Measured downtime</span>
              <strong>
                {duration(data.covered_seconds - data.up_seconds)}
              </strong>
            </div>
            <div>
              <span>Missing coverage</span>
              <strong>{duration(data.missing_seconds)}</strong>
            </div>
          </div>
          <div className="availability-bars" aria-label="Daily uptime bars">
            {data.daily.map((d) => {
              const denominator = d.period_seconds || 1;
              const label = `${d.date}: ${d.uptime === null ? "No measured coverage" : percent(d.uptime) + " measured uptime"}; ${duration(d.missing_seconds)} missing coverage`;
              return (
                <div
                  className="availability-day"
                  key={d.date}
                  role="img"
                  tabIndex={0}
                  title={label}
                  aria-label={label}
                >
                  <span
                    className="availability-up"
                    style={{ height: `${(100 * d.up_seconds) / denominator}%` }}
                  />
                  <span
                    className="availability-down"
                    style={{
                      height: `${(100 * (d.covered_seconds - d.up_seconds)) / denominator}%`,
                    }}
                  />
                  <span
                    className="availability-missing"
                    style={{
                      height: `${d.period_seconds ? (100 * d.missing_seconds) / denominator : 100}%`,
                    }}
                  />
                </div>
              );
            })}
          </div>
          <p className="label">
            {data.daily[0]?.date} — {data.daily.at(-1)?.date} · Green: up · Red:
            down · Gray: unmeasured
          </p>
          {data.uptime === null && <p>No measured coverage in this period.</p>}
          <p className="footnote">
            Each check covers at most its recorded interval. Gaps count as
            missing coverage, separately from downtime. Retention is{" "}
            {data.retention_days} days; older or pre-upgrade samples without an
            interval cannot establish coverage.
          </p>
          <h3>Incident durations</h3>
          {data.incidents.length ? (
            <div className="simple-list">
              {data.incidents.map((i) => (
                <div key={i.id}>
                  <div>
                    <p>
                      {date(i.opened_at)} —{" "}
                      {i.resolved_at ? date(i.resolved_at) : "Ongoing"}
                    </p>
                    <strong>{duration(i.duration_seconds)}</strong>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p>No incidents recorded in this period.</p>
          )}
          <p className="label">
            Latest 100 overlapping incidents. Durations include time before this
            view and ongoing time through the last update.
          </p>
        </>
      )}
    </section>
  );
}
