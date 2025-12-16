'use client';

import {useState, useCallback} from 'react';
import {toast} from 'sonner';
import {handleBulkImportContentWithFilter} from '@/components/common/project';

export function useBulkImport(initialItems: string[] = []) {
  const [items, setItems] = useState<string[]>(initialItems);
  const [bulkContent, setBulkContent] = useState('');
  const [allowDuplicates, setAllowDuplicates] = useState(false);

  const handleBulkImport = useCallback(() => {
    handleBulkImportContentWithFilter(
        bulkContent,
        items,
        allowDuplicates,
        (newItems: string[], importedCount: number, skippedInfo?: string) => {
          setItems(newItems);
          setBulkContent('');
          const message = `成功导入 ${importedCount} 个内容${skippedInfo || ''}`;
          toast.success(message);
        },
        (errorMessage: string) => {
          toast.error(errorMessage);
        },
    );
  }, [bulkContent, items, allowDuplicates]);

  const removeItem = useCallback((index: number) => {
    setItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const clearItems = useCallback(() => {
    setItems([]);
  }, []);

  const clearBulkContent = useCallback(() => {
    setBulkContent('');
  }, []);

  const resetBulkImport = useCallback((newItems: string[] = []) => {
    setItems(newItems);
    setBulkContent('');
    setAllowDuplicates(false);
  }, []);

  return {
    items,
    setItems,
    bulkContent,
    setBulkContent,
    allowDuplicates,
    setAllowDuplicates,
    handleBulkImport,
    removeItem,
    clearItems,
    clearBulkContent,
    resetBulkImport,
  };
}
