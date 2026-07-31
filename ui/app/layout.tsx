import { Geist_Mono, Inter } from "next/font/google"
import type { Metadata } from "next"

import "./globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { Toaster } from "@/components/ui/sonner"
import { siteDescription, siteUrl } from "@/lib/seo"
import { cn } from "@/lib/utils";

const title = "Kollaber — The shared timeline for infrastructure teams"
const description = siteDescription

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: title,
    template: "%s — Kollaber",
  },
  description,
  applicationName: "Kollaber",
  keywords: [
    "infrastructure timeline",
    "deploy tracking",
    "incident timeline",
    "alert correlation",
    "DevOps collaboration",
    "SRE tools",
    "deployment events",
    "Kubernetes",
  ],
  authors: [{ name: "Kollaber" }],
  // Canonical for "/" itself — the landing page is a client component and so
  // cannot export metadata. Every other public route overrides this in its own
  // metadata; app routes inherit it but are disallowed in robots.ts.
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    url: siteUrl,
    siteName: "Kollaber",
    title,
    description,
    images: [
      {
        url: "/og.png",
        width: 1200,
        height: 630,
        alt: "Kollaber — The shared timeline for infrastructure teams",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
    images: ["/og.png"],
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-image-preview": "large",
      "max-snippet": -1,
      "max-video-preview": -1,
    },
  },
}

const inter = Inter({subsets:['latin'],variable:'--font-sans'})

const fontMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
})

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      data-scroll-behavior="smooth"
      className={cn("antialiased", fontMono.variable, "font-sans", inter.variable)}
    >
      <body>
        <ThemeProvider>
          {children}
          <Toaster richColors position="bottom-right" />
        </ThemeProvider>
      </body>
    </html>
  )
}
