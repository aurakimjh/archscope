// ─────────────────────────────────────────────────────────────────────
// [한글] eslint.config.js — flat config. 핵심 규칙:
//   - consistent-type-imports: 타입 전용 import 는 `import type`.
//   - no-unused-vars: `_` prefix 변수는 의도적 미사용으로 허용.
//   - dist/dist-electron/node_modules 는 lint 제외.
// ─────────────────────────────────────────────────────────────────────
import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: ["dist", "dist-electron", "node_modules"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["src/**/*.ts", "src/**/*.tsx", "electron/**/*.ts"],
    rules: {
      "@typescript-eslint/consistent-type-imports": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
);
