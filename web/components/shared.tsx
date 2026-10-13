"use client";
import { Moon, Sun, Activity } from "lucide-react";
import { useEffect, useState } from "react";
import { api, type Incident, type IncidentUpdate, date } from "@/lib/api";
export function Brand() {
  return (
    <a className="brand" href="/" aria-label="Uptivra home">
      <Activity size={23} strokeWidth={1.8} />
      <span>
        uptivra<span className="brand-dot">.</span>
      </span>
    </a>
  );
}
export function Theme() {
  const [dark, setDark] = useState(false);
  useEffect(() => {
    const value = localStorage.getItem("uptivra-theme");
    const d = value
      ? value === "dark"
      : matchMedia("(prefers-color-scheme: dark)").matches;
    setDark(d);
    document.documentElement.dataset.theme = d ? "dark" : "light";
  }, []);
  return (
    <button
      className="icon-button"
      aria-label="Toggle colour theme"
      onClick={() => {
        const d = !dark;
        setDark(d);
        document.documentElement.dataset.theme = d ? "dark" : "light";
        localStorage.setItem("uptivra-theme", d ? "dark" : "light");
      }}
    >
      {dark ? <Sun size={17} /> : <Moon size={17} />}
    </button>
  );
}
export function Status({ value }: { value: string }) {
  return (
    <span className={"status " + value}>
      <span className="status-dot" />
      {value}
    </span>
  );
}
export function Empty({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="empty">
      <Activity size={30} strokeWidth={1.2} />
      <h3>{title}</h3>
      <p>{children}</p>
    </div>
  );
}
export function IncidentList({
  incidents,
  onUpdate,
}: {
  incidents: Incident[];
  onUpdate?: () => Promise<void>;
}) {
  if (!incidents.length)
    return (
      <Empty title="No incidents recorded">
        Incidents appear after a monitor reaches its failure threshold.
      </Empty>
    );
  return (
    <div className="incident-list">
      {incidents.map((i) => (
        <article key={i.id}>
          <div
            className={"timeline-dot " + (i.resolved_at ? "resolved" : "open")}
          />
          <div>
            <div className="row-between">
              <h3>{i.name}</h3>
              <span className="label">
                {i.resolved_at
                  ? "Resolved"
                  : (i.updates?.filter((u) => u.status !== "update").at(-1)
                      ?.status ?? "Investigating")}
              </span>
            </div>
            <p>
              {date(i.opened_at)}
              {i.resolved_at ? " → " + date(i.resolved_at) : " · Ongoing"}
            </p>
            {!!i.updates?.length && (
              <ol
                className="incident-updates"
                aria-label={`Updates for ${i.name}`}
              >
                {i.updates.map((u) => (
                  <li key={u.id}>
                    <div className="row-between">
                      <strong>{u.status}</strong>
                      <time dateTime={u.created_at}>{date(u.created_at)}</time>
                    </div>
                    <p className="incident-message">{u.message}</p>
                  </li>
                ))}
              </ol>
            )}
            {onUpdate && (
              <IncidentUpdateForm incident={i} onUpdate={onUpdate} />
            )}
          </div>
        </article>
      ))}
    </div>
  );
}
export function ErrorMessage({ message }: { message: string }) {
  return message ? (
    <div role="alert" className="error">
      {message}
    </div>
  ) : null;
}

function IncidentUpdateForm({
  incident,
  onUpdate,
}: {
  incident: Incident;
  onUpdate: () => Promise<void>;
}) {
  const [status, setStatus] = useState<IncidentUpdate["status"]>("update");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const bytes = new TextEncoder().encode(message.trim()).length;
  return (
    <details className="incident-editor">
      <summary>Add incident update</summary>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          setNotice("");
          try {
            await api(
              `/incidents/${encodeURIComponent(incident.id)}/updates`,
              "POST",
              { status, message },
            );
            setMessage("");
            setNotice("Update published.");
            await onUpdate();
          } catch (e) {
            setError(
              e instanceof Error ? e.message : "Unable to publish update",
            );
          } finally {
            setBusy(false);
          }
        }}
      >
        <p>
          Updates on public monitors appear on the public status page. Assigned
          webhooks receive every update. Do not include credentials or private
          information.
        </p>
        <label htmlFor={`stage-${incident.id}`}>Update stage</label>
        <select
          id={`stage-${incident.id}`}
          value={status}
          onChange={(e) =>
            setStatus(e.target.value as IncidentUpdate["status"])
          }
          disabled={busy}
        >
          <option value="update">General update / fix details</option>
          <option value="investigating">Investigating</option>
          <option value="identified">Cause identified</option>
          <option value="monitoring">Fix applied / monitoring</option>
        </select>
        <label htmlFor={`message-${incident.id}`}>Incident details</label>
        <textarea
          id={`message-${incident.id}`}
          rows={4}
          required
          maxLength={2000}
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          disabled={busy}
          aria-describedby={`limit-${incident.id}`}
        />
        <p id={`limit-${incident.id}`}>
          {bytes}/2000 bytes · Monitor checks determine recovery. Updates cannot
          be edited.
        </p>
        <ErrorMessage message={error} />
        <p role="status">{notice}</p>
        <button
          className="button"
          disabled={
            busy ||
            bytes === 0 ||
            bytes > 2000 ||
            (incident.updates?.length ?? 0) >= 100
          }
        >
          {busy ? "Publishing…" : "Publish update"}
        </button>
      </form>
    </details>
  );
}
