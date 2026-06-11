// ESLint 10 flat config. Vue 3 + TypeScript + Prettier-compat.
// Run via `npm run lint` (fix mode) or `npx eslint .` (check mode).

import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.{ts,mts,tsx,vue}'],
  },

  {
    name: 'app/files-to-ignore',
    ignores: ['**/dist/**', '**/dist-ssr/**', '**/coverage/**'],
  },

  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,

  // Disable any rule that conflicts with Prettier — Prettier owns formatting.
  skipFormatting,

  {
    rules: {
      // Views in this codebase use single-word names like ServersView; that's
      // the conventional admin-template pattern. The Vue default rule warns
      // because single-word names can clash with future native HTML elements,
      // but in our scoped admin SPA that risk is essentially zero.
      'vue/multi-word-component-names': 'off',

      // Warn-not-error so dead imports surface during refactoring without
      // breaking the build. Underscore-prefixed args are intentional unused.
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
    },
  },
)
