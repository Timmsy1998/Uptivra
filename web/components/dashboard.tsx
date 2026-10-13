"use client";
import { demoMode } from "@/lib/demo-mode";
import { useCallback, useEffect, useState } from "react";
import {
  Activity,
  ArrowUpRight,
  ChevronRight,
  Clock3,
  ExternalLink,
  LayoutList,
  LogOut,
  Pause,
  Play,
  Plus,
  Search,
  Settings2,
  Trash2,
  Webhook,
  RefreshCw,
} from "lucide-react";
import {
  api,
  APIError,
  type Monitor,
  type Incident,
  percent,
  ms,
  date,
} from "@/lib/api";
import {
  Brand,
  Theme,
  Status,
  Empty,
  ErrorMessage,
  IncidentList,
} from "./shared";
import MonitorEditor from "./monitor-editor";
import MonitorDetail from "./monitor-detail";
import SettingsPanel from "./settings";
import Channels from "./channels";
import Login from "./login";
import DemoBanner from "./demo-banner";
type View = "monitors" | "incidents" | "channels" | "settings";
export default function Dashboard() {
  const [auth, setAuth] = useState<boolean | null>(null);
  const [view, setView] = useState<View>("monitors");
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false);
  const [editor, setEditor] = useState<Monitor | "new" | null>(null);
  const [detail, setDetail] = useState<string | null>(null);
  const [refreshed, setRefreshed] = useState<string | null>(null);
  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [ms, is] = await Promise.all([
        api<Monitor[]>("/monitors"),
        api<Incident[]>("/incidents"),
      ]);
      setMonitors(ms);
      setIncidents(is);
      setRefreshed(new Date().toLocaleTimeString());
      setError("");
    } catch (e) {
      if (e instanceof APIError && e.status === 401) setAuth(false);
      else
        setError(
          e instanceof Error ? e.message : "Unable to load monitoring data",
        );
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    api("/auth/session")
      .then(() => setAuth(true))
      .catch((e) => {
        setAuth(false);
        if (e.status !== 401) setError(e.message);
      });
  }, []);
  useEffect(() => {
    if (auth) {
      void refresh();
      const timer = setInterval(() => {
        if (!document.hidden) void refresh();
      }, 30000);
      return () => clearInterval(timer);
    }
  }, [auth, refresh]);
  async function action(fn: () => Promise<void>, message: string) {
    setError("");
    try {
      await fn();
      setNotice(message);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Request failed");
    }
  }
  if (auth === null)
    return (
      <main className="boot">
        <Brand />
        <p>Connecting to monitoring service…</p>
      </main>
    );
  if (!auth && demoMode)
    return (
      <main className="boot">
        <Brand />
        <DemoBanner />
        <ErrorMessage message={error} />
        <button className="button" onClick={() => window.location.reload()}>
          Retry demo
        </button>
      </main>
    );
  if (!auth)
    return (
      <Login
        error={error}
        onLogin={() => {
          setError("");
          setAuth(true);
        }}
      />
    );
  const nav = [
    { id: "monitors", label: "Monitors", icon: LayoutList },
    { id: "incidents", label: "Incidents", icon: Clock3 },
    { id: "channels", label: "Channels", icon: Webhook },
    { id: "settings", label: "Settings & access", icon: Settings2 },
  ] as const;
  const healthy = monitors.filter((m) => m.status === "healthy").length;
  const down = monitors.filter(
    (m) => m.status === "down" || m.status === "degraded",
  ).length;
  const active = incidents.filter((i) => !i.resolved_at).length;
  const visible = monitors.filter(
    (m) =>
      (filter === "all" || m.status === filter) &&
      [m.name, m.group, m.url].some((v) =>
        v.toLowerCase().includes(query.toLowerCase()),
      ),
  );
  const selected = monitors.find((m) => m.id === detail);
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand />
        <div className="workspace">
          <span className="workspace-mark">U</span>
          <div>
            <strong>Workspace</strong>
            <small>Administrator</small>
          </div>
          <span className="label">01</span>
        </div>
        <span className="nav-label">Observe</span>
        <nav aria-label="Administration">
          {nav.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className={view === id ? "active" : ""}
              onClick={() => {
                setView(id);
                setDetail(null);
                setNotice("");
              }}
            >
              <Icon size={17} />
              {label}
              {id === "incidents" && active > 0 && (
                <span className="count">{active}</span>
              )}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <a
            href="/status"
            target={demoMode ? "_self" : "_blank"}
            rel="noreferrer"
          >
            <ArrowUpRight size={17} />
            Public status page
          </a>
          <div className="sidebar-footer">
            <span className="label">UPTIVRA / v0.1</span>
            <Theme />
          </div>
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="breadcrumb">
            Workspace <ChevronRight size={13} />
            <span>{nav.find((n) => n.id === view)?.label}</span>
          </div>
          <div className="top-actions">
            <span className="live-indicator">
              <i />
              {demoMode ? "Local demo" : "Live data"}
            </span>
            <button
              className="icon-button"
              aria-label="Refresh data"
              disabled={loading}
              onClick={() => void refresh()}
            >
              <RefreshCw size={16} className={loading ? "spin" : ""} />
            </button>
            {!demoMode && (
              <button
                className="icon-button"
                aria-label="Sign out"
                onClick={() =>
                  void action(async () => {
                    await api("/auth/logout", "POST");
                    setAuth(false);
                  }, "Signed out.")
                }
              >
                <LogOut size={16} />
              </button>
            )}
          </div>
        </header>
        <main className="content">
          <DemoBanner />
          <ErrorMessage message={error} />
          {notice && (
            <div className="notice" role="status">
              {notice}
            </div>
          )}
          {view === "monitors" &&
            (selected ? (
              <MonitorDetail
                monitor={selected}
                onBack={() => setDetail(null)}
                onEdit={() => setEditor(selected)}
              />
            ) : (
              <>
                <div className="page-heading">
                  <div>
                    <span className="eyebrow">Service observability</span>
                    <h1>
                      Every endpoint.
                      <br className="mobile-break" /> A clear picture.
                    </h1>
                    <p>
                      Availability, response times and the moments that need
                      attention.
                    </p>
                  </div>
                  <button
                    className="button primary"
                    onClick={() => setEditor("new")}
                  >
                    <Plus size={16} />
                    New monitor
                  </button>
                </div>
                <div className="stats">
                  <div>
                    <span>Services monitored</span>
                    <strong>
                      {monitors.length.toString().padStart(2, "0")}
                      <small>configured</small>
                    </strong>
                  </div>
                  <div>
                    <span>Healthy now</span>
                    <strong className="green">
                      {healthy.toString().padStart(2, "0")}
                      <small>responding as expected</small>
                    </strong>
                  </div>
                  <div>
                    <span>Need attention</span>
                    <strong className={down ? "red" : ""}>
                      {down.toString().padStart(2, "0")}
                      <small>down or degraded</small>
                    </strong>
                  </div>
                  <div>
                    <span>Open incidents</span>
                    <strong>
                      {active.toString().padStart(2, "0")}
                      <small>awaiting recovery</small>
                    </strong>
                  </div>
                </div>
                <section className="panel monitor-panel">
                  <div className="section-heading">
                    <div className="section-title">
                      <h2>Monitors</h2>
                      <span className="number-tag">{monitors.length}</span>
                    </div>
                    <span className="label">
                      {refreshed ? "Updated " + refreshed : "Loading…"}
                    </span>
                  </div>
                  <div className="toolbar">
                    <label className="search">
                      <Search size={16} />
                      <input
                        aria-label="Search monitors"
                        placeholder="Search name, group or endpoint…"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                      />
                    </label>
                    <select
                      aria-label="Filter by status"
                      value={filter}
                      onChange={(e) => setFilter(e.target.value)}
                    >
                      {[
                        "all",
                        "healthy",
                        "degraded",
                        "down",
                        "unknown",
                        "paused",
                        "maintenance",
                      ].map((s) => (
                        <option key={s} value={s}>
                          {s === "all"
                            ? "All states"
                            : s.charAt(0).toUpperCase() + s.slice(1)}
                        </option>
                      ))}
                    </select>
                  </div>
                  {!monitors.length ? (
                    <Empty
                      title={
                        loading
                          ? "Loading monitors…"
                          : "Your first endpoint starts here"
                      }
                    >
                      {demoMode
                        ? "Add an example endpoint to try the configuration workflow. No requests will be sent."
                        : "Add a service you own. Uptivra will record checks, track incidents and notify your team."}
                      <button
                        className="button"
                        onClick={() => setEditor("new")}
                      >
                        <Plus size={15} />
                        Create a monitor
                      </button>
                    </Empty>
                  ) : !visible.length ? (
                    <Empty title="No matching monitors">
                      Try a different search or status filter.
                    </Empty>
                  ) : (
                    <div className="table-scroll">
                      <table className="monitor-table">
                        <thead>
                          <tr>
                            <th>Service / endpoint</th>
                            <th>Health</th>
                            <th>
                              Uptime <span>24h</span>
                            </th>
                            <th>
                              Response <span>24h mean</span>
                            </th>
                            <th>Last check</th>
                            <th>
                              <span className="sr-only">Actions</span>
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {visible.map((m) => (
                            <tr key={m.id}>
                              <td>
                                <button
                                  className="service-link"
                                  onClick={() => setDetail(m.id)}
                                >
                                  <span className="service-icon">
                                    <Activity size={17} />
                                  </span>
                                  <span>
                                    <strong>{m.name}</strong>
                                    <small>
                                      {m.group ? m.group + " · " : ""}
                                      {m.type === "heartbeat"
                                        ? "Heartbeat"
                                        : m.url}
                                    </small>
                                  </span>
                                </button>
                              </td>
                              <td>
                                <Status value={m.status} />
                              </td>
                              <td className="mono">
                                {percent(m.uptime)}
                                <small>
                                  {m.samples
                                    ? m.samples + " samples"
                                    : "Awaiting data"}
                                </small>
                              </td>
                              <td className="mono">{ms(m.latency_ms)}</td>
                              <td
                                className="last-check"
                                title={date(m.last_check)}
                              >
                                {m.last_check
                                  ? new Date(m.last_check).toLocaleTimeString()
                                  : "—"}
                              </td>
                              <td>
                                <div className="row-actions">
                                  <button
                                    className="icon-button"
                                    aria-label={"Edit " + m.name}
                                    onClick={() => setEditor(m)}
                                  >
                                    <Settings2 size={15} />
                                  </button>
                                  <button
                                    className="icon-button"
                                    aria-label={
                                      (m.enabled ? "Pause " : "Resume ") +
                                      m.name
                                    }
                                    onClick={() =>
                                      void action(
                                        () =>
                                          api("/monitors/" + m.id, "PUT", {
                                            ...m,
                                            enabled: !m.enabled,
                                          }),
                                        m.enabled
                                          ? "Monitor paused."
                                          : "Monitor resumed.",
                                      )
                                    }
                                  >
                                    {m.enabled ? (
                                      <Pause size={15} />
                                    ) : (
                                      <Play size={15} />
                                    )}
                                  </button>
                                  <button
                                    className="icon-button danger"
                                    aria-label={"Delete " + m.name}
                                    onClick={() => {
                                      if (
                                        confirm(
                                          "Delete " +
                                            m.name +
                                            "? Its checks and incidents will be permanently removed.",
                                        )
                                      )
                                        void action(
                                          () =>
                                            api("/monitors/" + m.id, "DELETE"),
                                          "Monitor deleted.",
                                        );
                                    }}
                                  >
                                    <Trash2 size={15} />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                  <div className="panel-footer">
                    <span>
                      {demoMode
                        ? "Configuration preview · checks disabled"
                        : "HTTP & HTTPS · scheduled checks"}
                    </span>
                    <span>Uptime is measured from recorded checks.</span>
                  </div>
                </section>
                <div className="lower-grid">
                  <section className="panel">
                    <div className="section-heading">
                      <h2>Recent incidents</h2>
                      <button
                        className="text-button"
                        onClick={() => setView("incidents")}
                      >
                        View history
                        <ChevronRight size={14} />
                      </button>
                    </div>
                    <IncidentList incidents={incidents.slice(0, 3)} />
                  </section>
                  <section className="status-callout">
                    <div className="callout-icon">
                      <ExternalLink size={19} />
                    </div>
                    <span className="eyebrow">Keep everyone informed</span>
                    <h2>A shared view of service health.</h2>
                    <p>
                      Publish selected services and incident history. Private
                      endpoints stay in your workspace.
                    </p>
                    <a
                      className="button"
                      href="/status"
                      target={demoMode ? "_self" : "_blank"}
                      rel="noreferrer"
                    >
                      Open status page
                      <ArrowUpRight size={15} />
                    </a>
                  </section>
                </div>
                <p className="footnote">
                  No result is not a success. Services become unknown when
                  scheduled checks stop arriving.
                </p>
              </>
            ))}
          {view === "incidents" && (
            <>
              <div className="page-heading">
                <div>
                  <span className="eyebrow">Operational history</span>
                  <h1>Incident timeline</h1>
                  <p>
                    Detected automatically. Resolved when recovery conditions
                    are met.
                  </p>
                </div>
                <span className="label">
                  {active} open · {incidents.length} recent
                </span>
              </div>
              <section className="panel">
                <IncidentList incidents={incidents} onUpdate={refresh} />
              </section>
            </>
          )}
          {view === "channels" && <Channels />}
          {view === "settings" && (
            <SettingsPanel onLogout={() => setAuth(false)} />
          )}
          <footer className="content-footer">
            <span>
              uptivra<span className="brand-dot">.</span>
            </span>
            <span>Availability, measured.</span>
          </footer>
        </main>
      </div>
      {editor && (
        <MonitorEditor
          monitor={editor === "new" ? undefined : editor}
          onClose={() => setEditor(null)}
          onSave={() => {
            setEditor(null);
            setNotice("Monitor saved.");
            void refresh();
          }}
        />
      )}
    </div>
  );
}
