/**
 * Host Management Hook
 * 主机管理 Hook
 *
 * Provides state management and data fetching for host management.
 * 提供主机管理的状态管理和数据获取功能。
 */

import {useState, useEffect, useCallback, useMemo} from 'react';
import services from '@/lib/services';
import {
  HostInfo,
  HostType,
  HostStatus,
  AgentStatus,
  ListHostsRequest,
  HostListData,
} from '@/lib/services/host/types';

/**
 * Host filter state
 * 主机过滤状态
 */
export interface HostFilter {
  name?: string;
  hostType?: HostType;
  status?: HostStatus;
  agentStatus?: AgentStatus;
  isOnline?: boolean;
}

/**
 * Host hook return type
 * 主机 Hook 返回类型
 */
interface UseHostReturn {
  /** Host list / 主机列表 */
  hosts: HostInfo[];
  /** Total count / 总数量 */
  total: number;
  /** Loading state / 加载状态 */
  isLoading: boolean;
  /** Error state / 错误状态 */
  error: string | null;
  /** Current page / 当前页码 */
  currentPage: number;
  /** Page size / 每页数量 */
  pageSize: number;
  /** Total pages / 总页数 */
  totalPages: number;
  /** Current filter / 当前过滤条件 */
  filter: HostFilter;
  /** Set current page / 设置当前页码 */
  setCurrentPage: (page: number) => void;
  /** Set filter / 设置过滤条件 */
  setFilter: (filter: HostFilter) => void;
  /** Refresh data / 刷新数据 */
  refresh: () => Promise<void>;
  /** Clear filter / 清除过滤条件 */
  clearFilter: () => void;
}


/**
 * Cache structure
 * 缓存结构
 */
interface CachedData {
  data: HostListData;
  timestamp: number;
}

// Global cache - shared across all component instances
// 全局缓存 - 所有组件实例共享
const hostCache = new Map<string, CachedData>();
const CACHE_DURATION = 30 * 1000; // 30 seconds cache duration / 30秒缓存过期时间

/**
 * Generate cache key from request params
 * 从请求参数生成缓存键
 */
function getCacheKey(params: ListHostsRequest): string {
  return JSON.stringify(params);
}

/**
 * Check if cache is valid
 * 检查缓存是否有效
 */
function isCacheValid(cachedData: CachedData): boolean {
  return Date.now() - cachedData.timestamp < CACHE_DURATION;
}

/**
 * Default page size
 * 默认每页数量
 */
const DEFAULT_PAGE_SIZE = 10;

/**
 * Host management hook
 * 主机管理 Hook
 *
 * @param initialPageSize - Initial page size / 初始每页数量
 * @returns Host management state and methods / 主机管理状态和方法
 */
export function useHost(initialPageSize: number = DEFAULT_PAGE_SIZE): UseHostReturn {
  // State / 状态
  const [hosts, setHosts] = useState<HostInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize] = useState(initialPageSize);
  const [filter, setFilter] = useState<HostFilter>({});

  /**
   * Calculate total pages
   * 计算总页数
   */
  const totalPages = useMemo(() => Math.ceil(total / pageSize), [total, pageSize]);

  /**
   * Build request params from current state
   * 从当前状态构建请求参数
   */
  const buildRequestParams = useCallback((): ListHostsRequest => {
    return {
      current: currentPage,
      size: pageSize,
      name: filter.name,
      host_type: filter.hostType,
      status: filter.status,
      agent_status: filter.agentStatus,
      is_online: filter.isOnline,
    };
  }, [currentPage, pageSize, filter]);

  /**
   * Fetch hosts data
   * 获取主机数据
   */
  const fetchHosts = useCallback(
    async (forceRefresh = false) => {
      const params = buildRequestParams();
      const cacheKey = getCacheKey(params);

      // Check cache first (unless force refresh)
      // 首先检查缓存（除非强制刷新）
      if (!forceRefresh) {
        const cached = hostCache.get(cacheKey);
        if (cached && isCacheValid(cached)) {
          setHosts(cached.data.hosts);
          setTotal(cached.data.total);
          setIsLoading(false);
          setError(null);
          return;
        }
      }

      setIsLoading(true);
      setError(null);

      try {
        const result = await services.host.getHostsSafe(params);

        if (result.success && result.data) {
          setHosts(result.data.hosts);
          setTotal(result.data.total);

          // Update cache / 更新缓存
          hostCache.set(cacheKey, {
            data: result.data,
            timestamp: Date.now(),
          });
        } else {
          setError(result.error || '获取主机列表失败');
          setHosts([]);
          setTotal(0);
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : '获取主机列表失败';
        setError(errorMessage);
        setHosts([]);
        setTotal(0);
      } finally {
        setIsLoading(false);
      }
    },
    [buildRequestParams],
  );

  /**
   * Refresh data (force refresh, ignore cache)
   * 刷新数据（强制刷新，忽略缓存）
   */
  const refresh = useCallback(async () => {
    // Clear all cache when refreshing
    // 刷新时清除所有缓存
    hostCache.clear();
    await fetchHosts(true);
  }, [fetchHosts]);

  /**
   * Clear filter
   * 清除过滤条件
   */
  const clearFilter = useCallback(() => {
    setFilter({});
    setCurrentPage(1);
  }, []);

  /**
   * Handle filter change - reset to page 1
   * 处理过滤条件变化 - 重置到第1页
   */
  const handleSetFilter = useCallback((newFilter: HostFilter) => {
    setFilter(newFilter);
    setCurrentPage(1);
  }, []);

  /**
   * Fetch data when dependencies change
   * 当依赖变化时获取数据
   */
  useEffect(() => {
    fetchHosts();
  }, [fetchHosts]);

  return {
    hosts,
    total,
    isLoading,
    error,
    currentPage,
    pageSize,
    totalPages,
    filter,
    setCurrentPage,
    setFilter: handleSetFilter,
    refresh,
    clearFilter,
  };
}

/**
 * Single host hook return type
 * 单个主机 Hook 返回类型
 */
interface UseHostDetailReturn {
  /** Host data / 主机数据 */
  host: HostInfo | null;
  /** Loading state / 加载状态 */
  isLoading: boolean;
  /** Error state / 错误状态 */
  error: string | null;
  /** Refresh data / 刷新数据 */
  refresh: () => Promise<void>;
}

/**
 * Single host detail hook
 * 单个主机详情 Hook
 *
 * @param hostId - Host ID / 主机 ID
 * @returns Host detail state and methods / 主机详情状态和方法
 */
export function useHostDetail(hostId: number | null): UseHostDetailReturn {
  const [host, setHost] = useState<HostInfo | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  /**
   * Fetch host detail
   * 获取主机详情
   */
  const fetchHost = useCallback(async () => {
    if (!hostId) {
      setHost(null);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const result = await services.host.getHostSafe(hostId);

      if (result.success && result.data) {
        setHost(result.data);
      } else {
        setError(result.error || '获取主机详情失败');
        setHost(null);
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : '获取主机详情失败';
      setError(errorMessage);
      setHost(null);
    } finally {
      setIsLoading(false);
    }
  }, [hostId]);

  /**
   * Refresh data
   * 刷新数据
   */
  const refresh = useCallback(async () => {
    await fetchHost();
  }, [fetchHost]);

  /**
   * Fetch data when hostId changes
   * 当 hostId 变化时获取数据
   */
  useEffect(() => {
    fetchHost();
  }, [fetchHost]);

  return {
    host,
    isLoading,
    error,
    refresh,
  };
}
