import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  // AIDEV-NOTE: '.remember' is a local session-state artifact dir (gitignored);
  // ESLint flat config does not read .gitignore, so ignore it explicitly.
  { ignores: ['dist', '.remember'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
      // AIDEV-NOTE: Keep stray debug logging out of the shipped app. console.warn/
      // console.error are allowed for genuine error paths (ErrorBoundary, config
      // validation); console.log/debug/info are errors. Relaxed for tests/e2e/tooling
      // in the override block below. The prod build also drops all console via esbuild.
      'no-console': ['error', { allow: ['warn', 'error'] }],
    },
  },
  // Tests, e2e specs, and build/tooling configs may log freely.
  {
    files: [
      '**/*.test.{ts,tsx}',
      'src/test/**',
      'e2e/**',
      '*.config.{ts,js}',
    ],
    rules: {
      'no-console': 'off',
    },
  },
)
