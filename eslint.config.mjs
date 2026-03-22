// @ts-check
import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';
import globals from 'globals';

export default tseslint.config(
    // 1. Recommended JS rules
    eslint.configs.recommended,
    // 2. Recommended TS rules
    ...tseslint.configs.recommended,
    {
        languageOptions: {
            globals: {
                ...globals.node, // Tells ESLint this is a Node.js project (handles 'process', etc.)
            },
        },
        rules: {
            // Custom overrides for JSTS-SaleBot
            '@typescript-eslint/no-explicit-any': 'warn', // Don't fail the build for 'any'
            'no-console': 'off', // Bots need console.log for tracking events
        },
    },
    {
        // 3. Files to ignore (replaces .eslintignore)
        ignores: ['dist/**', 'node_modules/**', 'docs/**'],
    }
);