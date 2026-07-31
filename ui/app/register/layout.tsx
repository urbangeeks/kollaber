import type { Metadata } from "next"

const title = "Create an account"
const description =
  "Start a Kollaber organization free. Capture deploys, alerts, and notes on one shared timeline in minutes — no credit card required."

// The page itself is a client component and cannot export metadata.
export const metadata: Metadata = {
  title,
  description,
  alternates: { canonical: "/register" },
  openGraph: { title, description, url: "/register" },
  twitter: { title, description },
}

export default function RegisterLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}
