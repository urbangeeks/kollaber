import Link from "next/link"
import type { Metadata } from "next"
import { notFound } from "next/navigation"

import { JsonLd } from "@/components/json-ld"
import { MarketingNav } from "@/components/marketing-nav"
import { INTEGRATIONS, getIntegration } from "@/lib/integrations"
import { breadcrumbSchema, faqPageSchema, howToSchema } from "@/lib/seo"

type Params = { slug: string }

export function generateStaticParams(): Params[] {
  return INTEGRATIONS.map(({ slug }) => ({ slug }))
}

// Any slug outside generateStaticParams is a 404, not an on-demand render —
// these pages are a fixed set and an unknown one should never be indexed.
export const dynamicParams = false

export async function generateMetadata({
  params,
}: {
  params: Promise<Params>
}): Promise<Metadata> {
  const { slug } = await params
  const integration = getIntegration(slug)
  if (!integration) return {}

  const path = `/integrations/${integration.slug}`
  const { title, description } = integration

  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: { title, description, url: path },
    twitter: { title, description },
  }
}

function Code({ children }: { children: string }) {
  return (
    <pre className="my-4 overflow-x-auto rounded-lg border border-white/10 bg-white/5 p-4 font-mono text-sm leading-relaxed text-white/80">
      {children}
    </pre>
  )
}

export default async function IntegrationPage({
  params,
}: {
  params: Promise<Params>
}) {
  const { slug } = await params
  const integration = getIntegration(slug)
  if (!integration) notFound()

  const path = `/integrations/${integration.slug}`

  return (
    <div className="min-h-screen bg-[#0a0a0a] font-sans text-white antialiased">
      <JsonLd
        schema={howToSchema({
          name: `Send ${integration.name} events to Kollaber`,
          description: integration.description,
          path,
          steps: integration.steps,
        })}
      />
      <JsonLd schema={faqPageSchema(integration.faqs)} />
      <JsonLd
        schema={breadcrumbSchema([
          { name: "Home", path: "/" },
          { name: "Integrations", path: "/integrations" },
          { name: integration.name, path },
        ])}
      />

      <MarketingNav label="Integrations" />

      <div className="container mx-auto max-w-3xl px-6 pb-24 pt-12">
        <nav aria-label="Breadcrumb" className="mb-8 text-sm text-white/40">
          <Link href="/integrations" className="transition-colors hover:text-white">
            Integrations
          </Link>
          <span className="mx-2 text-white/20">/</span>
          <span className="text-white/60">{integration.name}</span>
        </nav>

        <header className="mb-10">
          <h1 className="text-4xl font-bold tracking-tight">{integration.h1}</h1>
          <p className="mt-4 text-lg leading-relaxed text-white/50">{integration.intro}</p>
        </header>

        <section className="mb-12">
          <h2 className="mb-4 text-2xl font-bold">What lands on the timeline</h2>
          <dl className="space-y-3">
            {integration.captures.map((capture) => (
              <div
                key={capture.label}
                className="flex flex-col gap-1 rounded-lg border border-white/10 bg-white/5 p-4 sm:flex-row sm:gap-4"
              >
                <dt className="shrink-0 font-mono text-sm font-semibold text-[#a78bfa] sm:w-32">
                  {capture.label}
                </dt>
                <dd className="text-sm leading-relaxed text-white/60">{capture.detail}</dd>
              </div>
            ))}
          </dl>
        </section>

        <section className="mb-12">
          <h2 className="mb-6 text-2xl font-bold">Setup</h2>
          <ol className="space-y-8">
            {integration.steps.map((step, index) => (
              <li key={step.title} id={`step-${index + 1}`} className="scroll-mt-24">
                <h3 className="flex items-baseline gap-3 text-lg font-semibold text-white">
                  <span className="font-mono text-sm text-white/30">{index + 1}</span>
                  {step.title}
                </h3>
                <p className="mt-2 leading-relaxed text-white/60">{step.body}</p>
                {step.code && <Code>{step.code}</Code>}
              </li>
            ))}
          </ol>
        </section>

        {integration.notes.length > 0 && (
          <section className="mb-12">
            <h2 className="mb-4 text-2xl font-bold">Good to know</h2>
            <ul className="space-y-3">
              {integration.notes.map((note) => (
                <li
                  key={note}
                  className="border-l-2 border-white/10 pl-4 text-sm leading-relaxed text-white/60"
                >
                  {note}
                </li>
              ))}
            </ul>
          </section>
        )}

        <section className="mb-12">
          <h2 className="mb-4 text-2xl font-bold">Questions</h2>
          <div className="space-y-5">
            {integration.faqs.map((faq) => (
              <div key={faq.q}>
                <h3 className="font-semibold text-white">{faq.q}</h3>
                <p className="mt-1.5 text-sm leading-relaxed text-white/60">{faq.a}</p>
              </div>
            ))}
          </div>
        </section>

        <div className="rounded-lg border border-white/10 bg-white/5 p-6">
          <h2 className="text-lg font-semibold text-white">
            Start recording {integration.name} changes
          </h2>
          <p className="mt-2 text-sm leading-relaxed text-white/50">
            The free plan covers two environments and five members. The full reference for this
            integration lives in the{" "}
            <Link
              href={`/docs#${integration.docsAnchor}`}
              className="text-[#a78bfa] hover:underline"
            >
              docs
            </Link>
            .
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <Link
              href="/register"
              className="rounded-full bg-white px-5 py-2 text-sm font-medium text-black transition-colors hover:bg-neutral-200"
            >
              Get started free
            </Link>
            <Link
              href="/integrations"
              className="rounded-full border border-white/20 px-5 py-2 text-sm font-medium text-white/70 transition-colors hover:border-white/40 hover:text-white"
            >
              All integrations
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}
