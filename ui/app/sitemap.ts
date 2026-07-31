import type { MetadataRoute } from "next"

import { siteUrl } from "@/lib/seo"

export const dynamic = "force-static"

type Route = {
  path: string
  priority: number
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"]
  /**
   * When the page's content last meaningfully changed, as YYYY-MM-DD. Bump it
   * when you edit the page. This used to be `new Date()` at build time, which
   * told crawlers every page changed on every deploy, and so told them nothing.
   */
  lastModified: string
}

// Public, indexable marketing/content pages only — app routes are excluded via robots.ts.
const routes: Route[] = [
  { path: "/", priority: 1.0, changeFrequency: "weekly", lastModified: "2026-07-31" },
  { path: "/pricing", priority: 0.8, changeFrequency: "monthly", lastModified: "2026-07-31" },
  { path: "/download", priority: 0.7, changeFrequency: "monthly", lastModified: "2026-07-31" },
  { path: "/docs", priority: 0.7, changeFrequency: "weekly", lastModified: "2026-07-31" },
  { path: "/login", priority: 0.5, changeFrequency: "yearly", lastModified: "2026-07-31" },
  { path: "/register", priority: 0.6, changeFrequency: "yearly", lastModified: "2026-07-31" },
  { path: "/terms", priority: 0.3, changeFrequency: "yearly", lastModified: "2026-07-31" },
  { path: "/privacy", priority: 0.3, changeFrequency: "yearly", lastModified: "2026-07-31" },
]

export default function sitemap(): MetadataRoute.Sitemap {
  return routes.map(({ path, priority, changeFrequency, lastModified }) => ({
    url: `${siteUrl}${path}`,
    lastModified: new Date(lastModified),
    changeFrequency,
    priority,
  }))
}
