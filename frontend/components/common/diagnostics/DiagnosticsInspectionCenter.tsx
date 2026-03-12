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
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useTranslations} from 'next-intl';
import {ClipboardCheck, Loader2, RefreshCw} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  DiagnosticsInspectionFinding,
  DiagnosticsInspectionFindingSeverity,
  DiagnosticsInspectionReport,
  DiagnosticsInspectionReportStatus,
} from '@/lib/services/diagnostics';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {ScrollArea} from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Skeleton} from '@/components/ui/skeleton';
import {localizeDiagnosticsText} from './text-utils';

type DiagnosticsInspectionCenterProps = {
  clusterId?: number;
  clusterName?: string;
  reportId?: number;
  onSelectReport?: (reportId: number | null) => void;
};

const DEFAULT_DIAGNOSIS_TASK_OPTIONS = {
  include_thread_dump: true,
  include_jvm_dump: false,
  jvm_dump_min_free_mb: 2048,
  log_sample_lines: 200,
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

function getStatusVariant(
  status: DiagnosticsInspectionReportStatus,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (status) {
    case 'completed':
      return 'default';
    case 'failed':
      return 'destructive';
    case 'running':
      return 'secondary';
    case 'pending':
    default:
      return 'outline';
  }
}

function getSeverityVariant(
  severity: DiagnosticsInspectionFindingSeverity,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (severity) {
    case 'critical':
      return 'destructive';
    case 'warning':
      return 'secondary';
    case 'info':
    default:
      return 'outline';
  }
}

function getFindingSeverityScore(
  severity: DiagnosticsInspectionFindingSeverity,
): number {
  switch (severity) {
    case 'critical':
      return 3;
    case 'warning':
      return 2;
    case 'info':
    default:
      return 1;
  }
}

export function DiagnosticsInspectionCenter({
  clusterId,
  clusterName,
  reportId,
  onSelectReport,
}: DiagnosticsInspectionCenterProps) {
  const t = useTranslations('diagnosticsCenter');
  const commonT = useTranslations('common');
  const router = useRouter();

  const [statusFilter, setStatusFilter] = useState<
    'all' | DiagnosticsInspectionReportStatus
  >('all');
  const [severityFilter, setSeverityFilter] = useState<
    'all' | DiagnosticsInspectionFindingSeverity
  >('all');
  const [page, setPage] = useState(1);
  const [loadingReports, setLoadingReports] = useState(true);
  const [startingInspection, setStartingInspection] = useState(false);
  const [lookbackMinutes, setLookbackMinutes] = useState(30);
  const [reports, setReports] = useState<DiagnosticsInspectionReport[]>([]);
  const [reportTotal, setReportTotal] = useState(0);
  const [selectedReportId, setSelectedReportId] = useState<number | null>(
    reportId ?? null,
  );
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [selectedReport, setSelectedReport] =
    useState<DiagnosticsInspectionReport | null>(null);
  const [relatedTask, setRelatedTask] =
    useState<import('@/lib/services/diagnostics').DiagnosticsTask | null>(null);
  const [findings, setFindings] = useState<DiagnosticsInspectionFinding[]>([]);
  const [selectedFindingId, setSelectedFindingId] = useState<number | null>(null);
  const [creatingDiagnosisTask, setCreatingDiagnosisTask] = useState(false);
  const [dismissedFollowUpReportId, setDismissedFollowUpReportId] = useState<
    number | null
  >(null);

  const loadReports = useCallback(async () => {
    setLoadingReports(true);
    try {
      const result = await services.diagnostics.getInspectionReportsSafe({
        cluster_id: clusterId,
        status: statusFilter !== 'all' ? statusFilter : undefined,
        severity: severityFilter !== 'all' ? severityFilter : undefined,
        page,
        page_size: 20,
      });
      if (!result.success || !result.data) {
        toast.error(result.error || t('inspections.loadReportsError'));
        setReports([]);
        setReportTotal(0);
        return;
      }
      setReports(result.data.items || []);
      setReportTotal(result.data.total || 0);
    } finally {
      setLoadingReports(false);
    }
  }, [clusterId, page, severityFilter, statusFilter, t]);

  const loadDetail = useCallback(
    async (nextReportId: number) => {
      setLoadingDetail(true);
      try {
        const result =
          await services.diagnostics.getInspectionReportDetailSafe(
            nextReportId,
          );
        if (!result.success || !result.data) {
          toast.error(result.error || t('inspections.loadDetailError'));
          setSelectedReport(null);
          setFindings([]);
          setRelatedTask(null);
          return;
        }
        setSelectedReport(result.data.report);
        setFindings(result.data.findings || []);
        setRelatedTask(result.data.related_diagnostic_task || null);
      } finally {
        setLoadingDetail(false);
      }
    },
    [t],
  );

  useEffect(() => {
    void loadReports();
  }, [loadReports]);

  useEffect(() => {
    if (reports.length === 0) {
      setSelectedReportId(null);
      setSelectedReport(null);
      setFindings([]);
      setRelatedTask(null);
      return;
    }
    if (
      !selectedReportId ||
      !reports.some((item) => item.id === selectedReportId)
    ) {
      setSelectedReportId(reports[0].id);
      onSelectReport?.(reports[0].id);
    }
  }, [onSelectReport, reports, selectedReportId]);

  useEffect(() => {
    if (
      !selectedReportId ||
      !reports.some((item) => item.id === selectedReportId)
    ) {
      return;
    }
    void loadDetail(selectedReportId);
  }, [loadDetail, reports, selectedReportId]);

  useEffect(() => {
    setSelectedReportId(reportId ?? null);
  }, [reportId]);

  useEffect(() => {
    if (findings.length === 0) {
      setSelectedFindingId(null);
      return;
    }
    if (selectedFindingId && findings.some((item) => item.id === selectedFindingId)) {
      return;
    }
    const recommendedFinding = [...findings].sort((left, right) => {
      const severityDelta =
        getFindingSeverityScore(right.severity) -
        getFindingSeverityScore(left.severity);
      if (severityDelta !== 0) {
        return severityDelta;
      }
      return left.id - right.id;
    })[0];
    setSelectedFindingId(recommendedFinding?.id ?? null);
  }, [findings, selectedFindingId]);

  const groupedFindings = useMemo(
    () =>
      findings.reduce<
        Record<
          DiagnosticsInspectionFindingSeverity,
          DiagnosticsInspectionFinding[]
        >
      >(
        (accumulator, finding) => {
          const bucket = accumulator[finding.severity] || [];
          bucket.push(finding);
          accumulator[finding.severity] = bucket;
          return accumulator;
        },
        {
          critical: [],
          warning: [],
          info: [],
        },
      ),
    [findings],
  );
  const selectedFinding = useMemo(
    () => findings.find((item) => item.id === selectedFindingId) ?? null,
    [findings, selectedFindingId],
  );
  const hasFindings = findings.length > 0;
  const showFollowUpPrompt =
    !!selectedReport &&
    selectedReport.status === 'completed' &&
    hasFindings &&
    dismissedFollowUpReportId !== selectedReport.id;

  const totalPages = Math.max(1, Math.ceil(reportTotal / 20));

  const handleStartInspection = useCallback(async () => {
    if (!clusterId) {
      return;
    }
    if (lookbackMinutes < 5 || lookbackMinutes > 1440) {
      toast.error(t('inspections.lookbackRangeError'));
      return;
    }
    setStartingInspection(true);
    try {
      const result = await services.diagnostics.startInspectionSafe({
        cluster_id: clusterId,
        trigger_source: 'diagnostics_workspace',
        lookback_minutes: lookbackMinutes,
      });
      if (!result.success || !result.data?.report) {
        toast.error(result.error || t('inspections.startError'));
        return;
      }
      toast.success(t('inspections.startSuccess'));
      setPage(1);
      await loadReports();
      setSelectedReportId(result.data.report.id);
      onSelectReport?.(result.data.report.id);
      setSelectedReport(result.data.report);
      setFindings(result.data.findings || []);
    } finally {
      setStartingInspection(false);
    }
  }, [clusterId, loadReports, lookbackMinutes, onSelectReport, t]);

  const handleCreateDiagnosisTask = useCallback(async () => {
    if (!selectedReport || !selectedFinding) {
      toast.error(t('inspections.followUp.chooseFindingFirst'));
      return;
    }
    // 已经存在关联诊断任务时，优先引导用户查看现有结果，避免重复创建。
    if (relatedTask) {
      router.push(`/diagnostics/inspections/${selectedReport.id}`);
      return;
    }
    setCreatingDiagnosisTask(true);
    try {
      const result = await services.diagnostics.createTaskSafe({
        cluster_id: selectedReport.cluster_id,
        trigger_source: 'inspection_finding',
        source_ref: {
          inspection_report_id: selectedReport.id,
          inspection_finding_id: selectedFinding.id,
        },
        options: DEFAULT_DIAGNOSIS_TASK_OPTIONS,
        auto_start: true,
      });
      if (!result.success || !result.data) {
        toast.error(result.error || t('inspections.followUp.createTaskError'));
        return;
      }
      toast.success(t('inspections.followUp.createTaskSuccess'));
      router.push(`/diagnostics/inspections/${selectedReport.id}`);
    } finally {
      setCreatingDiagnosisTask(false);
    }
  }, [relatedTask, router, selectedFinding, selectedReport, t]);

  return (
    <div className='grid gap-4 xl:grid-cols-[minmax(0,1.08fr)_minmax(360px,0.92fr)] xl:items-start'>
      <div className='space-y-4'>
        <Card>
          <CardHeader className='space-y-3'>
            <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
              <div>
                <CardTitle>{t('inspections.title')}</CardTitle>
                <div className='mt-1 text-sm text-muted-foreground'>
                  {clusterId
                    ? t('inspections.clusterScopedHint', {
                        name: clusterName || `#${clusterId}`,
                      })
                    : t('inspections.globalHint')}
                </div>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Badge variant='outline'>
                  {t('inspections.matchedReports', {count: reportTotal})}
                </Badge>
                <Button variant='outline' onClick={() => void loadReports()}>
                  <RefreshCw className='mr-2 h-4 w-4' />
                  {commonT('refresh')}
                </Button>
                <Button
                  onClick={() => void handleStartInspection()}
                  disabled={!clusterId || startingInspection}
                >
                  {startingInspection ? (
                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  ) : (
                    <ClipboardCheck className='mr-2 h-4 w-4' />
                  )}
                  {t('inspections.startInspection')}
                </Button>
              </div>
            </div>
            <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
              <div className='space-y-2'>
                <Label>{t('inspections.filters.status')}</Label>
                <Select
                  value={statusFilter}
                  onValueChange={(value) => {
                    setPage(1);
                    setStatusFilter(value as typeof statusFilter);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={t('inspections.filters.status')}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>
                      {t('inspections.filters.allStatuses')}
                    </SelectItem>
                    <SelectItem value='completed'>
                      {t('inspections.status.completed')}
                    </SelectItem>
                    <SelectItem value='failed'>
                      {t('inspections.status.failed')}
                    </SelectItem>
                    <SelectItem value='running'>
                      {t('inspections.status.running')}
                    </SelectItem>
                    <SelectItem value='pending'>
                      {t('inspections.status.pending')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2'>
                <Label>{t('inspections.filters.severity')}</Label>
                <Select
                  value={severityFilter}
                  onValueChange={(value) => {
                    setPage(1);
                    setSeverityFilter(value as typeof severityFilter);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={t('inspections.filters.severity')}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>
                      {t('inspections.filters.allSeverities')}
                    </SelectItem>
                    <SelectItem value='critical'>
                      {t('inspections.severity.critical')}
                    </SelectItem>
                    <SelectItem value='warning'>
                      {t('inspections.severity.warning')}
                    </SelectItem>
                    <SelectItem value='info'>
                      {t('inspections.severity.info')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2'>
                <Label htmlFor='diagnostics-inspection-lookback'>
                  {t('inspections.lookbackLabel')}
                </Label>
                <Input
                  id='diagnostics-inspection-lookback'
                  type='number'
                  min={5}
                  max={1440}
                  step={5}
                  value={lookbackMinutes}
                  onChange={(event) =>
                    setLookbackMinutes(
                      Number.parseInt(event.target.value, 10) || 30,
                    )
                  }
                />
                <div className='text-xs text-muted-foreground'>
                  {t('inspections.lookbackHint')}
                </div>
              </div>
            </div>
          </CardHeader>
        </Card>

        <Card className='flex min-h-[630px] flex-col overflow-hidden xl:h-[780px]'>
          <CardHeader>
            <CardTitle>{t('inspections.listTitle')}</CardTitle>
          </CardHeader>
          <CardContent className='flex flex-1 min-h-0 flex-col space-y-4'>
            {loadingReports ? (
              <div className='space-y-3'>
                <Skeleton className='h-24 w-full' />
                <Skeleton className='h-24 w-full' />
                <Skeleton className='h-24 w-full' />
              </div>
            ) : reports.length === 0 ? (
              <div className='flex flex-1 items-center justify-center rounded-lg border border-dashed p-6 text-sm text-muted-foreground'>
                {t('inspections.empty')}
              </div>
            ) : (
              <>
                <ScrollArea className='min-h-0 flex-1 pr-3'>
                  <div className='space-y-3'>
                    {reports.map((report) => (
                      <button
                        key={report.id}
                        type='button'
                        className={
                          selectedReportId === report.id
                            ? 'flex min-h-[140px] w-full flex-col gap-3 rounded-lg border border-primary bg-muted/40 p-4 text-left'
                            : 'flex min-h-[140px] w-full flex-col gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-muted/30'
                        }
                        onClick={() => {
                          setSelectedReportId(report.id);
                          onSelectReport?.(report.id);
                        }}
                        onDoubleClick={() => {
                          router.push(`/diagnostics/inspections/${report.id}`);
                        }}
                      >
                        <div className='flex flex-wrap items-center gap-2'>
                          <Badge variant={getStatusVariant(report.status)}>
                            {t(`inspections.status.${report.status}`)}
                          </Badge>
                          <Badge variant='outline'>#{report.id}</Badge>
                          <Badge variant='outline'>
                            {t(`inspections.trigger.${report.trigger_source}`)}
                          </Badge>
                        </div>
                        <div className='min-w-0 flex-1'>
                          <div
                            className='line-clamp-2 font-medium leading-6'
                            title={
                              localizeDiagnosticsText(report.summary) ||
                              t('inspections.summaryFallback')
                            }
                          >
                            {localizeDiagnosticsText(report.summary) ||
                              t('inspections.summaryFallback')}
                          </div>
                          <div className='mt-1 text-sm text-muted-foreground'>
                            {t('inspections.counts', {
                              total: report.finding_total,
                              critical: report.critical_count,
                              warning: report.warning_count,
                              info: report.info_count,
                            })}
                          </div>
                          <div className='mt-1 text-xs text-muted-foreground'>
                            {t('inspections.lookbackValue', {
                              minutes: report.lookback_minutes || 30,
                            })}
                          </div>
                        </div>
                        <div className='mt-auto flex items-center justify-between'>
                          <div className='text-xs text-muted-foreground'>
                            {t('inspections.finishedAt')}:{' '}
                            {formatDateTime(
                              report.finished_at || report.created_at,
                            )}
                          </div>
                          <Button
                            asChild
                            size='sm'
                            variant='ghost'
                            className='h-auto px-2 py-1 text-xs'
                            onClick={(e: React.MouseEvent) => e.stopPropagation()}
                          >
                            <Link href={`/diagnostics/inspections/${report.id}`}>
                              查看详情
                            </Link>
                          </Button>
                        </div>
                      </button>
                    ))}
                  </div>
                </ScrollArea>

                <div className='flex items-center justify-between text-sm'>
                  <div className='text-muted-foreground'>
                    {t('errors.pageSummary', {page, totalPages})}
                  </div>
                  <div className='flex items-center gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        setPage((current) => Math.max(1, current - 1))
                      }
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

      <Card className='flex min-h-[1020px] flex-col overflow-hidden xl:h-[1110px]'>
        <CardHeader>
          <CardTitle>{t('inspections.detailTitle')}</CardTitle>
        </CardHeader>
        <CardContent className='flex flex-1 min-h-0 flex-col space-y-4'>
          {loadingDetail ? (
            <div className='space-y-3'>
              <Skeleton className='h-20 w-full' />
              <Skeleton className='h-48 w-full' />
              <Skeleton className='h-56 w-full' />
            </div>
          ) : !selectedReport ? (
            <div className='flex flex-1 items-center justify-center rounded-lg border border-dashed p-6 text-sm text-muted-foreground'>
              {t('inspections.selectReport')}
            </div>
          ) : (
            <>
              <div className='space-y-3 rounded-lg border p-4'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge variant={getStatusVariant(selectedReport.status)}>
                    {t(`inspections.status.${selectedReport.status}`)}
                  </Badge>
                  <Badge variant='outline'>
                    {t(`inspections.trigger.${selectedReport.trigger_source}`)}
                  </Badge>
                </div>
                <div className='text-sm'>
                  {localizeDiagnosticsText(selectedReport.summary) ||
                    t('inspections.summaryFallback')}
                </div>
                {selectedReport.error_message ? (
                  <div className='rounded-md border border-destructive/20 bg-destructive/5 p-3 text-sm text-destructive'>
                    {selectedReport.error_message}
                  </div>
                ) : null}
                <div className='grid gap-2 text-sm sm:grid-cols-2'>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('inspections.requestedBy')}:
                    </span>{' '}
                    {selectedReport.requested_by || '-'}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('inspections.startedAt')}:
                    </span>{' '}
                    {formatDateTime(
                      selectedReport.started_at || selectedReport.created_at,
                    )}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('inspections.finishedAt')}:
                    </span>{' '}
                    {formatDateTime(selectedReport.finished_at)}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('inspections.lookbackLabel')}:
                    </span>{' '}
                    {t('inspections.lookbackValue', {
                      minutes: selectedReport.lookback_minutes || 30,
                    })}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('inspections.countSummary')}:
                    </span>{' '}
                    {t('inspections.counts', {
                      total: selectedReport.finding_total,
                      critical: selectedReport.critical_count,
                      warning: selectedReport.warning_count,
                      info: selectedReport.info_count,
                    })}
                  </div>
                </div>
              </div>

              <div className='flex min-h-0 flex-1 flex-col space-y-3'>
                {selectedReport.status === 'completed' ? (
                  <div className='rounded-lg border bg-muted/20 p-4'>
                    {showFollowUpPrompt ? (
                      <div className='space-y-3'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <Badge variant='secondary'>
                            {t('inspections.followUp.title')}
                          </Badge>
                          <Badge variant={getSeverityVariant(selectedFinding?.severity || 'info')}>
                            {t(
                              `inspections.severity.${selectedFinding?.severity || 'info'}`,
                            )}
                          </Badge>
                        </div>
                        <div className='text-sm leading-6'>
                          {t('inspections.followUp.hasFindingsDescription')}
                        </div>
                        {selectedFinding ? (
                          <div className='rounded-md border bg-background p-3'>
                            <div className='text-xs text-muted-foreground'>
                              {t('inspections.followUp.currentFinding')}
                            </div>
                            <div className='mt-1 text-sm font-medium leading-6'>
                              {localizeDiagnosticsText(
                                selectedFinding.check_name ||
                                  selectedFinding.summary,
                              )}
                            </div>
                            <div className='mt-1 text-xs text-muted-foreground'>
                              {localizeDiagnosticsText(selectedFinding.summary)}
                            </div>
                          </div>
                        ) : null}
                        <div className='flex flex-wrap gap-2'>
                          <Button
                            onClick={() => void handleCreateDiagnosisTask()}
                            disabled={creatingDiagnosisTask || !selectedFinding}
                          >
                            {creatingDiagnosisTask ? (
                              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                            ) : null}
                            {relatedTask
                              ? t('inspections.followUp.viewExistingTask')
                              : t('inspections.followUp.createTask')}
                          </Button>
                          <Button
                            variant='outline'
                            onClick={() =>
                              setDismissedFollowUpReportId(selectedReport.id)
                            }
                          >
                            {t('inspections.followUp.dismiss')}
                          </Button>
                        </div>
                        <div className='text-xs text-muted-foreground'>
                          {t('inspections.followUp.hasFindingsHint')}
                        </div>
                      </div>
                    ) : hasFindings ? (
                      <div className='space-y-2'>
                        <div className='font-medium'>
                          {t('inspections.followUp.dismissedTitle')}
                        </div>
                        <div className='text-sm text-muted-foreground'>
                          {t('inspections.followUp.dismissedDescription')}
                        </div>
                        <div>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => setDismissedFollowUpReportId(null)}
                          >
                            {t('inspections.followUp.resume')}
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <div className='space-y-2'>
                        <div className='font-medium'>
                          {t('inspections.followUp.noFindingsTitle')}
                        </div>
                        <div className='text-sm text-muted-foreground'>
                          {t('inspections.followUp.noFindingsDescription')}
                        </div>
                      </div>
                    )}
                  </div>
                ) : null}

                <div className='text-sm font-medium'>
                  {t('inspections.findingsTitle')}
                </div>
                {findings.length === 0 ? (
                  <div className='flex flex-1 items-center justify-center rounded-lg border border-dashed p-6 text-sm text-muted-foreground'>
                    {t('inspections.noFindings')}
                  </div>
                ) : (
                  <ScrollArea className='min-h-0 flex-1 pr-3'>
                    <div className='space-y-4'>
                      {(['critical', 'warning', 'info'] as const).map(
                        (severity) => {
                          const severityItems = groupedFindings[severity];
                          if (!severityItems || severityItems.length === 0) {
                            return null;
                          }
                          return (
                            <div key={severity} className='space-y-3'>
                              <div className='flex items-center gap-2'>
                                <Badge variant={getSeverityVariant(severity)}>
                                  {t(`inspections.severity.${severity}`)}
                                </Badge>
                                <span className='text-sm text-muted-foreground'>
                                  {severityItems.length}
                                </span>
                              </div>
                              {severityItems.map((finding) => (
                                <div
                                  key={finding.id}
                                  className='rounded-lg border p-4'
                                >
                                  <div className='flex flex-wrap items-center gap-2'>
                                    <Badge
                                      variant={getSeverityVariant(
                                        finding.severity,
                                      )}
                                    >
                                      {t(
                                        `inspections.severity.${finding.severity}`,
                                      )}
                                    </Badge>
                                    <Badge variant='outline'>
                                      {finding.category}
                                    </Badge>
                                    <Badge variant='outline'>
                                      {finding.check_code}
                                    </Badge>
                                  </div>
                                  <div className='mt-3 space-y-2 text-sm'>
                                    <div className='font-medium'>
                                      {localizeDiagnosticsText(
                                        finding.check_name || finding.summary,
                                      )}
                                    </div>
                                    <div>
                                      {localizeDiagnosticsText(finding.summary)}
                                    </div>
                                    {finding.evidence_summary ? (
                                      <div className='rounded-md bg-muted/40 p-3 text-muted-foreground'>
                                        {localizeDiagnosticsText(
                                          finding.evidence_summary,
                                        )}
                                      </div>
                                    ) : null}
                                    {finding.recommendation ? (
                                      <div className='text-muted-foreground'>
                                        {localizeDiagnosticsText(
                                          finding.recommendation,
                                        )}
                                      </div>
                                    ) : null}
                                    <div className='flex flex-wrap items-center gap-2 pt-1'>
                                      <Button
                                        size='sm'
                                        variant={
                                          selectedFindingId === finding.id
                                            ? 'default'
                                            : 'outline'
                                        }
                                        onClick={() =>
                                          setSelectedFindingId(finding.id)
                                        }
                                      >
                                        {selectedFindingId === finding.id
                                          ? t(
                                              'inspections.actions.selectedForDiagnosis',
                                            )
                                          : t(
                                              'inspections.actions.useForDiagnosis',
                                            )}
                                      </Button>
                                      {finding.related_error_group_id > 0 ? (
                                        <Button
                                          asChild
                                          size='sm'
                                          variant='outline'
                                        >
                                          <Link
                                            href={`/diagnostics?tab=errors&cluster_id=${selectedReport.cluster_id}&group_id=${finding.related_error_group_id}&source=inspection-finding`}
                                          >
                                            {t('inspections.actions.viewError')}
                                          </Link>
                                        </Button>
                                      ) : null}
                                      <Button
                                        asChild
                                        size='sm'
                                        variant='outline'
                                      >
                                        <Link
                                          href={`/diagnostics?tab=tasks&cluster_id=${selectedReport.cluster_id}&report_id=${selectedReport.id}&finding_id=${finding.id}&source=inspection-finding`}
                                        >
                                          {t('inspections.actions.createTask')}
                                        </Link>
                                      </Button>
                                    </div>
                                  </div>
                                </div>
                              ))}
                            </div>
                          );
                        },
                      )}
                    </div>
                  </ScrollArea>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
