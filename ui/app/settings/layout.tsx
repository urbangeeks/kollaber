import { SettingsNav } from "@/components/settings-nav"

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen">
      <div className="container mx-auto flex gap-10 px-6 py-12">
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
