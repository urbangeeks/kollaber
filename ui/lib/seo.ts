/**
 * Shared SEO constants and structured-data builders.
 *
 * `siteUrl` was previously duplicated in layout.tsx, sitemap.ts and robots.ts;
 * canonical URLs need the same value everywhere, so it lives here now.
 */

export const siteUrl =
  process.env.NEXT_PUBLIC_SITE_URL?.replace(/\/$/, "") || "https://kollaber.io"

export const siteName = "Kollaber"

export const siteDescription =
  "Stop correlating incidents in Slack. Capture deploys, alerts, and manual notes in one collaborative view designed for modern DevOps and SRE teams."

/** Absolute URL for a site-relative path, e.g. absoluteUrl("/pricing"). */
export function absoluteUrl(path: string): string {
  return `${siteUrl}${path}`
}

type JsonLdObject = Record<string, unknown>

/**
 * The product itself. Rendered on the landing page so search engines can
 * associate the name, category and pricing entry point with the site.
 */
export function softwareApplicationSchema(): JsonLdObject {
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: siteName,
    applicationCategory: "DeveloperApplication",
    applicationSubCategory: "Change intelligence and operational memory",
    operatingSystem: "Web, macOS, Linux, Windows",
    url: siteUrl,
    description: siteDescription,
    image: absoluteUrl("/og.png"),
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "USD",
      description: "Free plan for up to 5 members and 2 environments.",
      url: absoluteUrl("/pricing"),
    },
    publisher: {
      "@type": "Organization",
      name: siteName,
      url: siteUrl,
      logo: absoluteUrl("/logo.png"),
    },
  }
}

export type PricingPlan = {
  name: string
  price: string
  per: string
  description: string
}

/**
 * Pricing table as a Product with one Offer per plan. "Custom" pricing has no
 * numeric price, so those plans are emitted without a `price` field rather than
 * with a misleading zero.
 */
export function pricingSchema(plans: readonly PricingPlan[]): JsonLdObject {
  return {
    "@context": "https://schema.org",
    "@type": "Product",
    name: `${siteName} — plans and pricing`,
    description:
      "Plans for infrastructure teams capturing deploys, alerts and decisions on a shared timeline.",
    image: absoluteUrl("/og.png"),
    brand: { "@type": "Brand", name: siteName },
    offers: plans.map((plan) => {
      const numeric = plan.price.startsWith("$")
        ? plan.price.slice(1)
        : null

      const offer: JsonLdObject = {
        "@type": "Offer",
        name: plan.name,
        description: plan.description,
        priceCurrency: "USD",
        url: absoluteUrl("/pricing"),
        availability: "https://schema.org/InStock",
      }

      if (numeric !== null) {
        offer.price = numeric
        offer.priceSpecification = {
          "@type": "UnitPriceSpecification",
          price: numeric,
          priceCurrency: "USD",
          unitText: plan.per,
        }
      }

      return offer
    }),
  }
}

/**
 * Setup instructions for an integration page. Only steps are modelled — the
 * surrounding prose is not part of the HowTo, so the structured data stays a
 * faithful summary of what the page actually walks through.
 */
export function howToSchema(args: {
  name: string
  description: string
  path: string
  steps: { title: string; body: string }[]
}): JsonLdObject {
  return {
    "@context": "https://schema.org",
    "@type": "HowTo",
    name: args.name,
    description: args.description,
    url: absoluteUrl(args.path),
    step: args.steps.map((step, index) => ({
      "@type": "HowToStep",
      position: index + 1,
      name: step.title,
      text: step.body,
      url: `${absoluteUrl(args.path)}#step-${index + 1}`,
    })),
  }
}

export function faqPageSchema(faqs: { q: string; a: string }[]): JsonLdObject {
  return {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: faqs.map((faq) => ({
      "@type": "Question",
      name: faq.q,
      acceptedAnswer: { "@type": "Answer", text: faq.a },
    })),
  }
}

export function breadcrumbSchema(
  crumbs: { name: string; path: string }[],
): JsonLdObject {
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: crumbs.map((crumb, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: crumb.name,
      item: absoluteUrl(crumb.path),
    })),
  }
}
