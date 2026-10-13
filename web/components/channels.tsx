"use client";
import { demoMode } from "@/lib/demo-mode";
import { useEffect, useState } from "react";
import { Trash2, Webhook } from "lucide-react";
import { api, type Channel, type Monitor } from "@/lib/api";
import { Empty, ErrorMessage } from "./shared";
export default function Channels() {
  const [channels, setChannels] = useState<Channel[]>([]);
  const [groups, setGroups] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [routingBusy, setRoutingBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [kind, setKind] = useState("webhook");
  const load = async () => {
    const [destinations, monitors] = await Promise.all([
      api<Channel[]>("/channels"),
      api<Monitor[]>("/monitors"),
    ]);
    setChannels(destinations);
    setGroups(
      [
        ...new Set([
          "",
          ...monitors.map((m) => m.group),
          ...destinations.flatMap((c) => (c.group == null ? [] : [c.group])),
        ]),
      ].sort(),
    );
  };
  const groupOptions = (
    <>
      <option value="null">All groups</option>
      {groups.map((group) => (
        <option key={group} value={JSON.stringify(group)}>
          {group || "Ungrouped monitors"}
        </option>
      ))}
    </>
  );
  useEffect(() => {
    if (!demoMode)
      void load()
        .catch((e) => setError(e.message))
        .finally(() => setLoading(false));
  }, []);
  if (demoMode)
    return (
      <section className="panel">
        <div className="section-heading">
          <h1>Notification channels</h1>
        </div>
        <p className="notice">
          Alert delivery requires the full backend. No webhooks or Discord
          messages are sent in demo mode. Do not enter real destinations or
          signing secrets.
        </p>
      </section>
    );
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
            Choose all groups or one monitor group for incident openings and
            recoveries. Failed deliveries make up to eight attempts with
            exponential backoff.
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
                group: JSON.parse(String(f.get("group"))),
              });
              form.reset();
              await load();
              setNotice(
                "Channel created. It will receive matching incident events.",
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
            Monitor group
            <select name="group" defaultValue="null" disabled={loading}>
              {groupOptions}
            </select>
            <small>
              Only alerts from the selected group are sent to this destination.
            </small>
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
          <button className="button primary" disabled={busy || loading}>
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
        {loading ? (
          <p role="status">Loading destinations…</p>
        ) : channels.length ? (
          <div className="simple-list channel-list">
            {channels.map((c) => (
              <div key={c.id}>
                <div>
                  <strong>{c.name}</strong>
                  <small>{c.kind}</small>
                </div>
                <label>
                  Group for {c.name}
                  <select
                    value={JSON.stringify(c.group ?? null)}
                    disabled={routingBusy}
                    onChange={async (e) => {
                      const group: string | null = JSON.parse(e.target.value);
                      setRoutingBusy(true);
                      setError("");
                      setNotice("");
                      try {
                        await api("/channels/" + c.id + "/group", "PUT", {
                          group,
                        });
                        await load();
                        setNotice(
                          "Group updated for future incident events. Already queued deliveries are retained.",
                        );
                      } catch (e) {
                        setError(
                          e instanceof Error
                            ? e.message
                            : "Unable to update group",
                        );
                      } finally {
                        setRoutingBusy(false);
                      }
                    }}
                  >
                    {groupOptions}
                  </select>
                </label>
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
