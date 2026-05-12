"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import {
  ArrowRight,
  Activity,
  Bell,
  Zap,
  Users,
  Webhook,
  CheckCircle2,
  Container,
  Check,
  Github,
} from "lucide-react"
import { AnimatedGradientText } from "@/components/ui/animated-gradient-text"
import { BlurFade } from "@/components/ui/blur-fade"
import { BorderBeam } from "@/components/ui/border-beam"
import { NumberTicker } from "@/components/ui/number-ticker"
import { Button } from "@/components/ui/button"

const FEATURES = [
  {
    icon: Activity,
    title: "Deploy Tracking",
    desc: "Every CI/CD run is automatically recorded in your shared team timeline.",
  },
  {
    icon: Zap,
    title: "Alert Correlation",
    desc: "Overlay CloudWatch, Datadog, or PagerDuty alerts directly onto your releases.",
  },
  {
    icon: Users,
    title: "Team Notes",
    desc: "Context matters. Drop manual markers for maintenance or configuration changes.",
  },
  {
    icon: Webhook,
    title: "CLI + Webhooks",
    desc: "First-class CLI tool and flexible webhooks for any tool in your stack.",
  },
  {
    icon: Container,
    title: "Kubernetes Native",
    desc: "Auto-detect rollouts from any cluster. Deploy the watcher via Helm in under a minute.",
  },
  {
    icon: Bell,
    title: "Slack, Teams & Email",
    desc: "Get notified where your team already works — Slack, Microsoft Teams, or email — the moment something happens.",
  },
]

const STATS = [
  { label: "binary to deploy", value: 1, prefix: "" },
  { label: "min to integrate", value: 5, prefix: "< " },
  { label: "event types tracked", value: 3, prefix: "" },
]

const STEPS = [
  {
    step: "01",
    title: "Deploy Kollaber",
    body: "Spin up the single binary or install the Helm chart to start watching your cluster.",
  },
  {
    step: "02",
    title: "Events flow in automatically",
    body: "Kubernetes rollouts, CLI deploys, and webhook alerts all land on the same timeline.",
  },
  {
    step: "03",
    title: "Filter, comment, and get notified",
    body: "Slice the timeline by type, service, or date range — and get Slack, Teams, or email alerts the moment something lands.",
  },
]

const PLANS = [
  {
    id: "free",
    name: "Free",
    price: "$0",
    per: "forever",
    description: "For small teams getting started.",
    cta: "Get started",
    highlight: false,
    features: ["Up to 5 members", "2 environments", "30-day history", "Webhook ingestion", "CLI tool", "Email notifications"],
  },
  {
    id: "team",
    name: "Team",
    price: "$12",
    per: "seat/mo",
    description: "Growing teams that need more room.",
    cta: "Start free trial",
    highlight: false,
    features: ["Up to 25 members", "Unlimited environments", "Unlimited history", "AI event summaries", "Slack & Teams alerts", "Email support"],
  },
  {
    id: "pro",
    name: "Pro",
    price: "$24",
    per: "seat/mo",
    description: "Engineering orgs running on Kubernetes.",
    cta: "Start free trial",
    highlight: true,
    badge: "Most popular",
    features: ["Unlimited members", "Unlimited environments", "Unlimited history", "AI summaries + postmortems", "Kubernetes watcher", "SSO · Audit logs", "Priority support"],
  },
  {
    id: "enterprise",
    name: "Enterprise",
    price: "Custom",
    per: "",
    description: "Dedicated support and custom contracts.",
    cta: "Contact us",
    highlight: false,
    features: ["Everything in Pro", "Volume seat pricing", "Dedicated Slack channel", "SLA guarantees", "Custom data retention", "On-prem option"],
  },
]

