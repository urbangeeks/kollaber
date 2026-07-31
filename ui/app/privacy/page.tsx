import Link from "next/link"
import Image from "next/image"
import type { Metadata } from "next"

const description =
  "How Kollaber collects, uses, shares, and protects information about you and your organization's infrastructure events."

export const metadata: Metadata = {
  title: "Privacy Policy",
  description,
  alternates: { canonical: "/privacy" },
  openGraph: { title: "Privacy Policy", description, url: "/privacy" },
  twitter: { title: "Privacy Policy", description },
}

const SECTIONS = [
  {
    title: "1. Introduction",
    body: `Kollaber, Inc. ("Kollaber", "we", "our", or "us") operates the Kollaber platform, a shared timeline and collaboration tool for infrastructure teams. This Privacy Policy explains how we collect, use, share, and protect information about you when you use our website and services (collectively, the "Service"). By using the Service you agree to the practices described here.`,
  },
  {
    title: "2. Information We Collect",
    subsections: [
      {
        label: "Account information",
        text: "When you register, we collect your email address, name, and password (stored as a salted hash). Organization and team details you provide during onboarding are also stored.",
      },
      {
        label: "Usage and event data",
        text: "We collect infrastructure events you submit — deploys, alerts, and notes — along with associated metadata (service name, environment, version, timestamps). This is the core data the Service is built to store.",
      },
      {
        label: "Log and technical data",
        text: "We automatically collect IP addresses, browser type, device identifiers, and pages visited for security, debugging, and analytics purposes.",
      },
      {
        label: "Communications",
        text: "If you contact us by email or in-app, we retain those communications to resolve your request.",
      },
      {
        label: "Payment information",
        text: "Billing details are handled by Stripe. We do not store your full card number or CVV — only Stripe's tokenized reference.",
      },
    ],
  },
  {
    title: "3. How We Use Your Information",
    body: `We use the information we collect to: (a) provide, maintain, and improve the Service; (b) authenticate you and protect account security; (c) process payments and send billing receipts; (d) send transactional and product update emails; (e) respond to your support requests; (f) detect abuse, fraud, and security incidents; (g) comply with legal obligations. We do not sell your personal information or use it for advertising.`,
  },
  {
    title: "4. Data Sharing",
    body: `We share your information only with: (a) service providers who help us operate the Service (e.g., cloud infrastructure, email delivery, payment processing) under strict data processing agreements; (b) other members of your Kollaber organization, to the extent you have shared data with them within the platform; (c) law enforcement or government authorities when required by applicable law or valid legal process; (d) a successor entity in connection with a merger, acquisition, or sale of assets, with advance notice to you. We do not share your data with third parties for their independent marketing purposes.`,
  },
  {
    title: "5. Data Retention",
    body: `We retain your account data for as long as your account is active. Event data is retained according to your plan's history limits (30 days on Free, unlimited on Team/Pro/Enterprise). When you delete your account, we delete your personal data within 30 days, except where we are legally required to retain it. Anonymized, aggregated data may be retained indefinitely for analytics.`,
  },
  {
    title: "6. Data Security",
    body: `We implement industry-standard security measures including TLS encryption in transit, AES-256 encryption at rest, access controls, and regular security reviews. However, no method of transmission over the Internet is 100% secure. We will notify you and relevant authorities of a data breach as required by applicable law.`,
  },
  {
    title: "7. Cookies and Tracking",
    body: `We use strictly necessary cookies to maintain your authenticated session. We do not use third-party advertising or behavioral tracking cookies. We may use a minimal analytics tool to understand aggregate usage patterns; such data is not linked to identifiable individuals. You can disable cookies in your browser settings, though this may break authentication.`,
  },
  {
    title: "8. Your Rights",
    body: `Depending on your location, you may have the right to: access a copy of the personal data we hold about you; correct inaccurate data; request deletion of your data; restrict or object to certain processing; and receive your data in a portable format. To exercise these rights, email privacy@kollaber.io. We will respond within 30 days. Residents of the European Economic Area, United Kingdom, and California have additional rights under GDPR, UK GDPR, and the CCPA respectively; please contact us for details.`,
  },
  {
    title: "9. Children's Privacy",
    body: `The Service is not directed to individuals under the age of 16. We do not knowingly collect personal information from children. If you believe we have inadvertently collected such information, please contact us immediately and we will delete it.`,
  },
  {
    title: "10. International Transfers",
    body: `Kollaber is headquartered in the United States. If you access the Service from outside the US, your information may be transferred to and processed in the US or other countries where our service providers operate. We rely on standard contractual clauses and other approved transfer mechanisms to protect data transferred out of the EEA or UK.`,
  },
  {
    title: "11. Third-Party Links",
    body: `The Service may contain links to third-party websites or integrations. This Privacy Policy does not apply to those third-party services, and we are not responsible for their privacy practices. We encourage you to review their privacy policies.`,
  },
  {
    title: "12. Changes to This Policy",
    body: `We may update this Privacy Policy from time to time. We will notify you of material changes via email or a prominent notice in the Service at least 14 days before the change takes effect. The "Effective date" at the top of this page will always reflect the date of the latest revision.`,
  },
  {
    title: "13. Contact Us",
    body: `For privacy-related questions, requests, or concerns, contact our privacy team at privacy@kollaber.io or by mail at: Kollaber Inc., 2093 Philadelphia Pike #7418, Claymont, DE 19703, United States.`,
  },
]

