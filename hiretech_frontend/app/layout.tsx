import type { Metadata } from "next";
import "./globals.css";
import "./theme.css";

const title = "HireTech";
const description = "Structured technical interviews with evidence-led evaluation and human review.";

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_APP_URL ?? "http://localhost:3000"),
  title: { default: title, template: "%s · HireTech" },
  description,
  openGraph: { title, description, images: [{ url: "/og-neutral.png", width: 1200, height: 630, alt: "HireTech technical interview workspace" }] },
  twitter: { card: "summary_large_image", title, description, images: ["/og-neutral.png"] },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
