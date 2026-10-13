"use client";
import { Moon, Sun, Activity } from "lucide-react";
import { useEffect, useState } from "react";
import { type Incident, date } from "@/lib/api";
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
export function IncidentList({ incidents }: { incidents: Incident[] }) {
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
                {i.resolved_at ? "Resolved" : "Investigating"}
              </span>
            </div>
            <p>
              {date(i.opened_at)}
              {i.resolved_at ? " → " + date(i.resolved_at) : " · Ongoing"}
            </p>
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
