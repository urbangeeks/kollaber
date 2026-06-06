import { cn } from "@/lib/utils"

/**
 * DotBackground renders a subtle radial-dot grid behind page content
 * (Aceternity / Magic UI style). It's a fixed, non-interactive layer, so the
 * page container above it must be transparent for the dots to show through.
 *
 * `faded` applies a radial mask so the grid gently fades toward the edges.
 */
export function DotBackground({
  className,
  faded = true,
}: {
  className?: string
  faded?: boolean
}) {
  return (
    <div
      aria-hidden
      className={cn(
        "pointer-events-none fixed inset-0 -z-10",
        "[background-size:22px_22px]",
        "[background-image:radial-gradient(rgba(0,0,0,0.14)_1px,transparent_1px)]",
        "dark:[background-image:radial-gradient(rgba(255,255,255,0.13)_1px,transparent_1px)]",
        faded &&
          "[mask-image:radial-gradient(ellipse_at_center,black_55%,transparent_100%)]",
        className,
      )}
    />
  )
}
