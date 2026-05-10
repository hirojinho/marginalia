// eslint.config.js — flat config for ESLint 9
import globals from 'globals';

export default [
  {
    files: ['static/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      'no-var': 'error',
      'prefer-const': 'error',
      'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      'no-undef': 'error',
      'no-console': ['error', { allow: ['warn', 'error'] }],
      eqeqeq: ['error', 'always'],
      'no-implicit-globals': 'error',
      'no-prototype-builtins': 'error',
      'no-throw-literal': 'error',
    },
  },
];