export default function Home() {
  const router = useRouter()

  useEffect(() => {
    if (process.env.NEXT_PUBLIC_SELF_HOSTED === "true") {
      router.replace("/login")
    }
  }, [router])

  if (process.env.NEXT_PUBLIC_SELF_HOSTED === "true") return null

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white selection:bg-[#60a5fa]/30 font-sans antialiased">

      {/* 1. STICKY NAV */}
      <nav className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#0a0a0a]/80 backdrop-blur-md">
        <div className="container mx-auto flex h-16 items-center justify-between px-6">
          <div className="flex items-center gap-2">
            <div className="h-6 w-6 rounded bg-gradient-to-br from-[#60a5fa] to-[#a78bfa]" />
            <span className="text-xl font-bold tracking-tight">Kollaber</span>
          </div>
          <div className="hidden items-center gap-8 text-sm font-medium text-white/70 md:flex">
            <a href="#features" className="transition-colors hover:text-white">Features</a>
            <a href="#how-it-works" className="transition-colors hover:text-white">How it works</a>
            <a href="#pricing" className="transition-colors hover:text-white">Pricing</a>
            <Link href="/docs" className="transition-colors hover:text-white">Docs</Link>
          </div>
          <div className="flex items-center gap-4">
            <a
              href="https://github.com/urbangeeks/kollaber"
              target="_blank"
              rel="noopener noreferrer"
              className="hidden text-white/50 transition-colors hover:text-white md:inline-flex"
            >
              <Github className="h-5 w-5" />
            </a>
            <Button asChild variant="ghost" className="hidden text-white/70 hover:text-white md:inline-flex">
              <Link href="/login">Sign in</Link>
            </Button>
            <Button asChild className="rounded-full bg-white text-black hover:bg-neutral-200">
              <Link href="/register">Get started free</Link>
            </Button>
          </div>
        </div>
      </nav>

      <main className="container mx-auto px-6">

        {/* 2. HERO */}
        <section className="relative flex flex-col items-center pt-14 pb-12 text-center sm:pt-20 sm:pb-16 md:pt-32 md:pb-20">
          <BlurFade delay={0.1}>
            <div className="mb-6 flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-4 py-1.5 text-xs font-medium">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
              </span>
              Now in early access
            </div>
          </BlurFade>

          <BlurFade delay={0.2}>
            <h1 className="max-w-4xl text-3xl font-bold tracking-tight sm:text-5xl md:text-7xl">
              The shared timeline for{" "}
              <AnimatedGradientText colorFrom="#60a5fa" colorTo="#a78bfa" speed={0.6}>
                infrastructure
              </AnimatedGradientText>{" "}
              teams
            </h1>
          </BlurFade>

          <BlurFade delay={0.3}>
            <p className="mt-8 max-w-2xl text-lg leading-relaxed text-white/50 md:text-xl">
              Stop correlating incidents in Slack. Capture deploys, alerts, and
              manual notes in one collaborative view designed for modern DevOps teams.
            </p>
          </BlurFade>

          <BlurFade delay={0.4} className="mt-10 flex flex-col items-center gap-4 sm:flex-row">
            <Button asChild size="lg" className="group rounded-full bg-white px-8 text-black hover:bg-neutral-200">
              <Link href="/register">
                Start for free
                <ArrowRight className="ml-2 h-4 w-4 transition-transform group-hover:translate-x-1" />
              </Link>
            </Button>
            <Button
              asChild
              variant="outline"
              size="lg"
              className="rounded-full border-white/10 bg-white/5 text-white hover:bg-white/10"
            >
              <a href="#how-it-works">See how it works</a>
            </Button>
          </BlurFade>

          {/* Terminal window */}
          <BlurFade delay={0.6} className="mt-20 w-full max-w-3xl overflow-hidden rounded-xl border border-white/10 bg-[#0d0d0d] shadow-2xl">
            <div className="relative">
              <BorderBeam colorFrom="#60a5fa" colorTo="#a78bfa" duration={8} />
              <div className="flex items-center gap-2 border-b border-white/10 bg-white/5 px-4 py-3">
                <div className="flex gap-1.5">
                  <div className="h-3 w-3 rounded-full bg-[#ff5f57]" />
                  <div className="h-3 w-3 rounded-full bg-[#febc2e]" />
                  <div className="h-3 w-3 rounded-full bg-[#28c840]" />
                </div>
                <div className="mx-auto font-mono text-[11px] text-white/30">bash — kollaber</div>
              </div>
              <div className="overflow-x-auto p-3 sm:p-6 text-left font-mono text-xs sm:text-sm leading-relaxed">
                <div>
                  <span className="text-white/40">$</span>{" "}
                  <span className="text-green-400">kollaber deploy --env production --service api</span>
                </div>
                <div className="text-white/40">Analyzing infrastructure state...</div>
                <div className="text-blue-400">→ Detected release v2.4.1 (hash: 8f2a1b)</div>
                <div className="text-white">✓ Snapshot captured</div>
                <div className="text-purple-400">✓ Timeline updated for team #platform-ops</div>
                <div className="mt-2 text-white/40">
                  View at:{" "}
                  <span className="text-white/60">https://kollaber.sh/t/8f2a1b</span>
                </div>
              </div>
            </div>
          </BlurFade>
        </section>

        {/* 4. STATS ROW */}
        <section className="py-20">
          <div className="grid grid-cols-1 gap-12 md:grid-cols-3">
            {STATS.map((stat, i) => (
              <BlurFade key={stat.label} delay={i * 0.1} inView className="text-center">
                <div className="text-5xl font-bold tracking-tighter md:text-7xl">
                  {stat.prefix}
                  <NumberTicker value={stat.value} className="text-white" />
                </div>
                <p className="mt-2 text-sm font-medium uppercase leading-loose tracking-widest text-white/40">
                  {stat.label}
                </p>
              </BlurFade>
            ))}
          </div>
        </section>

        {/* 5. FEATURES GRID */}
        <section id="features" className="py-24">
          <BlurFade inView className="mb-16 text-center">
            <h2 className="text-3xl font-bold md:text-4xl">
              Everything you need to{" "}
              <span className="text-[#60a5fa]">debug faster</span>
            </h2>
          </BlurFade>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {FEATURES.map((feature, i) => (
              <BlurFade
                key={feature.title}
                delay={i * 0.07}
                inView
                className="group relative overflow-hidden rounded-2xl border border-white/10 bg-white/5 p-5 sm:p-8"
              >
                <BorderBeam
                  delay={i * 2}
                  colorFrom="#60a5fa"
                  colorTo="#a78bfa"
                  duration={10}
                  size={40}
                />
                <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-[#60a5fa]/10 text-[#60a5fa]">
                  <feature.icon className="h-5 w-5" />
                </div>
                <h3 className="mb-2 font-bold">{feature.title}</h3>
                <p className="text-sm leading-relaxed text-white/50">{feature.desc}</p>
              </BlurFade>
            ))}
          </div>
        </section>

        {/* 6. HOW IT WORKS */}
        <section id="how-it-works" className="py-24">
          <BlurFade inView className="mb-16 text-center">
            <h2 className="text-3xl font-bold md:text-4xl">Up and running in minutes</h2>
          </BlurFade>
          <div className="grid grid-cols-1 gap-16 lg:grid-cols-3">
            {STEPS.map((s, i) => (
              <BlurFade key={s.step} delay={i * 0.15} inView>
                <div className="text-6xl font-bold tracking-tighter text-white/5">{s.step}</div>
                <h3 className="mt-4 text-xl font-bold">{s.title}</h3>
                <p className="mt-4 leading-relaxed text-white/50">{s.body}</p>
              </BlurFade>
            ))}
          </div>
        </section>

        {/* 7. CODE BLOCK SECTION */}
        <section className="grid items-center gap-16 py-24 lg:grid-cols-2">
          <BlurFade inView>
            <h2 className="text-3xl font-bold md:text-5xl">
              Integrate in <br />one command.
            </h2>
            <p className="mt-6 text-lg leading-relaxed text-white/50">
              Kollaber was built for developers. Our API and CLI are designed to
              fit into your existing workflows without adding friction.
            </p>
            <div className="mt-8 flex flex-col gap-4">
              {[
                "Zero-config ingestion",
                "Supports GH Actions, GitLab, Jenkins, and Kubernetes",
                "Slack, Teams, and email alerts built in",
              ].map((item) => (
                <div key={item} className="flex items-center gap-3 text-white/70">
                  <CheckCircle2 className="h-5 w-5 shrink-0 text-green-500" />
                  <span>{item}</span>
                </div>
              ))}
            </div>
          </BlurFade>

          <BlurFade delay={0.2} inView className="rounded-xl border border-white/10 bg-[#0d0d0d] p-1 shadow-2xl">
            <div className="rounded-lg bg-white/5 p-6 font-mono text-sm leading-relaxed">
              <div className="mb-4 text-white/30">{"// Fetch environment timeline"}</div>
              <div className="text-white">
                <span className="text-[#a78bfa]">kollaber</span>{" "}
                timeline
                <span className="text-blue-400"> --env</span>{" "}
                production
                <span className="text-blue-400"> --limit</span>{" "}
                3
              </div>
              <div className="mt-6 border-t border-white/10 pt-4 text-white/40">
                <div className="flex justify-between">
                  <span className="text-green-500">DEPLOY</span>
                  <span>14:20:01</span>
                </div>
                <div className="mb-4 text-white">v2.4.0 published by @alice</div>
                <div className="flex justify-between">
                  <span className="text-red-400">ALERT</span>
                  <span>14:25:12</span>
                </div>
                <div className="mb-4 text-white">HTTP 500 spike in us-east-1</div>
                <div className="flex justify-between">
                  <span className="text-blue-400">NOTE</span>
                  <span>14:26:05</span>
                </div>
                <div className="text-white">@bob: Investigating gateway timeouts</div>
              </div>
            </div>
          </BlurFade>
        </section>

        {/* 8. PRICING */}
        <section id="pricing" className="py-24">
          <BlurFade inView className="mb-16 text-center">
            <h2 className="text-3xl font-bold md:text-4xl">Simple, transparent pricing</h2>
            <p className="mt-4 text-white/50">Start free. Scale as your team grows. No per-environment charges.</p>
          </BlurFade>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {PLANS.map((plan, i) => (
              <BlurFade
                key={plan.id}
                delay={i * 0.07}
                inView
                className={`relative flex flex-col overflow-hidden rounded-2xl border p-5 sm:p-8 ${
                  plan.highlight
                    ? "border-[#a78bfa]/50 bg-[#a78bfa]/5"
                    : "border-white/10 bg-white/5"
                }`}
              >
                {plan.highlight && (
                  <BorderBeam colorFrom="#60a5fa" colorTo="#a78bfa" duration={10} />
                )}
                {plan.badge && (
                  <span className="mb-4 inline-block self-start rounded-full border border-[#a78bfa]/40 bg-[#a78bfa]/10 px-2.5 py-0.5 text-[11px] font-medium text-[#a78bfa]">
                    {plan.badge}
                  </span>
                )}
                <p className="text-sm font-semibold text-white/60">{plan.name}</p>
                <div className="mt-2 flex items-end gap-1">
                  <span className="text-4xl font-bold">{plan.price}</span>
                  {plan.per && <span className="mb-1 text-sm text-white/40">{plan.per}</span>}
                </div>
                <p className="mt-2 text-sm text-white/40">{plan.description}</p>

                <ul className="mt-6 flex-1 space-y-2">
                  {plan.features.map((f) => (
                    <li key={f} className="flex items-start gap-2 text-sm text-white/60">
                      <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-green-500" />
                      {f}
                    </li>
                  ))}
                </ul>

                <Link
                  href={plan.id === "enterprise" ? "mailto:hello@kollaber.io" : "/register"}
                  className={`mt-8 block rounded-full py-2 text-center text-sm font-medium transition-colors ${
                    plan.highlight
                      ? "bg-white text-black hover:bg-neutral-200"
                      : "border border-white/20 bg-white/5 text-white hover:bg-white/10"
                  }`}
                >
                  {plan.cta}
                </Link>
              </BlurFade>
            ))}
          </div>
        </section>

        {/* 9. CTA CARD */}
        <section className="py-24">
          <BlurFade inView className="relative overflow-hidden rounded-3xl border border-white/10 bg-white/5 px-4 py-12 text-center sm:px-8 sm:py-16 md:px-20 md:py-20">
            <BorderBeam duration={12} size={400} colorFrom="#60a5fa" colorTo="#a78bfa" />
            <h2 className="text-4xl font-bold tracking-tight md:text-5xl">
              Stop flying blind on your infrastructure
            </h2>
            <p className="mx-auto mt-6 max-w-xl text-lg leading-relaxed text-white/50">
              Join DevOps teams using Kollaber to bring clarity to their production environments.
            </p>
            <div className="mt-10">
              <Button asChild size="lg" className="group rounded-full bg-white px-10 text-black hover:bg-neutral-200">
                <Link href="/register">
                  Create free account
                  <ArrowRight className="ml-2 h-5 w-5 transition-transform group-hover:translate-x-1" />
                </Link>
              </Button>
            </div>
          </BlurFade>
        </section>
      </main>

      {/* FOOTER */}
      <footer className="mt-20 border-t border-white/10 py-12">
        <div className="container mx-auto flex flex-col items-center justify-between gap-6 px-6 md:flex-row">
          <div className="flex items-center gap-2">
            <div className="h-4 w-4 rounded-sm bg-gradient-to-br from-[#60a5fa] to-[#a78bfa]" />
            <span className="font-bold text-white/40">Kollaber</span>
            <span className="text-xs text-white/20">© 2026 Kollaber Inc.</span>
          </div>
          <div className="flex items-center gap-8 text-sm font-medium text-white/30">
            <a href="#pricing" className="transition-colors hover:text-white">Pricing</a>
            <Link href="/docs" className="transition-colors hover:text-white">Docs</Link>
            {["Terms", "Privacy"].map((label) => (
              <a key={label} href="#" className="transition-colors hover:text-white">{label}</a>
            ))}
          </div>
          <div className="flex items-center gap-4 text-sm font-medium text-white/30">
            <Link href="/login" className="transition-colors hover:text-white">Sign in</Link>
            <Link href="/register" className="transition-colors hover:text-white">Sign up</Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
