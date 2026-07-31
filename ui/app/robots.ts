import type { MetadataRoute } from "next"

import { siteUrl } from "@/lib/seo"

export const dynamic = "force-static"

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      // Authenticated app routes. Anything not listed in sitemap.ts belongs
      // here — an unlisted, uncrawled route otherwise inherits the root
      // canonical and competes with "/" in the index.
      disallow: [
        "/dashboard",
        "/admin",
        "/settings",
        "/env",
        "/onboarding",
        "/auth",
        "/invite",
        "/incidents",
        "/decisions",
        "/inventory",
        "/metrics",
        "/search",
      ],
    },
    sitemap: `${siteUrl}/sitemap.xml`,
    host: siteUrl,
  }
}
