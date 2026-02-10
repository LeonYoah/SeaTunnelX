/*
 * @Author: Leon Yoah 1733839298@qq.com
 * @Date: 2025-12-16 15:49:24
 * @LastEditors: Leon Yoah 1733839298@qq.com
 * @LastEditTime: 2026-02-11 00:55:34
 * @FilePath: \SeaTunnelX\frontend\next.config.ts
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
import type {NextConfig} from 'next';

const nextConfig: NextConfig = {
  /* config options here */
  eslint: {
    // 只在构建时跳过 ESLint，开发时照常在编辑器里提示
    ignoreDuringBuilds: true,
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.NEXT_PUBLIC_BACKEND_BASE_URL || 'http://localhost:8000'}/api/:path*`,
      },
    ];
  },

  // 确保代理请求正确传递headers和cookie
  async headers() {
    return [
      {
        source: '/api/:path*',
        headers: [
          {
            key: 'Access-Control-Allow-Credentials',
            value: 'true',
          },
          {
            key: 'Access-Control-Allow-Origin',
            value:
              process.env.NEXT_PUBLIC_FRONTEND_BASE_URL ||
              'http://localhost:3000',
          },
          {
            key: 'Access-Control-Allow-Methods',
            value: 'GET, POST, PUT, DELETE, OPTIONS',
          },
          {
            key: 'Access-Control-Allow-Headers',
            value: 'Content-Type, Authorization, Cookie',
          },
        ],
      },
    ];
  },
};

export default nextConfig;
