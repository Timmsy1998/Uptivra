"use client";
import { useEffect, useState } from "react";
import { api, date, type Monitor, type MaintenanceWindow } from "@/lib/api";
import { demoMode } from "@/lib/demo-mode";
import { Empty, ErrorMessage } from "./shared";
export default function Maintenance() {
  const [windows, setWindows] = useState<MaintenanceWindow[]>([]);
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const load = async () => {
    const [w, m] = await Promise.all([
      api<MaintenanceWindow[]>("/maintenance"),
      api<Monitor[]>("/monitors"),
    ]);
    setWindows(w);
    setMonitors(m);
  };
  useEffect(() => {
    if (!demoMode)
      void load()
        .catch((e) => setError(e.message))
        .finally(() => setLoading(false));
  }, []);
  if (demoMode)
    return (
      <section className="panel">
        <h1>Scheduled maintenance</h1>
        <p className="notice">
          Maintenance scheduling requires the full backend.
        </p>
      </section>
    );
  return (
    <>
      <div className="page-heading">
        <div>
          <h1>Scheduled maintenance</h1>
          <p>
            Pause checks during planned work. Public monitors show active and
            upcoming windows for the next 30 days.
          </p>
        </div>
      </div>
      <ErrorMessage message={error} />
      {notice && (
        <p className="notice" role="status">
          {notice}
        </p>
      )}
      <section className="panel settings-panel">
        <div>
          <h2>Plan a window</h2>
          <p>
            Times are entered in your local timezone. Weekly windows repeat
            every 168 hours in UTC, so local times may shift with daylight
            saving.
          </p>
          <p>
            Existing incidents remain open until monitoring resumes and confirms
            recovery. Window titles and descriptions are public when the monitor
            is public.
          </p>
        </div>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            const form = e.currentTarget;
            const f = new FormData(form);
            setBusy(true);
            setError("");
            setNotice("");
            try {
              await api("/maintenance", "POST", {
                monitor_id: f.get("monitor"),
                title: f.get("title"),
                description: f.get("description"),
                starts_at: new Date(String(f.get("start"))).toISOString(),
                ends_at: new Date(String(f.get("end"))).toISOString(),
                weekly: f.get("weekly") === "on",
              });
              form.reset();
              await load();
              setNotice("Maintenance scheduled.");
            } catch (e) {
              setError(
                e instanceof Error
                  ? e.message
                  : "Unable to schedule maintenance",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          <label htmlFor="maintenance-monitor">Monitor</label>
          <select
            id="maintenance-monitor"
            name="monitor"
            required
            disabled={loading}
          >
            <option value="">Choose a monitor</option>
            {monitors.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
                {m.public ? " (public)" : " (private)"}
              </option>
            ))}
          </select>
          <label>
            Window title
            <input name="title" required maxLength={100} />
          </label>
          <label>
            Public description
            <textarea name="description" maxLength={500} />
          </label>
          <label>
            Starts at (local time)
            <input name="start" type="datetime-local" required />
          </label>
          <label>
            Ends at (local time)
            <input name="end" type="datetime-local" required />
          </label>
          <label className="checkbox-label">
            <input name="weekly" type="checkbox" />
            Repeat weekly in UTC
          </label>
          <button
            className="button primary"
            disabled={busy || loading || !monitors.length}
          >
            {busy ? "Saving…" : "Schedule maintenance"}
          </button>
        </form>
      </section>
      <section className="panel">
        <div className="section-heading">
          <h2>Maintenance windows</h2>
          <span className="label">{windows.length} schedules</span>
        </div>
        {loading ? (
          <p role="status">Loading maintenance…</p>
        ) : windows.length ? (
          <div className="simple-list">
            {windows.map((w) => (
              <div key={w.id}>
                <div>
                  <strong>{w.title}</strong>
                  <p>
                    {w.monitor_name} · {w.weekly ? "Weekly in UTC" : "One-time"}
                  </p>
                  <p>
                    First window: {date(w.starts_at)} — {date(w.ends_at)}
                  </p>
                  <p>{w.description}</p>
                </div>
                <button
                  className="button danger"
                  disabled={busy}
                  onClick={async () => {
                    if (
                      !confirm(
                        "Cancel " +
                          w.title +
                          " and all its future occurrences?",
                      )
                    )
                      return;
                    setBusy(true);
                    setError("");
                    try {
                      await api("/maintenance/" + w.id, "DELETE");
                      await load();
                      setNotice("Maintenance cancelled.");
                    } catch (e) {
                      setError(
                        e instanceof Error ? e.message : "Cancel failed",
                      );
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  Cancel {w.title}
                </button>
              </div>
            ))}
          </div>
        ) : (
          <Empty title="No maintenance scheduled">
            Plan a future window above.
          </Empty>
        )}
      </section>
    </>
  );
}
