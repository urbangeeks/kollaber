import Link from "next/link"
import Image from "next/image"
import type { Metadata } from "next"

const description =
  "The terms governing use of Kollaber, including accounts, acceptable use, billing, and liability."

export const metadata: Metadata = {
  title: "Terms of Service",
  description,
  alternates: { canonical: "/terms" },
  openGraph: { title: "Terms of Service", description, url: "/terms" },
  twitter: { title: "Terms of Service", description },
}

const SECTIONS = [
  {
    title: "1. Acceptance of Terms",
    body: `By accessing or using Kollaber ("Service"), you agree to be bound by these Terms of Service ("Terms"). If you do not agree, do not use the Service. These Terms apply to all visitors, users, and others who access the Service.`,
  },
  {
    title: "2. Description of Service",
    body: `Kollaber provides a shared timeline and collaboration platform for infrastructure teams. The Service enables teams to capture, annotate, and query infrastructure events including deployments, alerts, and manual notes. We reserve the right to modify or discontinue the Service at any time with reasonable notice.`,
  },
  {
    title: "3. Accounts",
    body: `You must create an account to use most features of the Service. You are responsible for maintaining the confidentiality of your credentials and for all activity that occurs under your account. You must provide accurate information and keep it up to date. You may not share your account with others or use another person's account. Notify us immediately at hello@kollaber.io if you suspect unauthorized access.`,
  },
  {
    title: "4. Acceptable Use",
    body: `You agree not to use the Service to: (a) violate any applicable law or regulation; (b) transmit malicious code or interfere with the Service's integrity; (c) attempt to gain unauthorized access to any part of the Service or its infrastructure; (d) scrape or harvest data without our prior written consent; (e) resell or sublicense access to the Service without authorization. We may suspend or terminate accounts that violate these restrictions.`,
  },
  {
    title: "5. Subscriptions and Billing",
    body: `Paid plans are billed in advance on a monthly or annual basis. All fees are non-refundable except as required by applicable law or as expressly stated in our refund policy. We may change pricing with 30 days' notice. Failure to pay may result in suspension of your account. All prices are in USD unless otherwise specified.`,
  },
  {
    title: "6. Data and Content",
    body: `You retain ownership of all data and content you submit to the Service ("Your Content"). By submitting Your Content, you grant Kollaber a limited, worldwide, royalty-free license to store, process, and display Your Content solely for the purpose of providing the Service. We will not sell Your Content to third parties. You are solely responsible for ensuring Your Content does not violate any laws or third-party rights.`,
  },
  {
    title: "7. Intellectual Property",
    body: `Kollaber and its licensors own all rights in the Service, including the software, design, trademarks, and documentation. These Terms do not grant you any rights to use our trademarks or branding. Feedback or suggestions you provide may be used by us without obligation to you.`,
  },
  {
    title: "8. Confidentiality",
    body: `Each party agrees to keep the other party's non-public information confidential and not to disclose it to third parties, except as required by law or with prior written consent. This obligation does not apply to information that is publicly available, independently developed, or lawfully obtained from a third party.`,
  },
  {
    title: "9. Disclaimers",
    body: `THE SERVICE IS PROVIDED "AS IS" AND "AS AVAILABLE" WITHOUT WARRANTIES OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. KOLLABER DOES NOT WARRANT THAT THE SERVICE WILL BE UNINTERRUPTED, ERROR-FREE, OR SECURE.`,
  },
  {
    title: "10. Limitation of Liability",
    body: `TO THE MAXIMUM EXTENT PERMITTED BY LAW, KOLLABER SHALL NOT BE LIABLE FOR ANY INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, OR PUNITIVE DAMAGES, OR ANY LOSS OF PROFITS OR REVENUES, WHETHER INCURRED DIRECTLY OR INDIRECTLY. IN NO EVENT SHALL OUR AGGREGATE LIABILITY EXCEED THE GREATER OF (A) THE AMOUNT YOU PAID US IN THE TWELVE MONTHS PRIOR TO THE CLAIM OR (B) $100 USD.`,
  },
  {
    title: "11. Indemnification",
    body: `You agree to indemnify and hold harmless Kollaber, its officers, directors, employees, and agents from any claims, damages, losses, or expenses (including reasonable attorneys' fees) arising out of your use of the Service, Your Content, or your violation of these Terms.`,
  },
  {
    title: "12. Termination",
    body: `Either party may terminate these Terms at any time. You may cancel your account from Settings. We may suspend or terminate your access immediately for violations of these Terms. Upon termination, your right to use the Service ceases and we may delete Your Content after a reasonable retention period in accordance with our Privacy Policy.`,
  },
  {
    title: "13. Governing Law",
    body: `These Terms are governed by the laws of the State of Delaware, United States, without regard to its conflict-of-law provisions. Any disputes shall be resolved exclusively in the state or federal courts located in Delaware, and you consent to personal jurisdiction there.`,
  },
  {
    title: "14. Changes to Terms",
    body: `We may update these Terms from time to time. We will notify you of material changes via email or a prominent notice in the Service at least 14 days before the change takes effect. Your continued use of the Service after the effective date constitutes acceptance of the updated Terms.`,
  },
  {
    title: "15. Contact",
    body: `For questions about these Terms, contact us at hello@kollaber.io or write to: Kollaber Inc., 2093 Philadelphia Pike #7418, Claymont, DE 19703, United States.`,
  },
]

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white font-sans antialiased">
      <nav className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#0a0a0a]/80 backdrop-blur-md">
        <div className="container mx-auto flex h-16 items-center justify-between px-6">
          <Link href="/" className="flex items-center gap-2">
            <Image src="/logo.png" alt="Kollaber" width={24} height={23} className="rounded" style={{ height: "auto" }} />
            <span className="text-lg font-bold tracking-tight">Kollaber</span>
          </Link>
          <div className="flex items-center gap-4 text-sm text-white/50">
            <Link href="/privacy" className="transition-colors hover:text-white">Privacy Policy</Link>
            <Link href="/login" className="transition-colors hover:text-white">Sign in</Link>
          </div>
        </div>
      </nav>

      <main className="container mx-auto max-w-3xl px-6 py-16">
        <div className="mb-12">
          <p className="text-xs font-medium uppercase tracking-widest text-white/30 mb-3">Legal</p>
          <h1 className="text-4xl font-bold tracking-tight">Terms of Service</h1>
          <p className="mt-4 text-white/40 text-sm">Effective date: May 13, 2026</p>
          <p className="mt-6 text-white/60 leading-relaxed">
            These Terms of Service govern your access to and use of the Kollaber platform. Please read them carefully before using the Service.
          </p>
        </div>

        <div className="space-y-10">
          {SECTIONS.map((section) => (
            <div key={section.title}>
              <h2 className="text-base font-semibold text-white mb-3">{section.title}</h2>
              <p className="text-sm leading-relaxed text-white/55">{section.body}</p>
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
            <Link href="/terms" className="text-white/50">Terms of Service</Link>
            <Link href="/privacy" className="transition-colors hover:text-white">Privacy Policy</Link>
            <a href="mailto:hello@kollaber.io" className="transition-colors hover:text-white">Contact</a>
          </div>
        </div>
      </footer>
    </div>
  )
}
