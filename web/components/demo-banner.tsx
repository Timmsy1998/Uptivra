"use client";
import { demoMode } from "@/lib/demo-mode";
import { resetDemo } from "@/lib/demo";
export default function DemoBanner() {
  if (!demoMode) return null;
  return (
    <aside className="notice" aria-label="Demo mode">
      <strong>Temporary browser demo</strong>
      <p>
        Your workspace stays in this tab and resets after two hours or when you
        close it. No live checks, alerts, accounts or API keys. Health stays
        unmeasured. Status is a local preview, visible only in this tab. Use
        example URLs; do not enter secrets.
      </p>
      <button
        className="button"
        onClick={() => {
          if (confirm("Clear all demo data in this tab?")) resetDemo();
        }}
      >
        Reset demo
      </button>
    </aside>
  );
}
