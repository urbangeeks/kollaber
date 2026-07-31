import type { Metadata } from "next"

const title = "Sign in"
const description =
  "Sign in to Kollaber to see your team's infrastructure timeline — deploys, alerts, incidents, and the decisions attached to them."

// The page itself is a client component and cannot export metadata.
export const metadata: Metadata = {
  title,
  description,
  alternates: { canonical: "/login" },
  openGraph: { title, description, url: "/login" },
  twitter: { title, description },
}

export default function LoginLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}
