"use client";
import { demoMode } from "@/lib/demo-mode";
import { useEffect, useState } from "react";
import { KeyRound, Trash2 } from "lucide-react";
import { api, type Settings, type APIKey, date } from "@/lib/api";
import { ErrorMessage, Empty } from "./shared";
export default function SettingsPanel({ onLogout }: { onLogout: () => void }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const load = () =>
    Promise.all([
      api<Settings>("/settings").then(setSettings),
      demoMode ? Promise.resolve() : api<APIKey[]>("/keys").then(setKeys),
    ]).catch((e) => setError(e.message));
  useEffect(() => {
    void load();
  }, []);
  async function action(f: () => Promise<void>) {
    setError("");
    setNotice("");
    setBusy(true);
    try {
      await f();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">Administration</span>
          <h1>Settings & access</h1>
          <p>Public presence, data retention and integration credentials.</p>
        </div>
      </div>
      <ErrorMessage message={error} />
      {notice && (
        <div role="status" className="notice">
          {notice}
        </div>
      )}
      <section className="panel settings-panel">
        <div>
          <h2>Public status page</h2>
          <p>
            Only monitors explicitly marked public appear on{" "}
            <a
              href="/status"
              target={demoMode ? "_self" : "_blank"}
              rel="noreferrer"
            >
              your status page
            </a>
            .
          </p>
        </div>
        {settings && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              void action(async () => {
                await api("/settings", "PUT", {
                  title: f.get("title"),
                  description: f.get("description"),
                  retention_days: Number(f.get("retention")),
                });
                setNotice("Settings saved.");
              });
            }}
          >
            <label>
              Page title
              <input
                name="title"
                required
                maxLength={100}
                defaultValue={settings.title}
              />
            </label>
            <label>
              Description
              <textarea
                name="description"
                maxLength={500}
                rows={3}
                defaultValue={settings.description}
              />
            </label>
            <label>
              History retention (days)
              <input
                name="retention"
                type="number"
                min={7}
                max={365}
                required
                defaultValue={settings.retention_days}
              />
              <small>
                {demoMode
                  ? "Configuration preview only. The entire demo workspace expires after two hours regardless of this value."
                  : "Older checks and resolved incidents are removed by scheduled cleanup."}
              </small>
            </label>
            <button className="button primary" disabled={busy}>
              Save settings
            </button>
          </form>
        )}
      </section>
      {demoMode ? (
        <p className="notice">
          API keys and password changes require the full backend. No credentials
          are created in this demo.
        </p>
      ) : (
        <>
          <section className="panel settings-panel">
            <div>
              <h2>API keys</h2>
              <p>
                Read keys inspect monitors and history. Write keys also manage
                monitors and status settings. Account and channel access require
                a session.
              </p>
              <a
                href="https://github.com/timmsy1998/uptivra/blob/master/docs/integrations.md"
                target="_blank"
                rel="noreferrer"
              >
                Integration documentation ↗
              </a>
            </div>
            <div>
              {token && (
                <div className="token" role="status">
                  <strong>Copy this key now. It is shown once.</strong>
                  <code>{token}</code>
                  <button className="text-button" onClick={() => setToken("")}>
                    Dismiss
                  </button>
                </div>
              )}
              <form
                className="inline-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  const form = e.currentTarget;
                  const f = new FormData(form);
                  void action(async () => {
                    const v = await api<{ token: string }>("/keys", "POST", {
                      name: f.get("name"),
                      scope: f.get("scope"),
                    });
                    setToken(v.token);
                    form.reset();
                    await load();
                  });
                }}
              >
                <label>
                  Key name
                  <input
                    name="name"
                    required
                    maxLength={80}
                    placeholder="Deployment pipeline"
                  />
                </label>
                <label>
                  Scope
                  <select name="scope">
                    <option value="read">Read</option>
                    <option value="write">Write</option>
                  </select>
                </label>
                <button className="button" disabled={busy}>
                  <KeyRound size={15} />
                  Create key
                </button>
              </form>
              {keys.length ? (
                <div className="simple-list">
                  {keys.map((k) => (
                    <div key={k.id}>
                      <div>
                        <strong>{k.name}</strong>
                        <small>
                          {k.scope} · {date(k.created_at)}
                        </small>
                      </div>
                      <button
                        className="icon-button danger"
                        aria-label={"Revoke " + k.name}
                        onClick={() => {
                          if (
                            confirm(
                              "Revoke " +
                                k.name +
                                "? Integrations using it will lose access.",
                            )
                          )
                            void action(async () => {
                              await api("/keys/" + k.id, "DELETE");
                              await load();
                              setNotice("API key revoked.");
                            });
                        }}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  ))}
                </div>
              ) : (
                <Empty title="No integration keys">
                  Create a key when an application needs private API access.
                </Empty>
              )}
            </div>
          </section>
          <section className="panel settings-panel">
            <div>
              <h2>Administrator password</h2>
              <p>
                Changing your password ends every active session. Use 14–72
                bytes.
              </p>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void action(async () => {
                  await api("/account/password", "PUT", {
                    current: f.get("current"),
                    next: f.get("next"),
                  });
                  onLogout();
                });
              }}
            >
              <label>
                Current password
                <input
                  name="current"
                  type="password"
                  autoComplete="current-password"
                  required
                />
              </label>
              <label>
                New password
                <input
                  name="next"
                  type="password"
                  autoComplete="new-password"
                  minLength={14}
                  maxLength={72}
                  required
                />
              </label>
              <button className="button" disabled={busy}>
                Change password
              </button>
            </form>
          </section>
        </>
      )}
    </>
  );
}