export default function PrivacyPage() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white font-sans antialiased">
      <nav className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#0a0a0a]/80 backdrop-blur-md">
        <div className="container mx-auto flex h-16 items-center justify-between px-6">
          <Link href="/" className="flex items-center gap-2">
            <Image src="/logo.png" alt="Kollaber" width={24} height={23} className="rounded" style={{ height: "auto" }} />
            <span className="text-lg font-bold tracking-tight">Kollaber</span>
          </Link>
          <div className="flex items-center gap-4 text-sm text-white/50">
            <Link href="/terms" className="transition-colors hover:text-white">Terms of Service</Link>
            <Link href="/login" className="transition-colors hover:text-white">Sign in</Link>
          </div>
        </div>
      </nav>

      <main className="container mx-auto max-w-3xl px-6 py-16">
        <div className="mb-12">
          <p className="text-xs font-medium uppercase tracking-widest text-white/30 mb-3">Legal</p>
          <h1 className="text-4xl font-bold tracking-tight">Privacy Policy</h1>
          <p className="mt-4 text-white/40 text-sm">Effective date: May 13, 2026</p>
          <p className="mt-6 text-white/60 leading-relaxed">
            Your privacy matters. This policy explains what data we collect, why we collect it, and how you can control it.
          </p>
        </div>

        <div className="space-y-10">
          {SECTIONS.map((section) => (
            <div key={section.title}>
              <h2 className="text-base font-semibold text-white mb-3">{section.title}</h2>
              {"body" in section && (
                <p className="text-sm leading-relaxed text-white/55">{section.body}</p>
              )}
              {"subsections" in section && section.subsections && (
                <ul className="space-y-4">
                  {section.subsections.map((sub) => (
                    <li key={sub.label} className="text-sm leading-relaxed text-white/55">
                      <span className="font-medium text-white/80">{sub.label}: </span>
                      {sub.text}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          ))}
        </div>
      </main>

      <footer className="mt-20 border-t border-white/10 py-10">
        <div className="container mx-auto flex flex-col items-center justify-between gap-4 px-6 sm:flex-row">
          <div className="flex items-center gap-2">
            <Image src="/logo.png" alt="Kollaber" width={18} height={17} className="rounded-sm opacity-50" style={{ height: "auto" }} />
            <span className="text-sm font-bold text-white/30">Kollaber</span>
            <span className="text-xs text-white/20">© 2026 Kollaber Inc.</span>
          </div>
          <div className="flex items-center gap-6 text-xs text-white/30">
            <Link href="/terms" className="transition-colors hover:text-white">Terms of Service</Link>
            <Link href="/privacy" className="text-white/50">Privacy Policy</Link>
            <a href="mailto:hello@kollaber.io" className="transition-colors hover:text-white">Contact</a>
          </div>
        </div>
      </footer>
    </div>
  )
}
