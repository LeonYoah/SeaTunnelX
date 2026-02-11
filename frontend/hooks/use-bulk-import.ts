'use client';

import {useState, useCallback} from 'react';
import {toast} from 'sonner';

export function useBulkImport(initialItems: string[] = []) {
  const [items, setItems] = useState<string[]>(initialItems);
  const [bulkContent, setBulkContent] = useState('');
  const [allowDuplicates, setAllowDuplicates] = useState(false);

  const handleBulkImport = useCallback(() => {
    const lines = bulkContent
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    if (lines.length === 0) {
      toast.error('没有可导入的内容');
      return;
    }

    let importedCount = 0;
    let skippedDuplicates = 0;
    const newItems = [...items];

    for (const line of lines) {
      if (!allowDuplicates && newItems.includes(line)) {
        skippedDuplicates++;
        continue;
      }
      newItems.push(line);
      importedCount++;
    }

    if (importedCount === 0) {
      toast.error('没有新内容被导入');
      return;
    }

    setItems(newItems);
    setBulkContent('');

    const skippedInfo =
      skippedDuplicates > 0 ? `（跳过重复 ${skippedDuplicates} 条）` : '';
    const message = `成功导入 ${importedCount} 个内容${skippedInfo}`;
    toast.success(message);
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
