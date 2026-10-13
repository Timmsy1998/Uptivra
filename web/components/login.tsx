"use client";
import { useState } from "react";
import { Activity, ArrowUpRight, ChevronRight } from "lucide-react";
import { api } from "@/lib/api";
import { Brand, Theme, ErrorMessage } from "./shared";
export default function Login({
  error: initialError,
  onLogin,
}: {
  error: string;
  onLogin: () => void;
}) {
  const [error, setError] = useState(initialError);
  const [busy, setBusy] = useState(false);
  return (
    <div className="login-page">
      <header>
        <Brand />
        <Theme />
      </header>
      <main>
        <div className="login-intro">
          <span className="eyebrow">Your services. In view.</span>
          <h1>
            Stay close to
            <br />
            what keeps you running.
          </h1>
          <p>
            A quiet place for service health, response times and incidents that
            matter.
          </p>
          <div className="login-rule">
            <Activity size={20} />
            <span>Availability, measured.</span>
          </div>
        </div>
        <section className="login-form">
          <span className="eyebrow">Workspace access</span>
          <h2>Sign in to Uptivra</h2>
          <p>Use your administrator account to continue.</p>
          <ErrorMessage message={error} />
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              setBusy(true);
              setError("");
              try {
                await api("/auth/login", "POST", {
                  email: f.get("email"),
                  password: f.get("password"),
                });
                onLogin();
              } catch (e) {
                setError(e instanceof Error ? e.message : "Sign-in failed");
              } finally {
                setBusy(false);
              }
            }}
          >
            <label>
              Email address
              <input
                name="email"
                type="email"
                autoComplete="username"
                required
                autoFocus
                placeholder="you@example.com"
              />
            </label>
            <label>
              Password
              <input
                name="password"
                type="password"
                autoComplete="current-password"
                required
              />
            </label>
            <button className="button primary" disabled={busy}>
              {busy ? "Signing in…" : "Sign in"}
              <ChevronRight size={16} />
            </button>
          </form>
          <p className="footnote">
            First time here? Create your administrator with the{" "}
            <code>bootstrap</code> command.
          </p>
          <a className="text-button" href="/status">
            View public status
            <ArrowUpRight size={14} />
          </a>
        </section>
      </main>
      <footer>Uptivra · Independent uptime monitoring</footer>
    </div>
  );
}
