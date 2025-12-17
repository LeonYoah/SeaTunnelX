/**
 * Audit Management Hook
 * 审计管理 Hook
 *
 * Provides state management and data fetching for audit and command logs.
 * 提供审计日志和命令日志的状态管理和数据获取功能。
 */

import {useState, useEffect, useCallback, useMemo} from 'react';
import services from '@/lib/services';
import {
  CommandLogInfo,
  AuditLogInfo,
  CommandStatus,
  ListCommandLogsRequest,
  ListAuditLogsRequest,
  CommandLogListData,
  AuditLogListData,
} from '@/lib/services/audit/types';

// ==================== Command Log Hook 命令日志 Hook ====================

/**
 * Command log filter state
 * 命令日志过滤状态
 */
export interface CommandLogFilter {
  commandId?: string;
  agentId?: string;
  hostId?: number;
  commandType?: string;
  status?: CommandStatus;
  startTime?: string;
  endTime?: string;
}

/**
 * Command log hook return type
 * 命令日志 Hook 返回类型
 */
interface UseCommandLogsReturn {
  /** Command log list / 命令日志列表 */
  commands: CommandLogInfo[];
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
  filter: CommandLogFilter;
  /** Set current page / 设置当前页码 */
  setCurrentPage: (page: number) => void;
  /** Set filter / 设置过滤条件 */
  setFilter: (filter: CommandLogFilter) => void;
  /** Refresh data / 刷新数据 */
  refresh: () => Promise<void>;
  /** Clear filter / 清除过滤条件 */
  clearFilter: () => void;
}


/**
 * Cache structure for command logs
 * 命令日志缓存结构
 */
interface CachedCommandData {
  data: CommandLogListData;
  timestamp: number;
}

// Global cache for command logs
// 命令日志全局缓存
const commandLogCache = new Map<string, CachedCommandData>();
const CACHE_DURATION = 30 * 1000; // 30 seconds / 30秒

/**
 * Generate cache key from request params
 * 从请求参数生成缓存键
 */
function getCommandCacheKey(params: ListCommandLogsRequest): string {
  return `cmd_${JSON.stringify(params)}`;
}

/**
 * Check if cache is valid
 * 检查缓存是否有效
 */
function isCacheValid(timestamp: number): boolean {
  return Date.now() - timestamp < CACHE_DURATION;
}

const DEFAULT_PAGE_SIZE = 10;

/**
 * Command logs management hook
 * 命令日志管理 Hook
 *
 * @param initialPageSize - Initial page size / 初始每页数量
 * @returns Command logs management state and methods / 命令日志管理状态和方法
 */
