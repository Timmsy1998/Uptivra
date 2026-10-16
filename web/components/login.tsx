"use client";
import { useEffect, useState } from "react";
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
  const [setup, setSetup] = useState(false);
  useEffect(() => {
    api<{ required: boolean; enabled: boolean }>("/auth/setup")
      .then((v) => {
        setSetup(v.enabled);
        if (v.required && !v.enabled)
          setError(
            "Set SETUP_CODE on the server or use CLI bootstrap to create the first administrator.",
          );
      })
      .catch((e) => setError(e.message));
  }, []);
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
          <h2>{setup ? "Set up Uptivra" : "Sign in to Uptivra"}</h2>
          <p>
            {setup
              ? "Create the first administrator and workspace owner."
              : "Use your administrator account to continue."}
          </p>
          <ErrorMessage message={error} />
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              setBusy(true);
              setError("");
              try {
                if (setup) {
                  await api("/auth/setup", "POST", {
                    email: f.get("email"),
                    password: f.get("password"),
                    code: f.get("code"),
                  });
                  setSetup(false);
                  setError("Administrator created. Sign in to continue.");
                  return;
                }
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
            {setup && (
              <label>
                Verification code
                <input
                  name="code"
                  type="password"
                  inputMode="numeric"
                  pattern="[0-9]{4}"
                  minLength={4}
                  maxLength={4}
                  required
                  autoComplete="off"
                />
                <small>The four digits configured in SETUP_CODE.</small>
              </label>
            )}
            <label>
              Password
              <input
                name="password"
                type="password"
                autoComplete={setup ? "new-password" : "current-password"}
                minLength={setup ? 14 : undefined}
                maxLength={72}
                required
              />
            </label>
            <button className="button primary" disabled={busy}>
              {busy
                ? "Please wait…"
                : setup
                  ? "Create owner account"
                  : "Sign in"}
              <ChevronRight size={16} />
            </button>
          </form>
          <p className="footnote">
            Initial setup requires the verification code configured by your
            server operator.
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
