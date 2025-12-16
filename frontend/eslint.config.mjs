import {dirname} from 'path';
import {fileURLToPath} from 'url';
import {FlatCompat} from '@eslint/eslintrc';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({
  baseDirectory: __dirname,
});

const eslintConfig = [
  // 基础配置
  ...compat.extends('next/core-web-vitals', 'next/typescript', 'google'),
  // prettier 配置必须放在最后，用于关闭所有与 prettier 冲突的规则
  ...compat.extends('prettier'),
  {
    rules: {
      // 关闭 JSDoc 相关规则
      'valid-jsdoc': 'off',
      'require-jsdoc': 'off',
      // curly 规则：要求 if/else 等必须使用大括号（保持代码清晰）
      'linebreak-style': 'off', // 如果您是windows系统，可在开发时禁用行尾换行符检查
      curly: ['error', 'all'],
    },
  },
];

export default eslintConfig;
