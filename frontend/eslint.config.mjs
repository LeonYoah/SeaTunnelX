import {dirname} from 'path';
import {fileURLToPath} from 'url';
import {FlatCompat} from '@eslint/eslintrc';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({
  baseDirectory: __dirname,
});

const eslintConfig = [
  ...compat.extends(
      'next/core-web-vitals',
      'next/typescript',
      'google',
  ),
  {
    rules: {
      'valid-jsdoc': 'off',
      'require-jsdoc': 'off',
      'max-len': 'off',
      // 'linebreak-style': 'off', // 如果您是windows系统，可在开发时禁用行尾换行符检查
    },
  },
];

export default eslintConfig;
