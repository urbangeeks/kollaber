import Link from "next/link"
import type { Metadata } from "next"

import { JsonLd } from "@/components/json-ld"
import { MarketingNav } from "@/components/marketing-nav"
import { INTEGRATIONS, type Integration } from "@/lib/integrations"
import { absoluteUrl, breadcrumbSchema, siteName } from "@/lib/seo"

const title = "Integrations"
const description =
  "Send deploys, applies, and alerts to Kollaber from GitHub Actions, Argo CD, HCP Terraform, Atlantis, Prometheus Alertmanager, and Kubernetes."

export const metadata: Metadata = {
  title,
  description,
  alternates: { canonical: "/integrations" },
  openGraph: { title, description, url: "/integrations" },
  twitter: { title, description },
}

const CATEGORY_ORDER: Integration["category"][] = [
  "Deployments",
  "Infrastructure as code",
  "Alerting",
]

function itemListSchema() {
  return {
    "@context": "https://schema.org",
    "@type": "ItemList",
    name: `${siteName} integrations`,
    description,
    itemListElement: INTEGRATIONS.map((integration, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: integration.name,
      url: absoluteUrl(`/integrations/${integration.slug}`),
    })),
  }
}

export default function IntegrationsIndexPage() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] font-sans text-white antialiased">
      <JsonLd schema={itemListSchema()} />
      <JsonLd
        schema={breadcrumbSchema([
          { name: "Home", path: "/" },
          { name: "Integrations", path: "/integrations" },
        ])}
      />

      <MarketingNav label="Integrations" />

      <div className="container mx-auto max-w-4xl px-6 pb-24 pt-12">
        <div className="mb-12">
          <h1 className="text-4xl font-bold tracking-tight">Integrations</h1>
          <p className="mt-3 max-w-2xl text-lg text-white/50">
            Kollaber sits downstream of the tools that already change your infrastructure. Point them
            at it and every deploy, apply, and alert lands on one shared timeline.
          </p>
        </div>

        {CATEGORY_ORDER.map((category) => {
          const items = INTEGRATIONS.filter((i) => i.category === category)
          if (items.length === 0) return null

          return (
            <section key={category} className="mb-12">
              <h2 className="mb-4 text-xs font-semibold uppercase tracking-widest text-white/30">
                {category}
              </h2>
              <div className="grid gap-4 sm:grid-cols-2">
                {items.map((integration) => (
                  <Link
                    key={integration.slug}
                    href={`/integrations/${integration.slug}`}
                    className="group rounded-lg border border-white/10 bg-white/5 p-5 transition-colors hover:border-white/25 hover:bg-white/10"
                  >
                    <h3 className="font-semibold text-white group-hover:text-white">
                      {integration.name}
                    </h3>
                    <p className="mt-1.5 text-sm leading-relaxed text-white/50">
                      {integration.tagline}
                    </p>
                  </Link>
                ))}
              </div>
            </section>
          )
        })}

        <div className="rounded-lg border border-white/10 bg-white/5 p-6">
          <h2 className="font-semibold text-white">Something else?</h2>
          <p className="mt-2 text-sm leading-relaxed text-white/50">
            Anything that can send an HTTP request can post to the generic webhook endpoint, and the{" "}
            <Link href="/download" className="text-[#a78bfa] hover:underline">
              CLI
            </Link>{" "}
            records a deploy in one command. See the{" "}
            <Link href="/docs#webhooks" className="text-[#a78bfa] hover:underline">
              webhook reference
            </Link>{" "}
            for the payload shape.
          </p>
        </div>
      </div>
    </div>
  )
}
