"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { ErrorMessage } from "./shared";
type Admin = { id: number; email: string; owner: boolean };
export default function Admins() {
  const [admins, setAdmins] = useState<Admin[]>([]);
  const [me, setMe] = useState<Admin | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const load = async () => {
    const [a, m] = await Promise.all([
      api<Admin[]>("/admins"),
      api<Admin>("/auth/session"),
    ]);
    setAdmins(a);
    setMe(m);
  };
  useEffect(() => {
    void load().catch((e) => setError(e.message));
  }, []);
  async function action(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="panel settings-panel">
      <div>
        <h2>Administrators & ownership</h2>
        <p>
          Administrators manage monitoring, alerts and settings. The owner
          creates and removes administrators and can transfer ownership. The
          current owner is protected from deletion.
        </p>
      </div>
      <div>
        <ErrorMessage message={error} />
        {notice && <p role="status">{notice}</p>}
        {!me && <p role="status">Loading administrators…</p>}
        {me?.owner && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const form = e.currentTarget;
              const f = new FormData(form);
              void action(async () => {
                await api("/admins", "POST", {
                  email: f.get("email"),
                  password: f.get("password"),
                });
                form.reset();
                setNotice("Administrator created.");
              });
            }}
          >
            <label>
              New administrator email
              <input name="email" type="email" required maxLength={254} />
            </label>
            <label>
              Temporary password
              <input
                name="password"
                type="password"
                required
                minLength={14}
                maxLength={72}
                autoComplete="new-password"
              />
            </label>
            <button className="button" disabled={busy}>
              Create administrator
            </button>
          </form>
        )}
        <div className="simple-list">
          {admins.map((a) => (
            <div key={a.id}>
              <div>
                <strong>{a.email}</strong>
                <small>
                  {a.owner ? "Owner" : "Administrator"}
                  {a.id === me?.id ? " · You" : ""}
                </small>
              </div>
              {me?.owner && !a.owner && (
                <div className="row-actions">
                  <button
                    className="button"
                    disabled={busy}
                    onClick={() => {
                      if (
                        confirm(
                          `Transfer ownership to ${a.email}? You will become an administrator and lose account management access.`,
                        )
                      )
                        void action(async () => {
                          await api(`/admins/${a.id}/ownership`, "POST");
                          setNotice("Ownership transferred.");
                        });
                    }}
                  >
                    Transfer ownership to {a.email}
                  </button>
                  <button
                    className="button danger"
                    disabled={busy}
                    onClick={() => {
                      if (
                        confirm(
                          `Delete administrator ${a.email} and revoke their sessions?`,
                        )
                      )
                        void action(async () => {
                          await api(`/admins/${a.id}`, "DELETE");
                          setNotice("Administrator deleted.");
                        });
                    }}
                  >
                    Delete administrator {a.email}
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
