/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
