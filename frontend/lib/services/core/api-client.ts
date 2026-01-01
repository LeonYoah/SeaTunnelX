import axios, {AxiosError, AxiosResponse} from 'axios';
import {ApiError, ApiResponse} from './types';

/**
 * API客户端实例
 * API client instance
 * 统一处理请求配置、响应解析和错误处理
 * Unified handling of request configuration, response parsing, and error handling
 */
const apiClient = axios.create({
  baseURL: '/api/v1', // API 基础路径 / API base path
  timeout: 60000, // 60 seconds for plugin fetching from Maven / 60秒超时，用于从 Maven 获取插件
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
});

/**
 * 请求拦截器
 * 确保所有请求带上凭证
 */
apiClient.interceptors.request.use(
  (config) => {
    config.withCredentials = true;
    return config;
  },
  (error) => Promise.reject(error),
);

/**
 * 直接启动OAuth登录流程
 * Initiate OAuth login flow
 * @param currentPath - 当前路径，用于登录成功后重定向回来 / Current path for redirect after login
 */
function initiateLogin(currentPath: string): Promise<never> {
  // 防止循环重定向 / Prevent circular redirect
  if (
    !currentPath.startsWith('/login') &&
    !currentPath.startsWith('/callback')
  ) {
    // 动态导入AuthService避免循环依赖 / Dynamic import to avoid circular dependency
    import('../auth/auth.service').then(({AuthService}) => {
      // 调用OAuth登录方法，传入当前路径作为重定向目标 / Call OAuth login with current path as redirect target
      AuthService.loginWithOAuth(undefined, currentPath);
    });
  }

  // 返回永不解决的Promise / Return never-resolving Promise
  return new Promise<never>(() => {});
}

/**
 * 响应拦截器
 * 处理API响应和统一错误处理
 */
apiClient.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => response,
  (error: AxiosError<ApiError>) => {
    // 处理401未授权错误
    if (error.response?.status === 401) {
      return initiateLogin(window.location.pathname);
    }

    // 处理后端返回的错误信息
    if (error.response?.data?.error_msg) {
      const apiError = new Error(error.response.data.error_msg);
      apiError.name = 'ApiError';
      return Promise.reject(apiError);
    }

    // 处理网络错误
    if (error.code === 'ECONNABORTED') {
      return Promise.reject(new Error('请求超时，请检查网络连接'));
    }

    // 处理权限错误
    if (error.response?.status === 403) {
      return Promise.reject(new Error('权限不足'));
    }

    // 处理服务器错误
    if (error.response && error.response.status >= 500) {
      return Promise.reject(new Error('服务器内部错误，请稍后重试'));
    }

    return Promise.reject(new Error(error.message || '网络请求失败'));
  },
);

export default apiClient;
