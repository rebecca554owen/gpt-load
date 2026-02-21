import js from "@eslint/js";
import configPrettier from "@vue/eslint-config-prettier";
import configTypeScript from "@vue/eslint-config-typescript";
import pluginVue from "eslint-plugin-vue";

export default [
  {
    name: "app/files-to-lint",
    files: ["**/*.{js,mjs,ts,mts,tsx,vue}"],
  },

  {
    name: "app/files-to-ignore",
    ignores: ["**/dist/**", "**/dist-ssr/**", "**/coverage/**", "**/node_modules/**", "**/*.d.ts"],
  },

  // Base configurations
  js.configs.recommended,
  ...pluginVue.configs["flat/essential"],
  ...configTypeScript(),
  configPrettier,

  {
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
    },
    rules: {
      // Vue 规则
      "vue/multi-word-component-names": "off", // 允许单词组件名，适应现有代码
      "vue/no-unused-vars": "error",
      "vue/no-unused-components": "warn",
      "vue/component-definition-name-casing": ["error", "PascalCase"],
      "vue/component-name-in-template-casing": ["warn", "kebab-case"],
      "vue/prop-name-casing": ["error", "camelCase"],
      "vue/attribute-hyphenation": ["error", "always"],
      "vue/v-on-event-hyphenation": ["error", "always"],
      "vue/html-self-closing": [
        "warn",
        {
          html: {
            void: "always",
            normal: "always",
            component: "always",
          },
          svg: "always",
          math: "always",
        },
      ],
      "vue/max-attributes-per-line": "off",
      "vue/singleline-html-element-content-newline": "off",
      "vue/multiline-html-element-content-newline": "off",
      "vue/html-indent": "off", // Let Prettier handle Vue template indentation
      "vue/script-indent": [
        "error",
        2,
        {
          baseIndent: 0,
          switchCase: 1,
          ignores: [],
        },
      ],
      "vue/component-tags-order": "off",

      // Vue 3 Composition API 规则
      "vue/no-setup-props-destructure": "off",
      "vue/prefer-import-from-vue": "off",
      "vue/no-deprecated-slot-attribute": "off",
      "vue/no-deprecated-slot-scope-attribute": "off",

      // TypeScript 规则
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
        },
      ],
      "@typescript-eslint/explicit-function-return-type": "off",
      "@typescript-eslint/explicit-module-boundary-types": "off",
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-non-null-assertion": "warn",
      "@typescript-eslint/no-unused-expressions": "error",

      // 通用 JavaScript/TypeScript 规则
      "no-console": ["warn", { allow: ["warn", "error"] }],
      "no-debugger": "warn",
      "prefer-const": "error",
      "no-var": "error",
      "no-unused-vars": "off", // 使用 TypeScript 版本
      eqeqeq: ["error", "always"],
      curly: ["error", "all"],
      "no-throw-literal": "error",
      "prefer-promise-reject-errors": "error",

      // 开源项目最佳实践
      "no-eval": "error",
      "no-implied-eval": "error",
      "no-new-func": "error",
      "no-script-url": "error",
      "no-alert": "warn",
      "no-duplicate-imports": "error",
      "prefer-template": "error",
      "object-shorthand": "error",
      "prefer-arrow-callback": "error",
      "arrow-spacing": "error",
      "no-useless-return": "error",
    },
  },
];
