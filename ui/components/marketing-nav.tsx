import Link from "next/link"
import Image from "next/image"

/** Top bar for the public marketing pages, matching the docs header. */
export function MarketingNav({ label }: { label?: string }) {
  return (
    <nav className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#0a0a0a]/80 backdrop-blur-md">
      <div className="container mx-auto flex h-16 items-center justify-between px-6">
        <div className="flex items-center gap-6">
          <Link href="/" className="flex items-center gap-2">
            <Image
              src="/logo.png"
              alt="Kollaber"
              width={28}
              height={27}
              className="rounded"
              style={{ height: "auto" }}
            />
            <span className="text-xl font-bold tracking-tight">Kollaber</span>
          </Link>
          {label && <span className="hidden text-sm text-white/30 sm:block">{label}</span>}
        </div>
        <div className="flex items-center gap-4">
          <Link href="/docs" className="hidden text-sm text-white/60 transition-colors hover:text-white sm:block">
            Docs
          </Link>
          <Link href="/login" className="text-sm text-white/60 transition-colors hover:text-white">
            Sign in
          </Link>
          <Link
            href="/register"
            className="rounded-full bg-white px-4 py-1.5 text-sm font-medium text-black transition-colors hover:bg-neutral-200"
          >
            Get started free
          </Link>
        </div>
      </div>
    </nav>
  )
}
