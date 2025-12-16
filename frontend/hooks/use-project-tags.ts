'use client';

import {useState, useCallback} from 'react';
import services from '@/lib/services';

export function useProjectTags(initialTags: string[] = []) {
  const [tags, setTags] = useState<string[]>(initialTags);
  const [availableTags, setAvailableTags] = useState<string[]>([]);

  const fetchTags = useCallback(async () => {
    try {
      const result = await services.project.getTagsSafe();
      if (result.success) {
        setAvailableTags(result.tags);
      } else {
        setAvailableTags([]);
        console.warn('获取标签列表失败:', result.error);
      }
    } catch (error) {
      console.error('获取标签失败:', error);
      setAvailableTags([]);
    }
  }, []);

  const resetTags = useCallback((newTags: string[] = []) => {
    setTags(newTags);
  }, []);

  return {
    tags,
    setTags,
    availableTags,
    fetchTags,
    resetTags,
  };
}
