import { SettingsNav, SettingsNavMobile } from "@/components/settings-nav"

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen">
      {/* Mobile horizontal nav — visible below lg */}
      <div className="lg:hidden border-b overflow-x-auto scrollbar-none">
        <div className="px-4 py-2.5 min-w-max">
          <SettingsNavMobile />
        </div>
      </div>

      <div className="container mx-auto flex gap-10 px-4 py-8 sm:px-6 sm:py-10 lg:px-6 lg:py-12">
        <aside className="hidden w-44 shrink-0 lg:block">
          <div className="sticky top-16">
            <SettingsNav />
          </div>
        </aside>
        <main className="min-w-0 flex-1 max-w-lg">
          {children}
        </main>
      </div>
    </div>
  )
}
