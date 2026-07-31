import type { Metadata } from "next"

import { JsonLd } from "@/components/json-ld"
import { pricingSchema } from "@/lib/seo"
import { PLANS } from "./plans"

const title = "Pricing"
const description =
  "Kollaber pricing: a free plan for small teams, $12/seat for Team, $24/seat for Pro with Kubernetes ingestion, SSO and audit logs, plus custom Enterprise contracts."

// The page itself is a client component and cannot export metadata, so the
// route's metadata and structured data live in this layout.
export const metadata: Metadata = {
  title,
  description,
  alternates: { canonical: "/pricing" },
  openGraph: { title, description, url: "/pricing" },
  twitter: { title, description },
}

export default function PricingLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <>
      <JsonLd schema={pricingSchema(PLANS)} />
      {children}
    </>
  )
}
