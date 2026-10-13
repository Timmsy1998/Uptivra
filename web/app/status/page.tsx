import type { Metadata } from "next";
import StatusPage from "./status-page";
export const metadata: Metadata = {
  title: "Service status",
  description: "Current service availability and recent incident history.",
  robots: { index: true, follow: true },
  openGraph: {
    title: "Service status · Uptivra",
    description: "Current service availability and recent incident history.",
    type: "website",
  },
};
export default function Page() {
  return <StatusPage />;
}
