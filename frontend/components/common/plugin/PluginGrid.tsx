/**
 * Plugin Grid Component
 * 插件网格组件
 */

'use client';

import { useState, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { Skeleton } from '@/components/ui/skeleton';
import { Pagination } from '@/components/ui/pagination';
import type { Plugin } from '@/lib/services/plugin';
import { PluginCard } from './PluginCard';

interface PluginGridProps {
  plugins: Plugin[];
  loading: boolean;
  onViewDetail: (plugin: Plugin) => void;
  showInstallButton?: boolean;
  isTransformBuiltIn?: boolean;
  onInstall?: (plugin: Plugin) => void;
}

// Default page size / 默认每页数量
const DEFAULT_PAGE_SIZE = 12;

/**
 * Plugin Grid Component
 * 插件网格组件 - 以卡片网格形式展示插件列表，支持分页
 */
export function PluginGrid({ 
  plugins, 
  loading, 
  onViewDetail,
  showInstallButton = false,
  isTransformBuiltIn = false,
  onInstall,
}: PluginGridProps) {
  const t = useTranslations();

  // Pagination state / 分页状态
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);

  // Calculate pagination / 计算分页
  const totalItems = plugins.length;
  const totalPages = Math.ceil(totalItems / pageSize);

  // Get current page plugins / 获取当前页插件
  const currentPlugins = useMemo(() => {
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    return plugins.slice(startIndex, endIndex);
  }, [plugins, currentPage, pageSize]);

  // Reset to page 1 when plugins change / 当插件列表变化时重置到第一页
  useMemo(() => {
    if (currentPage > 1 && currentPage > totalPages) {
      setCurrentPage(1);
    }
  }, [plugins.length, totalPages, currentPage]);

  // Handle page change / 处理页码变化
  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    // Scroll to top of grid / 滚动到网格顶部
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  // Handle page size change / 处理每页数量变化
  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
    setCurrentPage(1); // Reset to first page / 重置到第一页
  };

  if (loading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {Array.from({ length: pageSize }).map((_, i) => (
          <div key={i} className="p-4 border rounded-lg space-y-3">
            <Skeleton className="h-10 w-10 rounded" />
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
            <div className="flex gap-2">
              <Skeleton className="h-6 w-16" />
              <Skeleton className="h-6 w-16" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (plugins.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        {t('plugin.noPluginsFound')}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Plugin grid / 插件网格 */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {currentPlugins.map((plugin) => (
          <PluginCard
            key={plugin.name}
            plugin={plugin}
            onClick={() => onViewDetail(plugin)}
            showInstallButton={showInstallButton}
            isBuiltIn={isTransformBuiltIn && plugin.category === 'transform'}
            onInstall={onInstall ? () => onInstall(plugin) : undefined}
          />
        ))}
      </div>

      {/* Pagination / 分页 */}
      {totalPages > 1 && (
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          pageSize={pageSize}
          totalItems={totalItems}
          onPageChange={handlePageChange}
          onPageSizeChange={handlePageSizeChange}
          pageSizeOptions={[12, 24, 48, 96]}
          showPageSizeSelector={true}
          showTotalItems={true}
        />
      )}
    </div>
  );
}
