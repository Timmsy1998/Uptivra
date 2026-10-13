"use client";
import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { api, type Monitor } from "@/lib/api";
import { ErrorMessage } from "./shared";
const defaults = {
  name: "",
  description: "",
  group: "",
  url: "",
  interval_seconds: 60,
  timeout_seconds: 5,
  expected_status: 200,
  contains: "",
  failure_threshold: 3,
  recovery_threshold: 2,
  enabled: true,
  public: false,
  authorized: false,
  maintenance_until: null,
};
export default function MonitorEditor({
  monitor,
  onClose,
  onSave,
}: {
  monitor?: Monitor;
  onClose: () => void;
  onSave: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  const v = monitor || defaults;
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const f = new FormData(e.currentTarget);
    try {
      const headers = JSON.parse(String(f.get("headers") || "{}"));
      if (
        typeof headers !== "object" ||
        headers === null ||
        Array.isArray(headers) ||
        Object.values(headers).some((v) => typeof v !== "string")
      )
        throw new Error("Headers must be a JSON object with string values.");
      const data = {
        name: f.get("name"),
        description: f.get("description"),
        group: f.get("group"),
        url: f.get("url"),
        interval_seconds: Number(f.get("interval")),
        timeout_seconds: Number(f.get("timeout")),
        expected_status: Number(f.get("status")),
        contains: f.get("contains"),
        headers,
        failure_threshold: Number(f.get("failures")),
        recovery_threshold: Number(f.get("recoveries")),
        enabled: f.has("enabled"),
        public: f.has("public"),
        authorized: f.has("authorized"),
        maintenance_until: f.get("maintenance")
          ? new Date(String(f.get("maintenance"))).toISOString()
          : null,
      };
      await api(
        monitor ? "/monitors/" + monitor.id : "/monitors",
        monitor ? "PUT" : "POST",
        data,
      );
      onSave();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to save monitor");
    } finally {
      setBusy(false);
    }
  }
  const localTime = v.maintenance_until
    ? new Date(
        new Date(v.maintenance_until).getTime() -
          new Date().getTimezoneOffset() * 60000,
      )
        .toISOString()
        .slice(0, 16)
    : "";
  return (
    <dialog ref={ref} onCancel={onClose} className="editor">
      <div className="dialog-heading">
        <div>
          <span className="eyebrow">Configuration</span>
          <h2>{monitor ? "Edit monitor" : "New monitor"}</h2>
        </div>
        <button
          className="icon-button"
          aria-label="Close editor"
          onClick={onClose}
        >
          <X size={19} />
        </button>
      </div>
      <form onSubmit={submit}>
        <ErrorMessage message={error} />
        <div className="form-grid">
          <label>
            Display name
            <input
              name="name"
              autoFocus
              required
              maxLength={100}
              defaultValue={v.name}
              placeholder="Production API"
            />
          </label>
          <label>
            Group
            <input
              name="group"
              maxLength={80}
              defaultValue={v.group}
              placeholder="Core services"
            />
          </label>
          <label className="wide">
            Endpoint
            <input
              name="url"
              type="url"
              required
              defaultValue={v.url}
              placeholder="https://api.example.com/health"
            />
            <small>Public HTTP or HTTPS endpoints, on ports 80 and 443.</small>
          </label>
          <label className="wide">
            Description
            <textarea
              name="description"
              maxLength={500}
              defaultValue={v.description}
              rows={2}
            />
          </label>
          <label>
            Interval (seconds)
            <input
              name="interval"
              type="number"
              min={60}
              max={86400}
              required
              defaultValue={v.interval_seconds}
            />
          </label>
          <label>
            Timeout (seconds)
            <input
              name="timeout"
              type="number"
              min={1}
              max={10}
              required
              defaultValue={v.timeout_seconds}
            />
          </label>
          <label>
            Expected HTTP status
            <input
              name="status"
              type="number"
              min={100}
              max={599}
              required
              defaultValue={v.expected_status}
            />
          </label>
          <label>
            Response must contain
            <input
              name="contains"
              maxLength={1024}
              defaultValue={v.contains}
              placeholder="Optional text"
            />
          </label>
          <label>
            Failures to open incident
            <input
              name="failures"
              type="number"
              min={1}
              max={10}
              required
              defaultValue={v.failure_threshold}
            />
          </label>
          <label>
            Successes to recover
            <input
              name="recoveries"
              type="number"
              min={1}
              max={10}
              required
              defaultValue={v.recovery_threshold}
            />
          </label>
          <label className="wide">
            Request headers (JSON)
            <textarea
              className="mono"
              name="headers"
              rows={3}
              defaultValue={JSON.stringify(monitor?.headers || {}, null, 2)}
              spellCheck={false}
            />
            <small>
              Encrypted at rest. Never included on the public status page.
            </small>
          </label>
          <label className="wide">
            Maintenance until (local time)
            <input
              name="maintenance"
              type="datetime-local"
              defaultValue={localTime}
            />
            <small>
              Checks are skipped during maintenance. Clear to resume
              immediately.
            </small>
          </label>
          <label className="check">
            <input type="checkbox" name="enabled" defaultChecked={v.enabled} />
            Monitoring enabled
          </label>
          <label className="check">
            <input type="checkbox" name="public" defaultChecked={v.public} />
            Show on public status page
          </label>
          <label className="check wide">
            <input
              type="checkbox"
              name="authorized"
              required
              defaultChecked={v.authorized}
            />
            I own this endpoint or have permission to monitor it.
          </label>
        </div>
        <div className="dialog-footer">
          <button type="button" className="button" onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? "Saving…" : "Save monitor"}
          </button>
        </div>
      </form>
    </dialog>
  );
}