export function useCommandLogs(
  initialPageSize: number = DEFAULT_PAGE_SIZE,
): UseCommandLogsReturn {
  const [commands, setCommands] = useState<CommandLogInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize] = useState(initialPageSize);
  const [filter, setFilter] = useState<CommandLogFilter>({});

  const totalPages = useMemo(
    () => Math.ceil(total / pageSize),
    [total, pageSize],
  );

  const buildRequestParams = useCallback((): ListCommandLogsRequest => {
    return {
      current: currentPage,
      size: pageSize,
      command_id: filter.commandId,
      agent_id: filter.agentId,
      host_id: filter.hostId,
      command_type: filter.commandType,
      status: filter.status,
      start_time: filter.startTime,
      end_time: filter.endTime,
    };
  }, [currentPage, pageSize, filter]);

  const fetchCommands = useCallback(
    async (forceRefresh = false) => {
      const params = buildRequestParams();
      const cacheKey = getCommandCacheKey(params);

      if (!forceRefresh) {
        const cached = commandLogCache.get(cacheKey);
        if (cached && isCacheValid(cached.timestamp)) {
          setCommands(cached.data.commands);
          setTotal(cached.data.total);
          setIsLoading(false);
          setError(null);
          return;
        }
      }

      setIsLoading(true);
      setError(null);

      try {
        const result = await services.audit.getCommandLogsSafe(params);

        if (result.success && result.data) {
          setCommands(result.data.commands);
          setTotal(result.data.total);
          commandLogCache.set(cacheKey, {
            data: result.data,
            timestamp: Date.now(),
          });
        } else {
          setError(result.error || '获取命令日志列表失败');
          setCommands([]);
          setTotal(0);
        }
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : '获取命令日志列表失败';
        setError(errorMessage);
        setCommands([]);
        setTotal(0);
      } finally {
        setIsLoading(false);
      }
    },
    [buildRequestParams],
  );

  const refresh = useCallback(async () => {
    commandLogCache.clear();
    await fetchCommands(true);
  }, [fetchCommands]);

  const clearFilter = useCallback(() => {
    setFilter({});
    setCurrentPage(1);
  }, []);

  const handleSetFilter = useCallback((newFilter: CommandLogFilter) => {
    setFilter(newFilter);
    setCurrentPage(1);
  }, []);

  useEffect(() => {
    fetchCommands();
  }, [fetchCommands]);

  return {
    commands,
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


// ==================== Audit Log Hook 审计日志 Hook ====================

/**
 * Audit log filter state
 * 审计日志过滤状态
 */
export interface AuditLogFilter {
  userId?: number;
  username?: string;
  action?: string;
  resourceType?: string;
  resourceId?: string;
  startTime?: string;
  endTime?: string;
}

/**
 * Audit log hook return type
 * 审计日志 Hook 返回类型
 */
interface UseAuditLogsReturn {
  /** Audit log list / 审计日志列表 */
  logs: AuditLogInfo[];
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
  filter: AuditLogFilter;
  /** Set current page / 设置当前页码 */
  setCurrentPage: (page: number) => void;
  /** Set filter / 设置过滤条件 */
  setFilter: (filter: AuditLogFilter) => void;
  /** Refresh data / 刷新数据 */
  refresh: () => Promise<void>;
  /** Clear filter / 清除过滤条件 */
  clearFilter: () => void;
}

/**
 * Cache structure for audit logs
 * 审计日志缓存结构
 */
interface CachedAuditData {
  data: AuditLogListData;
  timestamp: number;
}

// Global cache for audit logs
// 审计日志全局缓存
const auditLogCache = new Map<string, CachedAuditData>();

/**
 * Generate cache key from request params
 * 从请求参数生成缓存键
 */
function getAuditCacheKey(params: ListAuditLogsRequest): string {
  return `audit_${JSON.stringify(params)}`;
}

/**
 * Audit logs management hook
 * 审计日志管理 Hook
 *
 * @param initialPageSize - Initial page size / 初始每页数量
 * @returns Audit logs management state and methods / 审计日志管理状态和方法
 */
export function useAuditLogs(
  initialPageSize: number = DEFAULT_PAGE_SIZE,
): UseAuditLogsReturn {
  const [logs, setLogs] = useState<AuditLogInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize] = useState(initialPageSize);
  const [filter, setFilter] = useState<AuditLogFilter>({});

  const totalPages = useMemo(
    () => Math.ceil(total / pageSize),
    [total, pageSize],
  );

  const buildRequestParams = useCallback((): ListAuditLogsRequest => {
    return {
      current: currentPage,
      size: pageSize,
      user_id: filter.userId,
      username: filter.username,
      action: filter.action,
      resource_type: filter.resourceType,
      resource_id: filter.resourceId,
      start_time: filter.startTime,
      end_time: filter.endTime,
    };
  }, [currentPage, pageSize, filter]);

  const fetchLogs = useCallback(
    async (forceRefresh = false) => {
      const params = buildRequestParams();
      const cacheKey = getAuditCacheKey(params);

      if (!forceRefresh) {
        const cached = auditLogCache.get(cacheKey);
        if (cached && isCacheValid(cached.timestamp)) {
          setLogs(cached.data.logs);
          setTotal(cached.data.total);
          setIsLoading(false);
          setError(null);
          return;
        }
      }

      setIsLoading(true);
      setError(null);

      try {
        const result = await services.audit.getAuditLogsSafe(params);

        if (result.success && result.data) {
          setLogs(result.data.logs);
          setTotal(result.data.total);
          auditLogCache.set(cacheKey, {
            data: result.data,
            timestamp: Date.now(),
          });
        } else {
          setError(result.error || '获取审计日志列表失败');
          setLogs([]);
          setTotal(0);
        }
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : '获取审计日志列表失败';
        setError(errorMessage);
        setLogs([]);
        setTotal(0);
      } finally {
        setIsLoading(false);
      }
    },
    [buildRequestParams],
  );

  const refresh = useCallback(async () => {
    auditLogCache.clear();
    await fetchLogs(true);
  }, [fetchLogs]);

  const clearFilter = useCallback(() => {
    setFilter({});
    setCurrentPage(1);
  }, []);

  const handleSetFilter = useCallback((newFilter: AuditLogFilter) => {
    setFilter(newFilter);
    setCurrentPage(1);
  }, []);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  return {
    logs,
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

// ==================== Single Command Log Hook 单个命令日志 Hook ====================

/**
 * Single command log hook return type
 * 单个命令日志 Hook 返回类型
 */
interface UseCommandLogDetailReturn {
  /** Command log data / 命令日志数据 */
  command: CommandLogInfo | null;
  /** Loading state / 加载状态 */
  isLoading: boolean;
  /** Error state / 错误状态 */
  error: string | null;
  /** Refresh data / 刷新数据 */
  refresh: () => Promise<void>;
}

/**
 * Single command log detail hook
 * 单个命令日志详情 Hook
 *
 * @param logId - Command log ID / 命令日志 ID
 * @returns Command log detail state and methods / 命令日志详情状态和方法
 */
export function useCommandLogDetail(
  logId: number | null,
): UseCommandLogDetailReturn {
  const [command, setCommand] = useState<CommandLogInfo | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchCommand = useCallback(async () => {
    if (!logId) {
      setCommand(null);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const result = await services.audit.getCommandLogSafe(logId);

      if (result.success && result.data) {
        setCommand(result.data);
      } else {
        setError(result.error || '获取命令日志详情失败');
        setCommand(null);
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : '获取命令日志详情失败';
      setError(errorMessage);
      setCommand(null);
    } finally {
      setIsLoading(false);
    }
  }, [logId]);

  const refresh = useCallback(async () => {
    await fetchCommand();
  }, [fetchCommand]);

  useEffect(() => {
    fetchCommand();
  }, [fetchCommand]);

  return {
    command,
    isLoading,
    error,
    refresh,
  };
}
