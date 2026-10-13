"use client";
import { useEffect, useState } from "react";
import { Trash2, Webhook } from "lucide-react";
import { api, type Channel } from "@/lib/api";
import { Empty, ErrorMessage } from "./shared";
export default function Channels() {
  const [channels, setChannels] = useState<Channel[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [kind, setKind] = useState("webhook");
  const load = () =>
    api<Channel[]>("/channels")
      .then(setChannels)
      .catch((e) => setError(e.message));
  useEffect(() => {
    void load();
  }, []);
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">Incident delivery</span>
          <h1>Notification channels</h1>
          <p>Know when a service fails, and when it recovers.</p>
        </div>
      </div>
      <ErrorMessage message={error} />
      {notice && (
        <div className="notice" role="status">
          {notice}
        </div>
      )}
      <section className="panel settings-panel">
        <div>
          <h2>Add a destination</h2>
          <p>
            Every channel receives incident openings and recoveries. Failed
            deliveries retry up to eight times with exponential backoff.
          </p>
          <p>
            Webhook payloads are signed with HMAC-SHA256. Discord messages
            suppress mentions.
          </p>
        </div>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            const form = e.currentTarget;
            const f = new FormData(form);
            setBusy(true);
            setError("");
            try {
              await api("/channels", "POST", {
                name: f.get("name"),
                kind,
                url: f.get("url"),
                signing_secret: f.get("secret") || "",
              });
              form.reset();
              await load();
              setNotice(
                "Channel created. It will receive the next incident event.",
              );
            } catch (e) {
              setError(
                e instanceof Error ? e.message : "Unable to create channel",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            Name
            <input
              name="name"
              required
              maxLength={80}
              placeholder="Engineering on-call"
            />
          </label>
          <label>
            Provider
            <select value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="webhook">Signed webhook</option>
              <option value="discord">Discord webhook</option>
            </select>
          </label>
          <label>
            Destination URL
            <input
              name="url"
              type="url"
              required
              placeholder={
                kind === "discord"
                  ? "https://discord.com/api/webhooks/…"
                  : "https://example.com/hooks/uptivra"
              }
            />
          </label>
          {kind === "webhook" && (
            <label>
              Signing secret
              <input
                name="secret"
                type="password"
                required
                minLength={32}
                maxLength={256}
                autoComplete="off"
              />
              <small>
                At least 32 bytes. Store the same value in your webhook
                receiver.
              </small>
            </label>
          )}
          <button className="button primary" disabled={busy}>
            <Webhook size={15} />
            {busy ? "Creating…" : "Create channel"}
          </button>
        </form>
      </section>
      <section className="panel">
        <div className="section-heading">
          <h2>Configured destinations</h2>
          <span className="label">{channels.length} channels</span>
        </div>
        {channels.length ? (
          <div className="simple-list">
            {channels.map((c) => (
              <div key={c.id}>
                <div>
                  <strong>{c.name}</strong>
                  <small>{c.kind}</small>
                </div>
                <button
                  className="icon-button danger"
                  aria-label={"Delete " + c.name}
                  onClick={async () => {
                    if (
                      !confirm(
                        "Delete " + c.name + " and its queued deliveries?",
                      )
                    )
                      return;
                    try {
                      await api("/channels/" + c.id, "DELETE");
                      await load();
                      setNotice("Channel deleted.");
                    } catch (e) {
                      setError(
                        e instanceof Error ? e.message : "Delete failed",
                      );
                    }
                  }}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            ))}
          </div>
        ) : (
          <Empty title="No notification channels">
            Add a webhook or Discord destination to receive incident alerts.
          </Empty>
        )}
      </section>
    </>
  );
}
