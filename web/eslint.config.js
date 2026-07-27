import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import prettier from 'eslint-config-prettier';
import globals from 'globals';
import svelteConfig from './svelte.config.js';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs.recommended,
  prettier,
  ...svelte.configs.prettier,
  {
    languageOptions: {
      globals: { ...globals.browser, ...globals.node }
    },
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'warn',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }
      ],
      // Baseline debt: these rules fire on pre-existing code (not introduced by
      // the token/primitives migration). Start them at `warn` so the new design
      // gate (check:tokens) and build stay green, and ratchet each to `error`
      // as the existing violations are cleaned up.
      'svelte/require-each-key': 'warn',
      'svelte/prefer-svelte-reactivity': 'warn',
      'svelte/no-navigation-without-resolve': 'warn',
      '@typescript-eslint/no-explicit-any': 'warn',
      'no-empty': 'warn'
    }
  },
  {
    // Svelte files are parsed by svelte-eslint-parser with the TS parser for
    // <script lang="ts">. a11y diagnostics come from the Svelte compiler via
    // `npm run check --threshold warning`; this layer adds the JS/TS lint rules.
    files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
    languageOptions: {
      parserOptions: {
        parser: ts.parser,
        svelteConfig
      }
    }
  },
  {
    ignores: ['build/', '.svelte-kit/', 'dist/', 'node_modules/']
  }
);
