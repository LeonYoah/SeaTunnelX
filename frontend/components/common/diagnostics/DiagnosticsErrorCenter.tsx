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

import {useCallback, useEffect, useMemo, useState} from 'react';
import {useTranslations} from 'next-intl';
import {RefreshCw, Search} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  DiagnosticsErrorEvent,
  DiagnosticsErrorGroup,
} from '@/lib/services/diagnostics';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Input} from '@/components/ui/input';
import {ScrollArea} from '@/components/ui/scroll-area';
import {Skeleton} from '@/components/ui/skeleton';
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

type DiagnosticsErrorCenterProps = {
  clusterId?: number;
  clusterName?: string;
  groupId?: number;
  onSelectGroup?: (groupId: number | null) => void;
};

function formatDateTime(value?: string | null): string {
  if (!value) {
    return '-';
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

function resolveOccurrenceVariant(
  count: number,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  if (count >= 10) {
    return 'destructive';
  }
  if (count >= 3) {
    return 'secondary';
  }
  return 'outline';
}

export function DiagnosticsErrorCenter({
  clusterId,
  clusterName,
  groupId,
  onSelectGroup,
}: DiagnosticsErrorCenterProps) {
  const t = useTranslations('diagnosticsCenter');
  const commonT = useTranslations('common');

  const [keywordInput, setKeywordInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [loadingGroups, setLoadingGroups] = useState(true);
  const [groups, setGroups] = useState<DiagnosticsErrorGroup[]>([]);
  const [groupTotal, setGroupTotal] = useState(0);
  const [selectedGroupId, setSelectedGroupId] = useState<number | null>(groupId ?? null);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<DiagnosticsErrorGroup | null>(
    null,
  );
  const [groupEvents, setGroupEvents] = useState<DiagnosticsErrorEvent[]>([]);
  const [selectedEventId, setSelectedEventId] = useState<number | null>(null);

  const loadGroups = useCallback(async () => {
    setLoadingGroups(true);
    try {
      const result = await services.diagnostics.getErrorGroupsSafe({
        cluster_id: clusterId,
        keyword: keyword || undefined,
        page,
        page_size: 20,
      });
      if (!result.success || !result.data) {
        toast.error(result.error || t('errors.loadGroupsError'));
        setGroups([]);
        setGroupTotal(0);
        return;
      }
      setGroups(result.data.items || []);
      setGroupTotal(result.data.total || 0);
    } finally {
      setLoadingGroups(false);
    }
  }, [clusterId, keyword, page, t]);

  const loadGroupDetail = useCallback(async (groupId: number) => {
    setLoadingDetail(true);
    try {
      const result = await services.diagnostics.getErrorGroupDetailSafe(
        groupId,
        20,
      );
      if (!result.success || !result.data) {
        toast.error(result.error || t('errors.loadDetailError'));
        setSelectedGroup(null);
        setGroupEvents([]);
        setSelectedEventId(null);
        return;
      }
      setSelectedGroup(result.data.group);
      setGroupEvents(result.data.events || []);
      setSelectedEventId(result.data.events?.[0]?.id ?? null);
    } finally {
      setLoadingDetail(false);
    }
  }, [t]);

  useEffect(() => {
    void loadGroups();
  }, [loadGroups]);

  useEffect(() => {
    if (groups.length === 0) {
      setSelectedGroupId(null);
      setSelectedGroup(null);
      setGroupEvents([]);
      setSelectedEventId(null);
      return;
    }
    if (!selectedGroupId || !groups.some((item) => item.id === selectedGroupId)) {
      setSelectedGroupId(groups[0].id);
      onSelectGroup?.(groups[0].id);
    }
  }, [groups, onSelectGroup, selectedGroupId]);

  useEffect(() => {
    if (!selectedGroupId || !groups.some((item) => item.id === selectedGroupId)) {
      return;
    }
    void loadGroupDetail(selectedGroupId);
  }, [groups, loadGroupDetail, selectedGroupId]);

  useEffect(() => {
    setSelectedGroupId(groupId ?? null);
  }, [groupId]);

  const selectedEvent = useMemo(
    () => groupEvents.find((item) => item.id === selectedEventId) ?? groupEvents[0],
    [groupEvents, selectedEventId],
  );

  const totalPages = Math.max(1, Math.ceil(groupTotal / 20));

  return (
    <div className='grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]'>
      <div className='space-y-4'>
        <Card>
          <CardHeader className='space-y-3'>
            <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
              <div>
                <CardTitle>{t('errors.title')}</CardTitle>
                <div className='mt-1 text-sm text-muted-foreground'>
                  {clusterId
                    ? t('errors.clusterScopedHint', {
                        name: clusterName || `#${clusterId}`,
                      })
                    : t('errors.globalHint')}
                </div>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Badge variant='outline'>
                  {t('errors.matchedGroups', {count: groupTotal})}
                </Badge>
                <Button variant='outline' onClick={() => void loadGroups()}>
                  <RefreshCw className='mr-2 h-4 w-4' />
                  {commonT('refresh')}
                </Button>
              </div>
            </div>
            <form
              className='flex flex-col gap-2 sm:flex-row'
              onSubmit={(event) => {
                event.preventDefault();
                setPage(1);
                setKeyword(keywordInput.trim());
              }}
            >
              <Input
                value={keywordInput}
                onChange={(event) => setKeywordInput(event.target.value)}
                placeholder={t('errors.keywordPlaceholder')}
                className='flex-1 min-w-[220px] max-w-xl'
              />
              <Button type='submit'>
                <Search className='mr-2 h-4 w-4' />
                {t('errors.search')}
              </Button>
            </form>
          </CardHeader>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('errors.groupListTitle')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            {loadingGroups ? (
              <div className='space-y-3'>
                <Skeleton className='h-10 w-full' />
                <Skeleton className='h-10 w-full' />
                <Skeleton className='h-10 w-full' />
              </div>
            ) : groups.length === 0 ? (
              <div className='rounded-lg border border-dashed p-6 text-sm text-muted-foreground'>
                {t('errors.empty')}
              </div>
            ) : (
              <>
                <Table className='table-fixed'>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='w-[52%]'>
                        {t('errors.columns.group')}
                      </TableHead>
                      <TableHead className='w-[24%]'>
                        {t('errors.columns.exception')}
                      </TableHead>
                      <TableHead className='w-[96px] whitespace-nowrap'>
                        {t('errors.columns.occurrences')}
                      </TableHead>
                      <TableHead className='w-[180px] whitespace-nowrap'>
                        {t('errors.columns.lastSeen')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {groups.map((group) => (
                      <TableRow
                        key={group.id}
                        className={
                          selectedGroupId === group.id
                            ? 'cursor-pointer bg-muted/40'
                            : 'cursor-pointer'
                        }
                        onClick={() => {
                          setSelectedGroupId(group.id);
                          onSelectGroup?.(group.id);
                        }}
                      >
                        <TableCell className='w-[52%] max-w-0'>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <div className='overflow-hidden'>
                                <div
                                  className='truncate font-medium'
                                  title={group.title || '-'}
                                >
                                  {group.title || '-'}
                                </div>
                                <div
                                  className='mt-1 truncate text-xs text-muted-foreground'
                                  title={
                                    group.sample_message || group.fingerprint
                                  }
                                >
                                  {group.sample_message || group.fingerprint}
                                </div>
                              </div>
                            </TooltipTrigger>
                            <TooltipContent
                              side='top'
                              align='start'
                              className='max-w-[760px] break-all'
                            >
                              <div className='space-y-1 text-xs'>
                                <div>{group.title || '-'}</div>
                                <div className='text-muted-foreground'>
                                  {group.sample_message || group.fingerprint}
                                </div>
                              </div>
                            </TooltipContent>
                          </Tooltip>
                        </TableCell>
                        <TableCell className='w-[24%] max-w-0 text-sm text-muted-foreground'>
                          <div
                            className='truncate'
                            title={group.exception_class || '-'}
                          >
                            {group.exception_class || '-'}
                          </div>
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          <Badge
                            variant={resolveOccurrenceVariant(
                              group.occurrence_count,
                            )}
                          >
                            {group.occurrence_count}
                          </Badge>
                        </TableCell>
                        <TableCell className='whitespace-nowrap text-sm text-muted-foreground'>
                          {formatDateTime(group.last_seen_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                <div className='flex items-center justify-between text-sm'>
                  <div className='text-muted-foreground'>
                    {t('errors.pageSummary', {
                      page,
                      totalPages,
                    })}
                  </div>
                  <div className='flex items-center gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => setPage((current) => Math.max(1, current - 1))}
                      disabled={page <= 1}
                    >
                      {t('errors.previous')}
                    </Button>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        setPage((current) =>
                          current >= totalPages ? current : current + 1,
                        )
                      }
                      disabled={page >= totalPages}
                    >
                      {t('errors.next')}
                    </Button>
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      <Card className='min-h-[820px]'>
        <CardHeader>
          <CardTitle>{t('errors.detailTitle')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          {loadingDetail ? (
            <div className='space-y-3'>
              <Skeleton className='h-12 w-full' />
              <Skeleton className='h-32 w-full' />
              <Skeleton className='h-56 w-full' />
            </div>
          ) : !selectedGroup ? (
            <div className='rounded-lg border border-dashed p-6 text-sm text-muted-foreground'>
              {t('errors.selectGroup')}
            </div>
          ) : (
            <>
              <div className='space-y-3 rounded-lg border p-4'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge variant={resolveOccurrenceVariant(selectedGroup.occurrence_count)}>
                    {t('errors.columns.occurrences')}: {selectedGroup.occurrence_count}
                  </Badge>
                  <Badge variant='outline'>
                    {t('errors.columns.lastSeen')}: {formatDateTime(selectedGroup.last_seen_at)}
                  </Badge>
                </div>
                  <div>
                    <div className='break-all text-sm font-medium'>
                      {selectedGroup.title || '-'}
                    </div>
                    <div className='mt-1 text-sm text-muted-foreground'>
                      {selectedGroup.exception_class || selectedGroup.fingerprint}
                    </div>
                </div>
                <div className='rounded-md bg-muted/40 p-3 text-sm text-muted-foreground'>
                  {selectedGroup.sample_message || t('errors.noSampleMessage')}
                </div>
              </div>

              <div className='space-y-3'>
                <div className='text-sm font-medium'>
                  {t('errors.recentEventsTitle')}
                </div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('errors.columns.time')}</TableHead>
                      <TableHead>{t('errors.columns.job')}</TableHead>
                      <TableHead>{t('errors.columns.source')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {groupEvents.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={3}
                          className='text-center text-sm text-muted-foreground'
                        >
                          {t('errors.noEvents')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      groupEvents.map((event) => (
                        <TableRow
                          key={event.id}
                          className={
                            selectedEventId === event.id
                              ? 'cursor-pointer bg-muted/40'
                              : 'cursor-pointer'
                          }
                          onClick={() => setSelectedEventId(event.id)}
                        >
                          <TableCell className='text-sm text-muted-foreground'>
                            {formatDateTime(event.occurred_at)}
                          </TableCell>
                          <TableCell>{event.job_id || '-'}</TableCell>
                          <TableCell className='max-w-[220px] text-sm text-muted-foreground'>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <div className='truncate'>{event.source_file}</div>
                              </TooltipTrigger>
                              <TooltipContent
                                side='top'
                                align='start'
                                className='max-w-[720px] break-all'
                              >
                                {event.source_file}
                              </TooltipContent>
                            </Tooltip>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>

              <div className='space-y-3'>
                <div className='text-sm font-medium'>{t('errors.evidenceTitle')}</div>
                <ScrollArea className='h-[400px] rounded-lg border bg-muted/20'>
                  <pre className='whitespace-pre-wrap break-words p-4 text-xs leading-6 text-muted-foreground'>
                    {selectedEvent?.evidence || t('errors.noEvidence')}
                  </pre>
                </ScrollArea>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
