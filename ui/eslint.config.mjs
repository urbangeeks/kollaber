import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // "npm run build" copies out/ into dist/ for the Go binary to embed, so
    // dist/ is generated output too — without this, lint reports thousands of
    // problems against a duplicate of an already-ignored directory.
    "dist/**",
  ]),
]);

export default eslintConfig;
