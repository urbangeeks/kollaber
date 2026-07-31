/**
 * Renders a JSON-LD block into the document.
 *
 * Next.js server-renders client components too, so this emits into the initial
 * HTML either way — which is what crawlers read.
 */
export function JsonLd({ schema }: { schema: Record<string, unknown> }) {
  return (
    <script
      type="application/ld+json"
      // Schema objects are built in-process from static data, never user input.
      dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }}
    />
  )
}
