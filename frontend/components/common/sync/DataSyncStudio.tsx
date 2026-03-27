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

import dynamic from 'next/dynamic';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from 'react';
import {useTheme} from 'next-themes';
import {toast} from 'sonner';
import {
  Bug,
  Copy,
  ChevronDown,
  ChevronRight,
  Columns2,
  Folder,
  FolderPlus,
  FileCode2,
  FilePlus2,
  GitBranch,
  BarChart3,
  Maximize2,
  Play,
  RefreshCw,
  Save,
  Search,
  SquareTerminal,
  FolderTree,
  Database,
  ListTree,
  Square,
  Trash2,
  Funnel,
  GitCompareArrows,
  LayoutPanelTop,
  Globe2,
  MoreHorizontal,
  Pencil,
  Plus,
} from 'lucide-react';
import services from '@/lib/services';
import {cn} from '@/lib/utils';
import type {ClusterInfo} from '@/lib/services/cluster';
import type {
  CreateSyncTaskRequest,
  SyncDagResult,
  SyncFormat,
  SyncGlobalVariable,
  SyncJobInstance,
  SyncJobLogsResult,
  SyncJSON,
  SyncPreviewDataset,
  SyncTask,
  SyncTaskTreeNode,
  SyncTaskVersion,
  SyncValidateResult,
} from '@/lib/services/sync';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip';
import {WebUiDagPreview} from '@/components/common/sync/WebUiDagPreview';

const MonacoEditor = dynamic(() => import('@monaco-editor/react'), {
  ssr: false,
});

const MonacoDiffEditor = dynamic(
  () => import('@monaco-editor/react').then((module) => module.DiffEditor),
  {
    ssr: false,
  },
);

interface EditorState {
  id?: number;
  parentId?: number;
  name: string;
  description: string;
  clusterId: string;
  contentFormat: SyncFormat;
  content: string;
  definition: SyncJSON;
  currentVersion: number;
  status: string;
}

interface TreeContextMenuState {
  open: boolean;
  x: number;
  y: number;
  kind: 'root' | 'folder' | 'file';
  node: SyncTaskTreeNode | null;
}

interface TreeDialogState {
  open: boolean;
  action: 'create-folder' | 'create-file' | 'rename' | 'move' | 'delete' | null;
  targetNode: SyncTaskTreeNode | null;
  name: string;
  targetParentId: number | null;
}

interface OpenFileTab {
  id: number;
  name: string;
}

interface PersistedWorkspaceTabs {
  openTabIds: number[];
  activeTabId: number | null;
}

interface VariableRow {
  id: string;
  key: string;
  value: string;
}

interface VariableDraft {
  key: string;
  value: string;
}

interface UserFacingErrorState {
  title: string;
  description: string;
  raw?: string;
}

type RightSidebarTab = 'settings' | 'versions' | 'globals';
type BottomConsoleTab = 'logs' | 'jobs' | 'preview';
type ExecutionMode = 'cluster' | 'local';
type LogFilterMode = 'all' | 'warn' | 'error';

const LOG_CHUNK_BASE_BYTES = 64 * 1024;
const LOG_CHUNK_MAX_BYTES = 1024 * 1024;
const EXPANDED_LOG_CHUNK_BASE_BYTES = 256 * 1024;
const EXPANDED_LOG_CHUNK_MAX_BYTES = 2 * 1024 * 1024;
const WORKSPACE_TABS_STORAGE_KEY = 'data-sync-studio:workspace-tabs';

function nextLogChunkSize(
  current: number,
  logs: string,
  min: number,
  max: number,
): number {
  const actualBytes = new TextEncoder().encode(logs || '').length;
  if (actualBytes >= Math.floor(current * 0.8) && current < max) {
    return Math.min(max, current * 2);
  }
  if (
    actualBytes > 0 &&
    actualBytes <= Math.floor(current * 0.25) &&
    current > min
  ) {
    return Math.max(min, Math.floor(current / 2));
  }
  return current;
}
type MetricGroupKey =
  | 'read'
  | 'write'
  | 'throughput'
  | 'latency'
  | 'status'
  | 'other';

const EMPTY_EDITOR: EditorState = {
  name: '',
  description: '',
  clusterId: '',
  contentFormat: 'hocon',
  content: '',
  definition: {},
  currentVersion: 0,
  status: 'draft',
};

const WORKSPACE_NAME_PATTERN = /^[\p{L}\p{N}._-]+$/u;

function toObject(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function flattenTree(nodes: SyncTaskTreeNode[]): SyncTaskTreeNode[] {
  return nodes.flatMap((node) => [node, ...flattenTree(node.children || [])]);
}

function collectFolderIds(nodes: SyncTaskTreeNode[]): number[] {
  return flattenTree(nodes)
    .filter((node) => node.node_type === 'folder')
    .map((node) => node.id);
}

function findTreeNode(
  nodes: SyncTaskTreeNode[],
  nodeId: number,
): SyncTaskTreeNode | null {
  for (const node of nodes) {
    if (node.id === nodeId) {
      return node;
    }
    const child = findTreeNode(node.children || [], nodeId);
    if (child) {
      return child;
    }
  }
  return null;
}

function isTreeDescendant(
  nodes: SyncTaskTreeNode[],
  ancestorId: number,
  candidateId: number,
): boolean {
  const ancestor = findTreeNode(nodes, ancestorId);
  if (!ancestor) {
    return false;
  }
  return flattenTree(ancestor.children || []).some(
    (node) => node.id === candidateId,
  );
}

function listMoveTargets(
  nodes: SyncTaskTreeNode[],
  source: SyncTaskTreeNode | null,
): Array<{label: string; value: number | null; depth: number}> {
  const buildPathLabel = (target: SyncTaskTreeNode): string => {
    const segments: string[] = [target.name];
    let cursor = target.parent_id
      ? findTreeNode(nodes, target.parent_id)
      : null;
    while (cursor) {
      segments.unshift(cursor.name);
      cursor = cursor.parent_id ? findTreeNode(nodes, cursor.parent_id) : null;
    }
    return `/${segments.join('/')}`;
  };
  const folders = flattenTree(nodes).filter(
    (node) => node.node_type === 'folder',
  );
  const options: Array<{label: string; value: number | null; depth: number}> =
    source?.node_type === 'file'
      ? []
      : [{label: '/ 根目录', value: null, depth: 0}];
  for (const folder of folders) {
    if (source) {
      if (folder.id === source.id) {
        continue;
      }
      if (
        source.node_type === 'folder' &&
        isTreeDescendant(nodes, source.id, folder.id)
      ) {
        continue;
      }
    }
    options.push({
      label: buildPathLabel(folder),
      value: folder.id,
      depth: buildPathLabel(folder).split('/').filter(Boolean).length,
    });
  }
  return options;
}

function patchTreeNode(
  nodes: SyncTaskTreeNode[],
  task: SyncTask,
): SyncTaskTreeNode[] {
  return nodes.map((node) => {
    if (node.id === task.id) {
      return {
        ...node,
        parent_id: task.parent_id,
        node_type: task.node_type,
        name: task.name,
        description: task.description,
        cluster_id: task.cluster_id,
        content_format: task.content_format,
        content: task.content,
        definition: task.definition,
        current_version: task.current_version,
        status: task.status,
        job_name: task.job_name,
      };
    }
    if (node.children && node.children.length > 0) {
      return {...node, children: patchTreeNode(node.children, task)};
    }
    return node;
  });
}

function filterTree(
  nodes: SyncTaskTreeNode[],
  keyword: string,
): SyncTaskTreeNode[] {
  const trimmed = keyword.trim().toLowerCase();
  if (!trimmed) {
    return nodes;
  }
  return nodes
    .map((node) => {
      const children = filterTree(node.children || [], keyword);
      const matched = node.name.toLowerCase().includes(trimmed);
      if (matched || children.length > 0) {
        return {...node, children};
      }
      return null;
    })
    .filter(Boolean) as SyncTaskTreeNode[];
}

function detectVariables(content: string): string[] {
  const matches = [...content.matchAll(/\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}/g)];
  return Array.from(
    new Set(
      matches.map((match) => match[1]?.trim()).filter(Boolean) as string[],
    ),
  ).sort();
}

function validateWorkspaceName(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) {
    return '请输入名称';
  }
  if (!WORKSPACE_NAME_PATTERN.test(trimmed)) {
    return '仅支持字母、数字、点、下划线和短横线，不支持空格';
  }
  return null;
}

function formatSyncUserFacingError(
  error: unknown,
  fallbackTitle: string,
): UserFacingErrorState {
  const message =
    error instanceof Error ? error.message : '发生未知错误，请稍后重试';
  if (message.includes('sync: 配置解析失败')) {
    return {
      title: '配置解析失败',
      description: message.replace(/^sync:\s*/, ''),
      raw: error instanceof Error ? error.message : message,
    };
  }
  if (message.includes('sync: DAG 解析失败')) {
    return {
      title: fallbackTitle,
      description: message.replace(/^sync:\s*/, ''),
      raw: error instanceof Error ? error.message : message,
    };
  }
  return {
    title: fallbackTitle,
    description: message,
    raw: error instanceof Error ? error.message : undefined,
  };
}

function toVariableRows(value: unknown): VariableRow[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return [];
  }
  return Object.entries(value as Record<string, unknown>).map(
    ([key, item], index) => ({
      id: `${key}-${index}`,
      key,
      value: typeof item === 'string' ? item : String(item ?? ''),
    }),
  );
}

function fromVariableRows(rows: VariableRow[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) {
      continue;
    }
    result[key] = row.value;
  }
  return result;
}

function getExecutionMode(definition: SyncJSON | undefined): ExecutionMode {
  const value = definition?.execution_mode;
  if (value === 'local') {
    return 'local';
  }
  return 'cluster';
}

function extractPreviewRows(
  resultPreview: SyncJSON | undefined,
): Array<Record<string, unknown>> {
  const rows = resultPreview?.rows;
  if (!Array.isArray(rows)) {
    return [];
  }
  return rows.filter(
    (item) => item && typeof item === 'object' && !Array.isArray(item),
  ) as Array<Record<string, unknown>>;
}

function extractPreviewDatasets(
  resultPreview: SyncJSON | undefined,
): SyncPreviewDataset[] {
  const datasets = resultPreview?.datasets;
  if (Array.isArray(datasets)) {
    return datasets
      .filter(
        (item) => item && typeof item === 'object' && !Array.isArray(item),
      )
      .map((item, index) => {
        const mapped = item as SyncJSON;
        const rows = Array.isArray(mapped.rows)
          ? (mapped.rows.filter(
              (row) => row && typeof row === 'object' && !Array.isArray(row),
            ) as SyncJSON[])
          : [];
        const explicitColumns = Array.isArray(mapped.columns)
          ? mapped.columns.map((column) => String(column))
          : rows.length > 0
            ? Object.keys(rows[0])
            : [];
        return {
          name:
            typeof mapped.name === 'string'
              ? mapped.name
              : `dataset-${index + 1}`,
          catalog: toObject(mapped.catalog),
          columns: explicitColumns,
          rows,
          page: typeof mapped.page === 'number' ? mapped.page : 1,
          page_size:
            typeof mapped.page_size === 'number'
              ? mapped.page_size
              : rows.length || 20,
          total: typeof mapped.total === 'number' ? mapped.total : rows.length,
          updated_at:
            typeof mapped.updated_at === 'string'
              ? mapped.updated_at
              : undefined,
        } satisfies SyncPreviewDataset;
      });
  }
  const rows = extractPreviewRows(resultPreview);
  const columns = extractPreviewColumns(rows, resultPreview);
  if (rows.length === 0 && columns.length === 0) {
    return [];
  }
  return [
    {
      name: 'preview_dataset',
      catalog: {},
      columns,
      rows,
      page: 1,
      page_size: rows.length || 20,
      total: rows.length,
    },
  ];
}

function extractPreviewColumns(
  rows: Array<Record<string, unknown>>,
  resultPreview: SyncJSON | undefined,
): string[] {
  const explicit = resultPreview?.columns;
  if (Array.isArray(explicit)) {
    return explicit.map((item) => String(item));
  }
  if (rows.length > 0) {
    return Object.keys(rows[0]);
  }
  return [];
}

function formatCellValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '-';
  }
  if (typeof value === 'object') {
    return JSON.stringify(value);
  }
  return String(value);
}

function getEngineAPIMode(job: SyncJobInstance | null): string {
  const mode = job?.submit_spec?.engine_api_mode;
  if (typeof mode === 'string' && mode.trim()) {
    return mode.trim().toLowerCase();
  }
  return 'v2';
}

function submitSpecExecutionMode(spec: SyncJSON | undefined): ExecutionMode {
  if (spec?.execution_mode === 'local') {
    return 'local';
  }
  return 'cluster';
}

function getEngineEndpointLabel(job: SyncJobInstance | null): string {
  if (job && submitSpecExecutionMode(job.submit_spec) === 'local') {
    const installDir = job.submit_spec?.install_dir;
    return typeof installDir === 'string' && installDir.trim()
      ? installDir.trim()
      : 'local-agent';
  }
  const baseURL = job?.submit_spec?.engine_base_url;
  if (typeof baseURL === 'string' && baseURL.trim()) {
    return baseURL.trim();
  }
  return '-';
}

function getJobStatusBadgeClass(status: string): string {
  switch (status) {
    case 'success':
      return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400';
    case 'running':
      return 'border-sky-500/30 bg-sky-500/10 text-sky-600 dark:text-sky-400';
    case 'failed':
      return 'border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400';
    case 'canceled':
      return 'border-rose-500/30 bg-zinc-500/10 text-rose-600 dark:text-rose-400';
    default:
      return 'border-border/60 bg-muted/50 text-muted-foreground';
  }
}

function getJobStatusLabel(status: string): string {
  switch (status) {
    case 'success':
      return 'Success';
    case 'running':
      return 'Running';
    case 'failed':
      return 'Failed';
    case 'canceled':
      return 'Canceled';
    case 'pending':
      return 'Pending';
    default:
      return status || '-';
  }
}

function parseMetricNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

function extractJobMetricSummary(job: SyncJobInstance): {
  readCount: number | null;
  writeCount: number | null;
  averageSpeed: number | null;
  metricCount: number;
} {
  const metrics = toObject(job.result_preview?.metrics);
  const readCount = parseMetricNumber(metrics.SourceReceivedCount);
  const writeCount =
    parseMetricNumber(metrics.SinkWriteCount) ??
    parseMetricNumber(metrics.SinkCommittedCount);
  const readQps = parseMetricNumber(metrics.SourceReceivedQPS);
  const writeQps =
    parseMetricNumber(metrics.SinkWriteQPS) ??
    parseMetricNumber(metrics.SinkCommittedQPS);
  let averageSpeed: number | null = null;
  if (readQps !== null && writeQps !== null) {
    averageSpeed = (readQps + writeQps) / 2;
  } else if (readQps !== null) {
    averageSpeed = readQps;
  } else if (writeQps !== null) {
    averageSpeed = writeQps;
  }
  return {
    readCount,
    writeCount,
    averageSpeed,
    metricCount: Object.keys(metrics).length,
  };
}

function formatMetricValue(value: number | null, digits = 0): string {
  if (value === null) {
    return '-';
  }
  return digits > 0 ? value.toFixed(digits) : String(Math.round(value));
}

function buildDisplayLogLines(logs: string, maxLines: number): string[] {
  const lines = splitLogLines(logs);
  if (lines.length <= maxLines) {
    return lines;
  }
  return lines.slice(lines.length - maxLines);
}

function splitLogLines(logs: string): string[] {
  return logs.split('\n').filter((line) => line.trim() !== '');
}

function getLogLineClass(line: string): string {
  const upper = line.toUpperCase();
  if (upper.includes(' ERROR ') || upper.includes('ERROR')) {
    return 'text-red-600 dark:text-red-400';
  }
  if (upper.includes(' WARN ') || upper.includes('WARNING')) {
    return 'text-amber-600 dark:text-amber-400';
  }
  return 'text-muted-foreground';
}

function extractEditorState(task?: SyncTask | null): EditorState {
  if (!task) {
    return EMPTY_EDITOR;
  }
  return {
    id: task.id,
    parentId: task.parent_id,
    name: task.name || '',
    description: task.description || '',
    clusterId: task.cluster_id ? String(task.cluster_id) : '',
    contentFormat: 'hocon',
    content: task.content || '',
    definition: task.definition || {},
    currentVersion: task.current_version || 0,
    status: task.status || 'draft',
  };
}

function extractEditorStateFromTreeNode(
  task?: SyncTaskTreeNode | null,
): EditorState {
  if (!task) {
    return EMPTY_EDITOR;
  }
  return {
    id: task.id,
    parentId: task.parent_id,
    name: task.name,
    description: task.description || '',
    clusterId: task.cluster_id ? String(task.cluster_id) : '',
    contentFormat: 'hocon',
    content: task.content || '',
    definition: task.definition || {},
    currentVersion: task.current_version || 0,
    status: task.status || 'draft',
  };
}

function resolveFolderParent(
  selectedNodeId: number | null,
  tree: SyncTaskTreeNode[],
): number | null {
  if (!selectedNodeId) {
    return null;
  }
  const node = flattenTree(tree).find((item) => item.id === selectedNodeId);
  if (!node) {
    return null;
  }
  return node.node_type === 'folder' ? node.id : node.parent_id || null;
}

function resolveDefaultPreviewHTTPSinkURL(): string {
  if (typeof window !== 'undefined' && window.location?.origin) {
    return `${window.location.origin}/api/v1/sync/preview/collect`;
  }
  return 'http://127.0.0.1:8000/api/v1/sync/preview/collect';
}

function buildDefaultContent(format: SyncFormat): string {
  return 'env {\n  job.mode = "batch"\n}\n\nsource {\n}\n\ntransform {\n}\n\nsink {\n}\n';
}

function formatMetricDisplayValue(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return '-';
  }
  if (typeof value === 'number') {
    return Number.isInteger(value) ? String(value) : value.toFixed(2);
  }
  if (typeof value === 'object') {
    return JSON.stringify(value);
  }
  return String(value);
}

function classifyMetricGroup(key: string): MetricGroupKey {
  const normalized = key.toLowerCase();
  if (
    normalized.includes('source') ||
    normalized.includes('read') ||
    normalized.includes('receive')
  ) {
    return 'read';
  }
  if (
    normalized.includes('sink') ||
    normalized.includes('write') ||
    normalized.includes('commit')
  ) {
    return 'write';
  }
  if (
    normalized.includes('qps') ||
    normalized.includes('tps') ||
    normalized.includes('speed') ||
    normalized.includes('rate') ||
    normalized.includes('throughput')
  ) {
    return 'throughput';
  }
  if (
    normalized.includes('latency') ||
    normalized.includes('delay') ||
    normalized.includes('duration') ||
    normalized.includes('cost')
  ) {
    return 'latency';
  }
  if (
    normalized.includes('status') ||
    normalized.includes('error') ||
    normalized.includes('fail') ||
    normalized.includes('retry')
  ) {
    return 'status';
  }
  return 'other';
}

function buildMetricGroups(metrics: unknown): Array<{
  key: MetricGroupKey;
  title: string;
  items: Array<{key: string; value: unknown}>;
}> {
  const rawMetrics = Object.entries(toObject(metrics));
  const groups: Record<MetricGroupKey, Array<{key: string; value: unknown}>> = {
    read: [],
    write: [],
    throughput: [],
    latency: [],
    status: [],
    other: [],
  };
  for (const [key, value] of rawMetrics) {
    groups[classifyMetricGroup(key)].push({key, value});
  }
  const metadata: Array<{key: MetricGroupKey; title: string}> = [
    {key: 'read', title: '读取指标'},
    {key: 'write', title: '写入指标'},
    {key: 'throughput', title: '速度指标'},
    {key: 'latency', title: '时延指标'},
    {key: 'status', title: '状态指标'},
    {key: 'other', title: '其他指标'},
  ];
  return metadata
    .map((item) => ({
      ...item,
      items: groups[item.key].sort((left, right) =>
        left.key.localeCompare(right.key),
      ),
    }))
    .filter((item) => item.items.length > 0);
}

export function DataSyncStudio() {
  const {resolvedTheme} = useTheme();
  const [clusters, setClusters] = useState<ClusterInfo[]>([]);
  const [tree, setTree] = useState<SyncTaskTreeNode[]>([]);
  const [keyword, setKeyword] = useState('');
  const [selectedNodeId, setSelectedNodeId] = useState<number | null>(null);
  const [selectedFolderId, setSelectedFolderId] = useState<number | null>(null);
  const [editor, setEditor] = useState<EditorState>(EMPTY_EDITOR);
  const [dagResult, setDagResult] = useState<SyncDagResult | null>(null);
  const [dagError, setDagError] = useState<UserFacingErrorState | null>(null);
  const [dagOpen, setDagOpen] = useState(false);
  const [validationOpen, setValidationOpen] = useState(false);
  const [validationTitle, setValidationTitle] = useState('校验配置');
  const [validationResult, setValidationResult] =
    useState<SyncValidateResult | null>(null);
  const [versions, setVersions] = useState<SyncTaskVersion[]>([]);
  const [globalVariables, setGlobalVariables] = useState<SyncGlobalVariable[]>(
    [],
  );
  const [versionTotal, setVersionTotal] = useState(0);
  const [globalVariableTotal, setGlobalVariableTotal] = useState(0);
  const [versionPage, setVersionPage] = useState(1);
  const [globalVariablePage, setGlobalVariablePage] = useState(1);
  const [rightSidebarTab, setRightSidebarTab] =
    useState<RightSidebarTab>('settings');
  const [bottomConsoleTab, setBottomConsoleTab] =
    useState<BottomConsoleTab>('logs');
  const [versionPreview, setVersionPreview] = useState<SyncTaskVersion | null>(
    null,
  );
  const [compareVersion, setCompareVersion] = useState<SyncTaskVersion | null>(
    null,
  );
  const [jobs, setJobs] = useState<SyncJobInstance[]>([]);
  const [selectedJobId, setSelectedJobId] = useState<number | null>(null);
  const [jobLogs, setJobLogs] = useState<SyncJobLogsResult | null>(null);
  const [expandedJobLogs, setExpandedJobLogs] =
    useState<SyncJobLogsResult | null>(null);
  const [jobLogsOffset, setJobLogsOffset] = useState('');
  const [expandedJobLogsOffset, setExpandedJobLogsOffset] = useState('');
  const jobLogsOffsetRef = useRef('');
  const expandedJobLogsOffsetRef = useRef('');
  const jobLogChunkSizeRef = useRef(LOG_CHUNK_BASE_BYTES);
  const expandedJobLogChunkSizeRef = useRef(EXPANDED_LOG_CHUNK_BASE_BYTES);
  const jobLogsAbortRef = useRef<AbortController | null>(null);
  const expandedJobLogsAbortRef = useRef<AbortController | null>(null);
  const jobLogsRequestVersionRef = useRef(0);
  const [logsLoading, setLogsLoading] = useState(false);
  const [expandedLogsLoading, setExpandedLogsLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [recoverSourceId, setRecoverSourceId] = useState<string>('');
  const [previewDatasetName, setPreviewDatasetName] = useState('');
  const [previewPage, setPreviewPage] = useState(1);
  const [openTabs, setOpenTabs] = useState<OpenFileTab[]>([]);
  const [expandedFolderIds, setExpandedFolderIds] = useState<number[]>([]);
  const [customVariableRows, setCustomVariableRows] = useState<VariableRow[]>(
    [],
  );
  const [editingCustomVariableId, setEditingCustomVariableId] = useState<string | null>(null);
  const [customVariableDraft, setCustomVariableDraft] = useState<VariableDraft>({
    key: '',
    value: '',
  });
  const [jobMetricsDialogOpen, setJobMetricsDialogOpen] = useState(false);
  const [metricsDialogJob, setMetricsDialogJob] =
    useState<SyncJobInstance | null>(null);
  const [logsDialogOpen, setLogsDialogOpen] = useState(false);
  const [logFilterMode, setLogFilterMode] = useState<LogFilterMode>('all');
  const [logSearchTerm, setLogSearchTerm] = useState('');
  const [treeMenu, setTreeMenu] = useState<TreeContextMenuState>({
    open: false,
    x: 0,
    y: 0,
    kind: 'root',
    node: null,
  });
  const [treeDialog, setTreeDialog] = useState<TreeDialogState>({
    open: false,
    action: null,
    targetNode: null,
    name: '',
    targetParentId: null,
  });
  const [editingGlobalVariableId, setEditingGlobalVariableId] = useState<
    number | null
  >(null);
  const restoredWorkspaceTabsRef = useRef<PersistedWorkspaceTabs | null>(null);

  if (
    restoredWorkspaceTabsRef.current === null &&
    typeof window !== 'undefined'
  ) {
    try {
      const raw = window.localStorage.getItem(WORKSPACE_TABS_STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as Partial<PersistedWorkspaceTabs>;
        restoredWorkspaceTabsRef.current = {
          openTabIds: Array.isArray(parsed.openTabIds)
            ? parsed.openTabIds
                .map((value) => Number(value))
                .filter((value) => Number.isInteger(value) && value > 0)
            : [],
          activeTabId:
            typeof parsed.activeTabId === 'number' &&
            Number.isInteger(parsed.activeTabId) &&
            parsed.activeTabId > 0
              ? parsed.activeTabId
              : null,
        };
      } else {
        restoredWorkspaceTabsRef.current = {openTabIds: [], activeTabId: null};
      }
    } catch {
      restoredWorkspaceTabsRef.current = {openTabIds: [], activeTabId: null};
    }
  }

  const filteredTree = useMemo(
    () => filterTree(tree, keyword),
    [tree, keyword],
  );
  const detectedVariables = useMemo(
    () => detectVariables(editor.content),
    [editor.content],
  );
  const previewJob = useMemo(
    () => jobs.find((job) => job.run_type === 'preview') || null,
    [jobs],
  );
  const runJobs = useMemo(
    () =>
      jobs.filter(
        (job) => job.run_type === 'run' || job.run_type === 'recover',
      ),
    [jobs],
  );
  const selectedJob = useMemo(
    () => jobs.find((job) => job.id === selectedJobId) || jobs[0] || null,
    [jobs, selectedJobId],
  );
  const previewDatasets = useMemo(
    () => extractPreviewDatasets(previewJob?.result_preview),
    [previewJob],
  );
  const selectedPreviewDataset = useMemo(() => {
    if (previewDatasets.length === 0) {
      return null;
    }
    return (
      previewDatasets.find((dataset) => dataset.name === previewDatasetName) ||
      previewDatasets[0]
    );
  }, [previewDatasetName, previewDatasets]);
  const activeJobs = useMemo(
    () =>
      jobs.filter(
        (job) => job.status === 'pending' || job.status === 'running',
      ),
    [jobs],
  );
  const hasActivePreview = activeJobs.some((job) => job.run_type === 'preview');
  const hasActiveRun = activeJobs.some(
    (job) => job.run_type === 'run' || job.run_type === 'recover',
  );
  const dagNodes = useMemo(
    () => (Array.isArray(dagResult?.nodes) ? dagResult?.nodes : []),
    [dagResult],
  );
  const dagEdges = useMemo(
    () => (Array.isArray(dagResult?.edges) ? dagResult?.edges : []),
    [dagResult],
  );
  const dagWarnings = useMemo(
    () => (Array.isArray(dagResult?.warnings) ? dagResult.warnings : []),
    [dagResult],
  );
  const dagWebUIJob = dagResult?.webui_job ?? null;
  const monacoTheme = resolvedTheme === 'light' ? 'vs' : 'vs-dark';
  const executionMode = useMemo(
    () => getExecutionMode(editor.definition),
    [editor.definition],
  );
  const fileCount = useMemo(
    () => flattenTree(tree).filter((node) => node.node_type === 'file').length,
    [tree],
  );
  const moveTargetOptions = useMemo(
    () => listMoveTargets(tree, treeDialog.targetNode),
    [tree, treeDialog.targetNode],
  );

  const syncOpenTabs = useCallback((task: Pick<SyncTask, 'id' | 'name'>) => {
    setOpenTabs((current) => {
      const next = current.filter((tab) => tab.id !== task.id);
      return [...next, {id: task.id, name: task.name || `#${task.id}`}];
    });
  }, []);

  const loadWorkspace = useCallback(
    async (preferredFileId?: number | null) => {
      setLoading(true);
      try {
        const [clusterData, treeData] = await Promise.all([
          services.cluster.getClusters({current: 1, size: 100}),
          services.sync.getTree(),
        ]);
        const items = treeData.items || [];
        setClusters(clusterData.clusters || []);
        setTree(items);

        const allFiles = flattenTree(items).filter(
          (node) => node.node_type === 'file',
        );
        const restoredTabs = (
          restoredWorkspaceTabsRef.current?.openTabIds || []
        )
          .map((id) => allFiles.find((node) => node.id === id))
          .filter((node): node is SyncTaskTreeNode => Boolean(node))
          .map((node) => ({id: node.id, name: node.name || `#${node.id}`}));
        const restoredActiveId =
          restoredWorkspaceTabsRef.current?.activeTabId ?? null;
        const nextSelected =
          preferredFileId &&
          allFiles.find((node) => node.id === preferredFileId)?.id
            ? preferredFileId
            : restoredActiveId &&
                allFiles.find((node) => node.id === restoredActiveId)?.id
              ? restoredActiveId
            : null;

        setOpenTabs(restoredTabs);
        setSelectedNodeId(nextSelected);
        if (nextSelected) {
          const treeTask =
            allFiles.find((node) => node.id === nextSelected) || null;
          setEditor(extractEditorStateFromTreeNode(treeTask));
          setJobs([]);
          setSelectedJobId(null);
          setJobLogs(null);
          setExpandedJobLogs(null);
          if (treeTask) {
            syncOpenTabs(treeTask);
          }
          const task = await services.sync.getTask(nextSelected);
          setEditor(extractEditorState(task));
          syncOpenTabs(task);
        } else {
          setEditor(EMPTY_EDITOR);
        }
      } catch (error) {
        toast.error(
          error instanceof Error ? error.message : '加载数据同步 Studio 失败',
        );
      } finally {
        setLoading(false);
      }
    },
    [syncOpenTabs],
  );

  const loadJobs = useCallback(async (taskId: number | null) => {
    if (!taskId) {
      setJobs([]);
      setSelectedJobId(null);
      setJobLogs(null);
      setExpandedJobLogs(null);
      return;
    }
    try {
      const data = await services.sync.listJobs({
        current: 1,
        size: 50,
        task_id: taskId,
      });
      const items = data.items || [];
      setJobs(items);
      setSelectedJobId((prev) => {
        if (prev && items.some((item) => item.id === prev)) {
          return prev;
        }
        return items[0]?.id || null;
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '加载运行历史失败');
    }
  }, []);

  const loadVersions = useCallback(
    async (taskId: number | null) => {
      if (!taskId) {
        setVersions([]);
        setVersionTotal(0);
        return;
      }
      try {
        const data = await services.sync.listVersions(taskId, {
          current: versionPage,
          size: 10,
        });
        setVersions(data.items || []);
        setVersionTotal(data.total || 0);
      } catch (error) {
        toast.error(
          error instanceof Error ? error.message : '加载版本历史失败',
        );
      }
    },
    [versionPage],
  );

  const loadGlobalVariables = useCallback(async () => {
    try {
      const data = await services.sync.listGlobalVariables({
        current: globalVariablePage,
        size: 8,
      });
      setGlobalVariables(data.items || []);
      setGlobalVariableTotal(data.total || 0);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '加载全局变量失败');
    }
  }, [globalVariablePage]);

  useEffect(() => {
    void loadWorkspace();
  }, [loadWorkspace]);

  useEffect(() => {
    void loadGlobalVariables();
  }, [loadGlobalVariables]);

  useEffect(() => {
    const folderIds = new Set(collectFolderIds(tree));
    setExpandedFolderIds((current) =>
      current.filter((id) => folderIds.has(id)),
    );
  }, [tree]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const payload: PersistedWorkspaceTabs = {
      openTabIds: openTabs.map((tab) => tab.id),
      activeTabId: selectedNodeId,
    };
    window.localStorage.setItem(
      WORKSPACE_TABS_STORAGE_KEY,
      JSON.stringify(payload),
    );
  }, [openTabs, selectedNodeId]);

  useEffect(() => {
    if (selectedNodeId) {
      void loadJobs(selectedNodeId);
      void loadVersions(selectedNodeId);
    } else {
      setVersions([]);
      setVersionTotal(0);
    }
  }, [selectedNodeId, loadJobs, loadVersions]);

  useEffect(() => {
    setVersionPage(1);
  }, [selectedNodeId]);

  useEffect(() => {
    if (previewDatasets.length === 0) {
      setPreviewDatasetName('');
      setPreviewPage(1);
      return;
    }
    if (
      !previewDatasets.some((dataset) => dataset.name === previewDatasetName)
    ) {
      setPreviewDatasetName(previewDatasets[0]?.name || '');
      setPreviewPage(1);
    }
  }, [previewDatasetName, previewDatasets]);

  useEffect(() => {
    const rows = toVariableRows(editor.definition?.custom_variables);
    setCustomVariableRows(
      rows.length > 0 ? rows : [{id: 'custom-var-0', key: '', value: ''}],
    );
  }, [selectedNodeId, editor.currentVersion]);

  useEffect(() => {
    jobLogsAbortRef.current?.abort();
    expandedJobLogsAbortRef.current?.abort();
    jobLogsRequestVersionRef.current += 1;
    setJobLogs(null);
    setExpandedJobLogs(null);
    setJobLogsOffset('');
    setExpandedJobLogsOffset('');
    jobLogsOffsetRef.current = '';
    expandedJobLogsOffsetRef.current = '';
    jobLogChunkSizeRef.current = LOG_CHUNK_BASE_BYTES;
    expandedJobLogChunkSizeRef.current = EXPANDED_LOG_CHUNK_BASE_BYTES;
  }, [selectedJobId, logFilterMode, logSearchTerm]);

  const loadSelectedJobLogs = useCallback(
    async (all = false) => {
      if (!selectedJobId || (bottomConsoleTab !== 'logs' && !all)) {
        return;
      }
      const requestVersion = jobLogsRequestVersionRef.current;
      const abortRef = all ? expandedJobLogsAbortRef : jobLogsAbortRef;
      const currentOffsetRef = all
        ? expandedJobLogsOffsetRef
        : jobLogsOffsetRef;
      const chunkSizeRef = all
        ? expandedJobLogChunkSizeRef
        : jobLogChunkSizeRef;
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      if (all) {
        setExpandedLogsLoading(true);
      } else {
        setLogsLoading(true);
      }
      try {
        const currentOffset = currentOffsetRef.current;
        const result = await services.sync.getJobLogs(selectedJobId, {
          offset: currentOffset || undefined,
          limit_bytes: chunkSizeRef.current,
          keyword: logSearchTerm.trim() || undefined,
          level: logFilterMode === 'all' ? undefined : logFilterMode,
          signal: controller.signal,
        });
        if (
          controller.signal.aborted ||
          jobLogsRequestVersionRef.current !== requestVersion
        ) {
          return;
        }
        const mergedLogs = (previousLogs?: string) =>
          currentOffset
            ? {
                ...result,
                logs: result.logs
                  ? [previousLogs || '', result.logs].filter(Boolean).join('\n')
                  : previousLogs || '',
              }
            : result;
        const nextOffset = result.next_offset || currentOffset;
        currentOffsetRef.current = nextOffset;
        chunkSizeRef.current = nextLogChunkSize(
          chunkSizeRef.current,
          result.logs,
          all ? EXPANDED_LOG_CHUNK_BASE_BYTES : LOG_CHUNK_BASE_BYTES,
          all ? EXPANDED_LOG_CHUNK_MAX_BYTES : LOG_CHUNK_MAX_BYTES,
        );
        if (all) {
          setExpandedJobLogs((previous) => mergedLogs(previous?.logs));
          setExpandedJobLogsOffset(nextOffset);
        } else {
          setJobLogs((previous) => mergedLogs(previous?.logs));
          setJobLogsOffset(nextOffset);
        }
      } catch (error) {
        if (
          error instanceof Error &&
          (error.name === 'CanceledError' || error.name === 'AbortError')
        ) {
          return;
        }
        if (all) {
          setExpandedJobLogs(null);
          setExpandedJobLogsOffset('');
          expandedJobLogsOffsetRef.current = '';
          expandedJobLogChunkSizeRef.current = EXPANDED_LOG_CHUNK_BASE_BYTES;
        } else {
          setJobLogs(null);
          setJobLogsOffset('');
          jobLogsOffsetRef.current = '';
          jobLogChunkSizeRef.current = LOG_CHUNK_BASE_BYTES;
        }
        console.warn(
          error instanceof Error ? error.message : '加载任务日志失败',
        );
      } finally {
        if (abortRef.current === controller) {
          abortRef.current = null;
        }
        if (all) {
          setExpandedLogsLoading(false);
        } else {
          setLogsLoading(false);
        }
      }
    },
    [bottomConsoleTab, logFilterMode, logSearchTerm, selectedJobId],
  );

  useEffect(() => {
    if (!selectedJobId || bottomConsoleTab !== 'logs') {
      return;
    }
    void loadSelectedJobLogs();
  }, [
    bottomConsoleTab,
    loadSelectedJobLogs,
    selectedJobId,
    logFilterMode,
    logSearchTerm,
  ]);

  useEffect(() => {
    if (!logsDialogOpen || !selectedJobId) {
      expandedJobLogsAbortRef.current?.abort();
      return;
    }
    void loadSelectedJobLogs(true);
  }, [logsDialogOpen, selectedJobId, loadSelectedJobLogs]);

  useEffect(() => {
    if (bottomConsoleTab === 'logs') {
      return;
    }
    jobLogsAbortRef.current?.abort();
  }, [bottomConsoleTab]);

  useEffect(() => {
    return () => {
      jobLogsAbortRef.current?.abort();
      expandedJobLogsAbortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (activeJobs.length === 0) {
      return;
    }
    const timer = window.setInterval(() => {
      void (async () => {
        try {
          const refreshed = await Promise.all(
            activeJobs.map((job) => services.sync.getJob(job.id)),
          );
          setJobs((current) =>
            current.map(
              (job) => refreshed.find((item) => item.id === job.id) || job,
            ),
          );
        } catch {
          // 忽略瞬时轮询抖动，保持 Studio 可继续操作。
          // Ignore transient polling errors to keep the studio usable.
        }
      })();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [activeJobs]);

  useEffect(() => {
    if (!selectedJobId || bottomConsoleTab !== 'logs') {
      return;
    }
    const timer = window.setInterval(() => {
      void loadSelectedJobLogs();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [bottomConsoleTab, loadSelectedJobLogs, selectedJobId]);

  useEffect(() => {
    if (!logsDialogOpen || !selectedJobId) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadSelectedJobLogs(true);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [logsDialogOpen, loadSelectedJobLogs, selectedJobId]);

  useEffect(() => {
    if (!treeMenu.open) {
      return;
    }
    const handleClose = () => {
      setTreeMenu((prev) => ({...prev, open: false}));
    };
    window.addEventListener('click', handleClose);
    window.addEventListener('scroll', handleClose, true);
    return () => {
      window.removeEventListener('click', handleClose);
      window.removeEventListener('scroll', handleClose, true);
    };
  }, [treeMenu.open]);

  const updateEditor = <K extends keyof EditorState>(
    key: K,
    value: EditorState[K],
  ) => {
    setEditor((prev) => ({...prev, [key]: value}));
  };

  const buildTaskPayload = useCallback(
    (): CreateSyncTaskRequest => ({
      parent_id: editor.parentId,
      node_type: 'file',
      name: editor.name.trim(),
      description: editor.description.trim(),
      cluster_id: editor.clusterId ? Number(editor.clusterId) : 0,
      content_format: 'hocon',
      content: editor.content,
      job_name: editor.name.trim(),
      definition: {
        ...editor.definition,
        custom_variables: fromVariableRows(customVariableRows),
        execution_mode: getExecutionMode(editor.definition),
        preview_mode: 'source',
        preview_output_format: 'json',
        preview_http_sink: {
          url:
            typeof toObject(toObject(editor.definition).preview_http_sink)
              .url === 'string'
              ? String(
                  toObject(toObject(editor.definition).preview_http_sink).url,
                )
              : resolveDefaultPreviewHTTPSinkURL(),
          array_mode: false,
        },
      },
    }),
    [customVariableRows, editor],
  );

  const persistCurrentFile = useCallback(
    async (publishAfterSave = false) => {
      if (!editor.name.trim()) {
        toast.error('请先填写文件名称');
        return null;
      }
      if (!editor.content.trim()) {
        toast.error('请先填写任务内容');
        return null;
      }
      setSaving(true);
      try {
        const payload = buildTaskPayload();
        let task: SyncTask;
        if (editor.id) {
          task = await services.sync.updateTask(editor.id, payload);
        } else {
          task = await services.sync.createTask(payload);
        }
        const isNewTask = !editor.id;
        if (publishAfterSave) {
          await services.sync.publishTask(task.id, {
            comment: 'publish from data sync studio',
          });
          task = await services.sync.getTask(task.id);
        }
        setEditor(extractEditorState(task));
        syncOpenTabs(task);
        setSelectedNodeId(task.id);
        if (isNewTask) {
          await loadWorkspace(task.id);
        } else {
          setTree((current) => patchTreeNode(current, task));
        }
        return task;
      } catch (error) {
        toast.error(error instanceof Error ? error.message : '保存文件失败');
        return null;
      } finally {
        setSaving(false);
      }
    },
    [buildTaskPayload, editor, loadWorkspace, syncOpenTabs],
  );

  const openTreeDialog = useCallback(
    (
      action: TreeDialogState['action'],
      targetNode: SyncTaskTreeNode | null,
      initialName = '',
    ) => {
      const defaultParentId =
        action === 'move' ? targetNode?.parent_id || null : null;
      setTreeMenu((prev) => ({...prev, open: false}));
      setTreeDialog({
        open: true,
        action,
        targetNode,
        name: initialName,
        targetParentId: defaultParentId,
      });
    },
    [],
  );

  const openTreeContextMenu = useCallback(
    (
      event: MouseEvent,
      kind: TreeContextMenuState['kind'],
      node: SyncTaskTreeNode | null,
    ) => {
      event.preventDefault();
      event.stopPropagation();
      setTreeMenu({
        open: true,
        x: event.clientX,
        y: event.clientY,
        kind,
        node,
      });
    },
    [],
  );

  const handleTreeDialogSubmit = async () => {
    const name = treeDialog.name.trim();
    if (treeDialog.action !== 'move') {
      const nameError = validateWorkspaceName(name);
      if (nameError) {
        toast.error(nameError);
        return;
      }
    }
    try {
      if (treeDialog.action === 'create-folder') {
        const parentId =
          treeDialog.targetNode?.node_type === 'folder'
            ? treeDialog.targetNode.id
            : treeDialog.targetNode?.parent_id ||
              resolveFolderParent(selectedNodeId, tree);
        await services.sync.createTask({
          parent_id: parentId || undefined,
          node_type: 'folder',
          name,
          content_format: 'hocon',
        });
        toast.success('目录已创建');
        await loadWorkspace(selectedNodeId);
      } else if (treeDialog.action === 'create-file') {
        const parentId =
          treeDialog.targetNode?.node_type === 'folder'
            ? treeDialog.targetNode.id
            : treeDialog.targetNode?.parent_id ||
              resolveFolderParent(selectedNodeId, tree);
        if (!parentId) {
          toast.error('请先在目录下创建文件，根目录不允许直接创建文件');
          return;
        }
        const task = await services.sync.createTask({
          parent_id: parentId,
          node_type: 'file',
          name,
          cluster_id: editor.clusterId ? Number(editor.clusterId) : 0,
          content_format: 'hocon',
          content: buildDefaultContent('hocon'),
          definition: {},
        });
        toast.success('文件已创建');
        syncOpenTabs(task);
        await loadWorkspace(task.id);
      } else if (treeDialog.action === 'rename' && treeDialog.targetNode) {
        const current = await services.sync.getTask(treeDialog.targetNode.id);
        await services.sync.updateTask(treeDialog.targetNode.id, {
          parent_id: current.parent_id,
          node_type: current.node_type,
          name,
          description: current.description,
          cluster_id: current.cluster_id,
          content_format: current.content_format,
          content: current.content,
          definition: current.definition,
        });
        toast.success('名称已更新');
        await loadWorkspace(treeDialog.targetNode.id);
      } else if (treeDialog.action === 'move' && treeDialog.targetNode) {
        const current = await services.sync.getTask(treeDialog.targetNode.id);
        await services.sync.updateTask(treeDialog.targetNode.id, {
          parent_id: treeDialog.targetParentId || undefined,
          node_type: current.node_type,
          name: current.name,
          description: current.description,
          cluster_id: current.cluster_id,
          content_format: current.content_format,
          content: current.content,
          definition: current.definition,
        });
        toast.success('移动已完成');
        await loadWorkspace(treeDialog.targetNode.id);
      } else if (treeDialog.action === 'delete' && treeDialog.targetNode) {
        await services.sync.deleteTask(treeDialog.targetNode.id);
        setOpenTabs((current) =>
          current.filter((tab) => tab.id !== treeDialog.targetNode?.id),
        );
        if (selectedNodeId === treeDialog.targetNode.id) {
          setSelectedNodeId(null);
          setEditor(EMPTY_EDITOR);
          setJobs([]);
          setSelectedJobId(null);
        }
        toast.success(
          treeDialog.targetNode.node_type === 'folder'
            ? '目录已删除'
            : '文件已删除',
        );
        await loadWorkspace();
      }
      setTreeDialog({
        open: false,
        action: null,
        targetNode: null,
        name: '',
        targetParentId: null,
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '操作失败');
    }
  };

  const handleSelectNode = async (node: SyncTaskTreeNode) => {
    if (node.node_type === 'folder') {
      setSelectedFolderId(node.id);
      setExpandedFolderIds((current) =>
        current.includes(node.id)
          ? current.filter((id) => id !== node.id)
          : [...current, node.id],
      );
      return;
    }
    setSelectedNodeId(node.id);
    setSelectedFolderId(node.parent_id || null);
    setEditor(extractEditorStateFromTreeNode(node));
    syncOpenTabs(node);
    setDagResult(null);
    setJobs([]);
    setSelectedJobId(null);
    setJobLogs(null);
    setExpandedJobLogs(null);
    setBottomConsoleTab('logs');
    try {
      const task = await services.sync.getTask(node.id);
      setEditor(extractEditorState(task));
      syncOpenTabs(task);
      await loadJobs(node.id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '加载文件失败');
    }
  };

  const handleSelectTab = async (taskId: number) => {
    setSelectedNodeId(taskId);
    const treeTask = findTreeNode(tree, taskId);
    if (treeTask && treeTask.node_type === 'file') {
      setEditor(extractEditorStateFromTreeNode(treeTask));
      setSelectedFolderId(treeTask.parent_id || null);
      setJobs([]);
      setSelectedJobId(null);
      setJobLogs(null);
      setExpandedJobLogs(null);
    }
    try {
      const task = await services.sync.getTask(taskId);
      setEditor(extractEditorState(task));
      setSelectedFolderId(task.parent_id || null);
      await loadJobs(taskId);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '加载文件失败');
    }
  };

  const handleCloseTab = async (taskId: number) => {
    setOpenTabs((current) => current.filter((tab) => tab.id !== taskId));
    if (selectedNodeId !== taskId) {
      return;
    }
    const remaining = openTabs.filter((tab) => tab.id !== taskId);
    const nextTab = remaining[remaining.length - 1];
    if (nextTab) {
      await handleSelectTab(nextTab.id);
    } else {
      setSelectedNodeId(null);
      setEditor(EMPTY_EDITOR);
      setJobs([]);
      setSelectedJobId(null);
      setJobLogs(null);
      setExpandedJobLogs(null);
    }
  };

  const handleSave = async () => {
    const task = await persistCurrentFile(true);
    if (task) {
      await loadVersions(task.id);
      toast.success('文件已保存并生成新版本');
    }
  };

  const runPreflightValidation = async (
    taskId: number,
    actionLabel: string,
  ) => {
    const result = await services.sync.validateTask(taskId);
    if (!result.valid) {
      setValidationTitle('校验配置');
      setValidationResult(result);
      setValidationOpen(true);
      toast.error(`${actionLabel}前校验未通过，请先修复配置问题`);
      return false;
    }
    return true;
  };

  const handleBuildDag = async () => {
    const task = await persistCurrentFile(false);
    if (!task) {
      return;
    }
    try {
      const passed = await runPreflightValidation(task.id, 'DAG 解析');
      if (!passed) {
        return;
      }
      const result = await services.sync.buildDag(task.id);
      setDagResult(result);
      setDagError(null);
      setDagOpen(true);
      toast.success('DAG 已生成');
    } catch (error) {
      setDagResult(null);
      setDagError(formatSyncUserFacingError(error, 'DAG 解析失败'));
      setDagOpen(true);
      toast.error(
        error instanceof Error ? error.message : '生成 DAG 失败',
      );
    }
  };

  const handleValidateConfig = async () => {
    const task = await persistCurrentFile(false);
    if (!task) {
      return;
    }
    try {
      const result = await services.sync.validateTask(task.id);
      setValidationTitle('校验配置');
      setValidationResult(result);
      setValidationOpen(true);
      toast.success(result.valid ? '配置校验通过' : '配置校验完成');
    } catch (error) {
      const uiError = formatSyncUserFacingError(error, '配置校验失败');
      setValidationTitle(uiError.title);
      setValidationResult({
        valid: false,
        errors: [uiError.description],
        warnings: [],
        summary: uiError.title,
      });
      setValidationOpen(true);
      toast.error(uiError.description);
    }
  };

  const handleTestConnections = async () => {
    const task = await persistCurrentFile(false);
    if (!task) {
      return;
    }
    try {
      const result = await services.sync.testConnections(task.id);
      setValidationTitle('测试连接');
      setValidationResult(result);
      setValidationOpen(true);
      toast.success(result.valid ? '连接测试通过' : '连接测试完成');
    } catch (error) {
      const uiError = formatSyncUserFacingError(error, '测试连接失败');
      setValidationTitle(uiError.title);
      setValidationResult({
        valid: false,
        errors: [uiError.description],
        warnings: [],
        summary: uiError.title,
      });
      setValidationOpen(true);
      toast.error(uiError.description);
    }
  };

  const handlePreview = async () => {
    if (hasActiveRun || hasActivePreview) {
      toast.error('请先等待当前运行或预览结束，或先取消它');
      return;
    }
    const task = await persistCurrentFile(false);
    if (!task) {
      return;
    }
    try {
      const passed = await runPreflightValidation(task.id, '预览');
      if (!passed) {
        return;
      }
      const job = await services.sync.previewTask(task.id);
      await loadJobs(task.id);
      setSelectedJobId(job.id);
      setBottomConsoleTab('preview');
      toast.success('预览任务已提交');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '预览失败');
    }
  };

  const handleRun = async (mode: 'run' | 'recover') => {
    if (hasActiveRun || hasActivePreview) {
      toast.error('请先等待当前运行或预览结束，或先取消它');
      return;
    }
    const task = await persistCurrentFile(true);
    if (!task) {
      return;
    }
    try {
      const passed = await runPreflightValidation(
        task.id,
        mode === 'recover' ? '恢复' : '运行',
      );
      if (!passed) {
        return;
      }
      if (mode === 'recover' && !recoverSourceId && !runJobs[0]?.id) {
        throw new Error('当前没有可用于 Savepoint 恢复的历史运行实例');
      }
      const job =
        mode === 'recover'
          ? await services.sync.recoverJob(
              Number(recoverSourceId || runJobs[0]?.id || 0),
            )
          : await services.sync.submitTask(task.id);
      await loadJobs(task.id);
      setSelectedJobId(job.id);
      setBottomConsoleTab('jobs');
      toast.success(mode === 'recover' ? '恢复任务已提交' : '运行任务已提交');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '运行失败');
    }
  };

  const handleCancelJob = async (jobId: number, stopWithSavepoint = false) => {
    try {
      await services.sync.cancelJob(jobId, {
        stop_with_savepoint: stopWithSavepoint,
      });
      await loadJobs(editor.id || null);
      toast.success(
        stopWithSavepoint ? '任务已触发 Savepoint 停止' : '任务已停止',
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '取消任务失败');
    }
  };

  const handleStopActiveJob = async (mode: 'normal' | 'savepoint') => {
    const activeJob =
      selectedJob &&
      (selectedJob.status === 'pending' || selectedJob.status === 'running')
        ? selectedJob
        : activeJobs[0];
    if (!activeJob) {
      toast.error('当前没有可停止的运行实例');
      return;
    }
    await handleCancelJob(activeJob.id, mode === 'savepoint');
  };

  const handleRecoverFromHistory = (jobId: number) => {
    setRecoverSourceId(String(jobId));
    setBottomConsoleTab('jobs');
  };

  const handleExecutionModeChange = (value: ExecutionMode) => {
    updateEditor('definition', {
      ...editor.definition,
      execution_mode: value,
    });
  };

  const syncCustomVariablesToEditor = useCallback(
    (rows: VariableRow[]) => {
      setCustomVariableRows(rows);
      setEditingCustomVariableId(null);
      setCustomVariableDraft({key: '', value: ''});
      updateEditor('definition', {
        ...editor.definition,
        custom_variables: fromVariableRows(rows),
      });
    },
    [editor.definition],
  );

  const handleStartEditCustomVariableRow = (rowId: string) => {
    const target = customVariableRows.find((row) => row.id === rowId);
    if (!target) {
      return;
    }
    setEditingCustomVariableId(rowId);
    setCustomVariableDraft({key: target.key, value: target.value});
  };

  const handleCustomVariableDraftChange = (
    field: keyof VariableDraft,
    value: string,
  ) => {
    setCustomVariableDraft((current) => ({...current, [field]: value}));
  };

  const handleCancelEditCustomVariableRow = () => {
    setEditingCustomVariableId(null);
    setCustomVariableDraft({key: '', value: ''});
  };

  const handleSaveCustomVariableRow = (rowId: string) => {
    syncCustomVariablesToEditor(
      customVariableRows.map((row) =>
        row.id === rowId ? {...row, ...customVariableDraft} : row,
      ),
    );
    setEditingCustomVariableId(null);
    setCustomVariableDraft({key: '', value: ''});
  };

  const handleAddCustomVariableRow = () => {
    const id = `custom-var-${Date.now()}`;
    syncCustomVariablesToEditor([
      ...customVariableRows,
      {id, key: '', value: ''},
    ]);
    setEditingCustomVariableId(id);
    setCustomVariableDraft({key: '', value: ''});
  };

  const handleDeleteCustomVariableRow = (rowId: string) => {
    const nextRows = customVariableRows.filter((row) => row.id !== rowId);
    syncCustomVariablesToEditor(
      nextRows.length > 0
        ? nextRows
        : [{id: 'custom-var-0', key: '', value: ''}],
    );
    if (editingCustomVariableId === rowId) {
      setEditingCustomVariableId(null);
      setCustomVariableDraft({key: '', value: ''});
    }
  };

  const handleSaveGlobalVariable = async (
    item: SyncGlobalVariable | null,
    payload: {key: string; value: string; description: string},
  ) => {
    try {
      if (item) {
        await services.sync.updateGlobalVariable(item.id, payload);
        toast.success('全局变量已更新');
      } else {
        await services.sync.createGlobalVariable(payload);
        toast.success('全局变量已创建');
      }
      setEditingGlobalVariableId(null);
      await loadGlobalVariables();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '保存全局变量失败');
    }
  };

  const handleDeleteGlobalVariable = async (id: number) => {
    try {
      await services.sync.deleteGlobalVariable(id);
      toast.success('全局变量已删除');
      if (editingGlobalVariableId === id) {
        setEditingGlobalVariableId(null);
      }
      if (globalVariables.length === 1 && globalVariablePage > 1) {
        setGlobalVariablePage((current) => Math.max(1, current - 1));
        return;
      }
      await loadGlobalVariables();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除全局变量失败');
    }
  };

  const copyToClipboard = async (value: string, successText: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(successText);
    } catch {
      toast.error('复制失败');
    }
  };

  const handleRollbackVersion = async (versionId: number) => {
    if (!editor.id) {
      return;
    }
    try {
      const task = await services.sync.rollbackVersion(editor.id, versionId);
      setEditor(extractEditorState(task));
      setTree((current) => patchTreeNode(current, task));
      await loadVersions(task.id);
      toast.success('已回滚到指定版本');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '回滚版本失败');
    }
  };

  const handleDeleteVersion = async (versionId: number) => {
    if (!editor.id) {
      return;
    }
    try {
      await services.sync.deleteVersion(editor.id, versionId);
      if (versions.length === 1 && versionPage > 1) {
        setVersionPage((current) => Math.max(1, current - 1));
        return;
      }
      await loadVersions(editor.id);
      toast.success('版本已删除');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除版本失败');
    }
  };

  return (
    <div className='-mx-2 flex h-[calc(100vh-96px)] min-h-[780px] flex-col gap-2 bg-background/10 lg:-mx-3'>
      <Card className='gap-0 border-border/60 bg-background/85 py-0 shadow-sm'>
        <CardContent className='flex h-14 items-center justify-between gap-3 px-4 py-2'>
          <div className='flex min-w-0 items-center gap-3'>
            <FolderTree className='size-4 shrink-0 text-primary' />
            <div className='relative w-[240px]'>
              <Search className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
              <Input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                className='h-9 border-border/60 bg-background pl-9 text-sm'
                placeholder='搜索文件或目录'
              />
            </div>
          </div>

          <div className='flex flex-wrap items-center justify-end gap-2'>
            <Badge variant='outline' className='h-9 rounded-md px-3 text-sm'>
              HOCON
            </Badge>
            <Button
              size='sm'
              className='h-9 px-3'
              variant='outline'
              onClick={handleSave}
              disabled={saving || !editor.name.trim()}
            >
              <Save className='mr-1.5 size-4' />
              保存
            </Button>
            <Button
              size='sm'
              className='h-9 px-3'
              variant='outline'
              onClick={handleTestConnections}
              disabled={saving || !editor.name.trim()}
            >
              <Database className='mr-1.5 size-4' />
              测试连接
            </Button>
            <Button
              size='sm'
              className='h-9 px-3'
              variant='outline'
              onClick={handleBuildDag}
              disabled={saving || !editor.name.trim()}
            >
              <GitBranch className='mr-1.5 size-4' />
              DAG
            </Button>
            <Button
              size='sm'
              className='h-9 px-3'
              variant='outline'
              onClick={handlePreview}
              disabled={saving || hasActiveRun || hasActivePreview}
            >
              <Bug className='mr-1.5 size-4' />
              预览
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  size='sm'
                  className='h-9 px-3'
                  disabled={saving || hasActiveRun || hasActivePreview}
                >
                  <Play className='mr-1.5 size-4' />
                  运行
                  <ChevronDown className='ml-1.5 size-4' />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end'>
                <DropdownMenuItem onClick={() => void handleRun('run')}>
                  运行
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={executionMode === 'local'}
                  onClick={() => void handleRun('recover')}
                >
                  Savepoint 恢复
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  size='sm'
                  className='h-9 px-3'
                  variant='outline'
                  disabled={activeJobs.length === 0}
                >
                  <Square className='mr-1.5 size-4' />
                  停止
                  <ChevronDown className='ml-1.5 size-4' />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end'>
                <DropdownMenuItem
                  onClick={() => void handleStopActiveJob('normal')}
                >
                  正常停止
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={executionMode === 'local'}
                  onClick={() => void handleStopActiveJob('savepoint')}
                >
                  Savepoint 停止
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardContent>
      </Card>

      <div className='grid min-h-0 flex-1 grid-cols-[220px_minmax(0,1fr)_304px] grid-rows-[minmax(0,1fr)_260px] gap-2'>
        <Card className='row-start-1 gap-0 overflow-hidden border-border/60 bg-background/75 py-0 shadow-sm'>
          <CardContent className='flex h-full min-h-0 flex-col p-0'>
            <div className='flex items-center justify-between border-b border-border/50 px-3 py-2'>
              <div className='flex items-center gap-2 text-sm font-medium'>
                <Folder className='size-4 text-primary' />
                资源
              </div>
              <div className='flex items-center gap-1'>
                <Badge variant='outline' className='rounded-sm'>
                  {fileCount}
                </Badge>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size='icon'
                      variant='ghost'
                      className='size-7'
                      onClick={() =>
                        openTreeDialog(
                          'create-folder',
                          selectedFolderId
                            ? findTreeNode(tree, selectedFolderId)
                            : null,
                        )
                      }
                    >
                      <FolderPlus className='size-4' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>新建目录</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size='icon'
                      variant='ghost'
                      className='size-7'
                      onClick={() => {
                        const folderNode = selectedFolderId
                          ? findTreeNode(tree, selectedFolderId)
                          : null;
                        if (!folderNode) {
                          toast.error('请先选中一个目录后再创建文件');
                          return;
                        }
                        openTreeDialog('create-file', folderNode);
                      }}
                    >
                      <FilePlus2 className='size-4' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>新建文件</TooltipContent>
                </Tooltip>
              </div>
            </div>
            <ScrollArea
              className='min-h-0 flex-1'
              onContextMenu={(event) =>
                openTreeContextMenu(event, 'root', null)
              }
            >
              <div className='px-2 py-2'>
                {loading ? (
                  <div className='p-3 text-sm text-muted-foreground'>
                    加载中...
                  </div>
                ) : filteredTree.length === 0 ? (
                  <div className='p-3 text-sm text-muted-foreground'>
                    暂无工作区内容。
                  </div>
                ) : (
                  <TreeView
                    nodes={filteredTree}
                    selectedNodeId={selectedNodeId}
                    selectedFolderId={selectedFolderId}
                    expandedFolderIds={expandedFolderIds}
                    onSelect={handleSelectNode}
                    onContextMenu={openTreeContextMenu}
                  />
                )}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <Card className='row-start-1 gap-0 overflow-hidden border-border/60 bg-background/75 py-0 shadow-sm'>
          <CardContent className='flex h-full min-h-0 flex-col p-0'>
            <div className='flex min-h-9 items-end gap-0 overflow-x-auto border-b border-border/50 bg-background/80 px-1'>
              {openTabs.length > 0 ? (
                openTabs.map((tab) => (
                  <button
                    key={tab.id}
                    type='button'
                    className={cn(
                      'group -mb-px flex h-8 items-center gap-1.5 border-b-2 px-3 text-xs transition-colors',
                      selectedNodeId === tab.id
                        ? 'border-primary bg-primary/5 text-foreground'
                        : 'border-transparent text-muted-foreground hover:bg-muted/40 hover:text-foreground',
                    )}
                    onClick={() => void handleSelectTab(tab.id)}
                  >
                    <FileCode2 className='size-3.5' />
                    <span className='max-w-[180px] truncate'>{tab.name}</span>
                    <span
                      className='rounded px-1 text-[10px] opacity-60 transition hover:bg-muted hover:opacity-100'
                      onClick={(event) => {
                        event.stopPropagation();
                        void handleCloseTab(tab.id);
                      }}
                    >
                      ×
                    </span>
                  </button>
                ))
              ) : (
                <div className='px-3 py-2 text-xs text-muted-foreground'>
                  暂无打开文件
                </div>
              )}
            </div>
            <div className='min-h-0 flex-1'>
              <MonacoEditor
                height='100%'
                language={editor.contentFormat === 'json' ? 'json' : 'shell'}
                theme={monacoTheme}
                value={editor.content}
                onChange={(value) => updateEditor('content', value || '')}
                options={{
                  minimap: {enabled: true},
                  fontSize: 13,
                  wordWrap: 'on',
                  automaticLayout: true,
                  scrollBeyondLastLine: false,
                  smoothScrolling: true,
                  tabSize: 2,
                  renderLineHighlight: 'all',
                  padding: {top: 14, bottom: 14},
                }}
              />
            </div>
          </CardContent>
        </Card>

        <StudioSidebarShell
          className='row-span-2'
          rail={
            <>
              <SidebarIconTab
                active={rightSidebarTab === 'settings'}
                icon={<Database className='size-4' />}
                label='设置'
                onClick={() => setRightSidebarTab('settings')}
              />
              <SidebarIconTab
                active={rightSidebarTab === 'versions'}
                icon={<GitBranch className='size-4' />}
                label='版本管理'
                onClick={() => setRightSidebarTab('versions')}
              />
              <SidebarIconTab
                active={rightSidebarTab === 'globals'}
                icon={<Globe2 className='size-4' />}
                label='全局变量'
                onClick={() => setRightSidebarTab('globals')}
              />
            </>
          }
        >
          {rightSidebarTab === 'settings' ? (
            <SettingsSidebarPanel
              executionMode={executionMode}
              clusterId={editor.clusterId}
              clusters={clusters}
              detectedVariables={detectedVariables}
              customVariableRows={customVariableRows}
              editingCustomVariableId={editingCustomVariableId}
              customVariableDraft={customVariableDraft}
              onExecutionModeChange={handleExecutionModeChange}
              onClusterChange={(value) =>
                updateEditor('clusterId', value === '__empty__' ? '' : value)
              }
              onStartEditCustomVariableRow={handleStartEditCustomVariableRow}
              onCustomVariableDraftChange={handleCustomVariableDraftChange}
              onSaveCustomVariableRow={handleSaveCustomVariableRow}
              onCancelEditCustomVariableRow={handleCancelEditCustomVariableRow}
              onAddCustomVariableRow={handleAddCustomVariableRow}
              onDeleteCustomVariableRow={handleDeleteCustomVariableRow}
            />
          ) : rightSidebarTab === 'versions' ? (
            <VersionSidebarPanel
              taskId={editor.id}
              currentVersion={editor.currentVersion}
              versions={versions}
              total={versionTotal}
              page={versionPage}
              pageSize={10}
              onPageChange={setVersionPage}
              onPreview={setVersionPreview}
              onCompare={setCompareVersion}
              onRollback={(versionId) => void handleRollbackVersion(versionId)}
              onDelete={(versionId) => void handleDeleteVersion(versionId)}
            />
          ) : (
            <GlobalVariablesSidebarPanel
              variables={globalVariables}
              total={globalVariableTotal}
              page={globalVariablePage}
              pageSize={8}
              onPageChange={setGlobalVariablePage}
              editingId={editingGlobalVariableId}
              onStartEdit={setEditingGlobalVariableId}
              onCancelEdit={() => setEditingGlobalVariableId(null)}
              onSave={handleSaveGlobalVariable}
              onDelete={(id) => void handleDeleteGlobalVariable(id)}
              onCopy={(value) => void copyToClipboard(value, '已复制变量值')}
            />
          )}
        </StudioSidebarShell>

        <Card className='col-span-2 row-start-2 gap-0 overflow-hidden border-border/60 bg-background/75 py-0 shadow-sm'>
          <CardContent className='flex h-full min-h-0 p-0'>
            <div className='flex w-12 shrink-0 flex-col items-center gap-2 border-r border-border/50 bg-muted/10 py-3'>
              <SidebarIconTab
                active={bottomConsoleTab === 'logs'}
                icon={<SquareTerminal className='size-4' />}
                label='日志'
                onClick={() => setBottomConsoleTab('logs')}
              />
              <SidebarIconTab
                active={bottomConsoleTab === 'jobs'}
                icon={<ListTree className='size-4' />}
                label='任务'
                onClick={() => setBottomConsoleTab('jobs')}
              />
              <SidebarIconTab
                active={bottomConsoleTab === 'preview'}
                icon={<Bug className='size-4' />}
                label='预览'
                onClick={() => setBottomConsoleTab('preview')}
              />
            </div>
            <div className='min-h-0 flex-1 p-3'>
              {bottomConsoleTab === 'logs' ? (
                <ConsolePanel
                  job={selectedJob}
                  logsResult={jobLogs}
                  loading={logsLoading}
                  filterMode={logFilterMode}
                  onFilterChange={setLogFilterMode}
                  onExpand={() => {
                    setLogsDialogOpen(true);
                  }}
                />
              ) : bottomConsoleTab === 'jobs' ? (
                <JobRunsPanel
                  jobs={jobs}
                  selectedJobId={selectedJobId}
                  onSelectJob={setSelectedJobId}
                  onRecover={handleRecoverFromHistory}
                  onCancel={handleCancelJob}
                  onViewMetrics={(job) => {
                    setMetricsDialogJob(job);
                    setJobMetricsDialogOpen(true);
                  }}
                />
              ) : (
                <PreviewWorkspacePanel
                  job={previewJob}
                  datasets={previewDatasets}
                  selectedDatasetName={selectedPreviewDataset?.name || ''}
                  previewPage={previewPage}
                  onSelectDataset={(name) => {
                    setPreviewDatasetName(name);
                    setPreviewPage(1);
                  }}
                  onChangePage={setPreviewPage}
                />
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      <Dialog open={validationOpen} onOpenChange={setValidationOpen}>
        <DialogContent className='w-[94vw] max-w-[94vw] sm:max-w-[1240px]'>
          <DialogHeader>
            <DialogTitle>{validationTitle}</DialogTitle>
            <DialogDescription>
              {validationResult?.summary || '当前结果展示配置语法、官方校验与连接测试结论。'}
            </DialogDescription>
          </DialogHeader>
          <ValidationResultPanel result={validationResult} />
        </DialogContent>
      </Dialog>

      <Dialog
        open={dagOpen}
        onOpenChange={(open) => {
          setDagOpen(open);
          if (!open) {
            setDagError(null);
          }
        }}
      >
        <DialogContent className='flex h-[88vh] w-[96vw] max-w-[96vw] flex-col overflow-hidden gap-0 p-0 sm:max-w-[1400px]'>
          <DialogHeader className='px-6 pt-6'>
            <DialogTitle>DAG 预览</DialogTitle>
            <DialogDescription>
              {dagError
                ? '当前文件的 DAG 解析失败，请先修复配置问题后再试。'
                : `当前文件解析出的 DAG 结果，共 ${dagNodes.length} 个节点，${dagEdges.length} 条连线。`}
            </DialogDescription>
          </DialogHeader>
          <div className='min-h-0 flex-1 overflow-auto px-6 pb-6'>
            <div className='space-y-4'>
              {dagError ? (
                <Card className='border-destructive/30 bg-destructive/5'>
                  <CardHeader className='pb-3'>
                    <CardTitle className='text-sm text-destructive'>
                      {dagError.title}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className='space-y-3 text-sm'>
                    <div>{dagError.description}</div>
                    {dagError.raw ? (
                      <pre className='max-h-[360px] overflow-auto rounded-lg border border-destructive/20 bg-background/80 p-3 text-xs text-muted-foreground'>
                        {dagError.raw}
                      </pre>
                    ) : null}
                  </CardContent>
                </Card>
              ) : null}
              {dagWarnings.length > 0 ? (
                <div className='flex flex-wrap gap-2'>
                  {dagWarnings.map((warning, index) => (
                    <Badge key={`${warning}-${index}`} variant='outline'>
                      {warning}
                    </Badge>
                  ))}
                </div>
              ) : null}
              {!dagError && dagWebUIJob ? (
                <WebUiDagPreview job={dagWebUIJob} />
              ) : !dagError ? (
                <Card>
                  <CardHeader className='pb-3'>
                    <CardTitle className='text-sm'>原始 DAG JSON</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <pre className='max-h-[560px] overflow-auto whitespace-pre-wrap break-all text-xs text-muted-foreground'>
                      {JSON.stringify(dagResult, null, 2)}
                    </pre>
                  </CardContent>
                </Card>
              ) : null}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(versionPreview)}
        onOpenChange={(open) => {
          if (!open) {
            setVersionPreview(null);
          }
        }}
      >
        <DialogContent className='max-w-5xl'>
          <DialogHeader>
            <DialogTitle>
              版本预览 {versionPreview ? `v${versionPreview.version}` : ''}
            </DialogTitle>
          </DialogHeader>
          <pre className='max-h-[70vh] overflow-auto rounded-lg border p-4 text-xs text-muted-foreground'>
            {versionPreview ? versionPreview.content_snapshot : '暂无版本内容'}
          </pre>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(compareVersion)}
        onOpenChange={(open) => {
          if (!open) {
            setCompareVersion(null);
          }
        }}
      >
        <DialogContent className='flex h-[88vh] w-[96vw] max-w-[96vw] flex-col overflow-hidden p-0 gap-0 sm:max-w-[1320px]'>
          <DialogHeader>
            <DialogTitle className='px-6 pt-6'>
              版本对比 {compareVersion ? `v${compareVersion.version}` : ''}
            </DialogTitle>
          </DialogHeader>
          <div className='mx-6 flex flex-wrap items-center justify-between gap-3 rounded-t-lg border border-b-0 border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground'>
            <div className='flex items-center gap-2'>
              <GitCompareArrows className='size-4 text-primary' />
              <Badge variant='outline'>Diff</Badge>
              <Badge variant='outline'>只读</Badge>
              <Badge variant='outline'>Side by Side</Badge>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <div className='flex items-center gap-2 rounded-md border border-border/50 bg-background/70 px-2 py-1'>
                <LayoutPanelTop className='size-3.5' />
                <span>
                  v{compareVersion?.version || '-'} /{' '}
                  {compareVersion?.name_snapshot || '历史版本'}
                </span>
              </div>
              <div className='flex items-center gap-2 rounded-md border border-border/50 bg-background/70 px-2 py-1'>
                <Columns2 className='size-3.5' />
                <span>当前编辑 / {editor.name || '未命名文件'}</span>
              </div>
            </div>
          </div>
          <div className='mx-6 mb-6 min-h-0 flex-1 overflow-hidden rounded-lg border border-border/60'>
            <MonacoDiffEditor
              height='100%'
              theme={monacoTheme}
              language={
                (compareVersion?.content_format_snapshot ||
                  editor.contentFormat) === 'json'
                  ? 'json'
                  : 'shell'
              }
              original={compareVersion?.content_snapshot || ''}
              modified={editor.content || ''}
              options={{
                renderSideBySide: true,
                automaticLayout: true,
                readOnly: true,
                minimap: {enabled: false},
                fontSize: 13,
                scrollBeyondLastLine: false,
              }}
            />
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={jobMetricsDialogOpen}
        onOpenChange={setJobMetricsDialogOpen}
      >
        <DialogContent className='max-w-5xl'>
          <DialogHeader>
            <DialogTitle>
              指标详情 {metricsDialogJob ? `#${metricsDialogJob.id}` : ''}
            </DialogTitle>
          </DialogHeader>
          <MetricsDialogContent job={metricsDialogJob} />
        </DialogContent>
      </Dialog>

      <Dialog open={logsDialogOpen} onOpenChange={setLogsDialogOpen}>
        <DialogContent className='flex h-[88vh] w-[96vw] max-w-[96vw] flex-col overflow-hidden sm:max-w-[1400px]'>
          <DialogHeader>
            <DialogTitle>日志查看</DialogTitle>
            <DialogDescription>
              展示当前任务的全部日志；支持搜索、级别过滤与流式追踪。
            </DialogDescription>
          </DialogHeader>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='flex items-center gap-2'>
              <Input
                value={logSearchTerm}
                onChange={(event) => setLogSearchTerm(event.target.value)}
                placeholder='搜索日志关键字'
                className='h-8 w-[320px]'
              />
              <div className='flex items-center gap-1 rounded-md border border-border/50 bg-background px-1 py-1'>
                <Funnel className='ml-1 size-3.5 text-muted-foreground' />
                {(['all', 'warn', 'error'] as LogFilterMode[]).map((mode) => (
                  <button
                    key={mode}
                    type='button'
                    className={cn(
                      'rounded px-2 py-1 text-xs',
                      logFilterMode === mode
                        ? 'bg-primary/10 text-primary'
                        : 'text-muted-foreground',
                    )}
                    onClick={() => setLogFilterMode(mode)}
                  >
                    {mode === 'all' ? '全部' : mode.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            <div className='flex items-center gap-2 text-xs text-muted-foreground'>
              <span>
                {expandedLogsLoading
                  ? '加载全部日志中...'
                  : `共 ${splitLogLines(expandedJobLogs?.logs || jobLogs?.logs || '').length} 行`}
              </span>
            </div>
          </div>
          <VirtualizedLogViewer
            lines={splitLogLines(expandedJobLogs?.logs || jobLogs?.logs || '')}
            height={620}
            emptyText='当前暂无日志输出。'
          />
        </DialogContent>
      </Dialog>

      <Dialog
        open={treeDialog.open}
        onOpenChange={(open) => setTreeDialog((prev) => ({...prev, open}))}
      >
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {treeDialog.action === 'create-folder'
                ? '新建目录'
                : treeDialog.action === 'create-file'
                  ? '新建文件'
                  : treeDialog.action === 'move'
                    ? '移动到'
                    : treeDialog.action === 'delete'
                      ? '删除确认'
                      : '重命名'}
            </DialogTitle>
          </DialogHeader>
          <div className='grid gap-3'>
            {treeDialog.action === 'delete' ? (
              <div className='rounded-lg border border-border/40 bg-muted/10 p-3 text-sm'>
                将删除
                {treeDialog.targetNode?.node_type === 'folder'
                  ? '目录'
                  : '文件'}
                <span className='mx-1 font-medium'>
                  {treeDialog.targetNode?.name}
                </span>
                {treeDialog.targetNode?.node_type === 'folder'
                  ? '及其全部子节点和运行历史。'
                  : '。'}
              </div>
            ) : treeDialog.action === 'move' ? (
              <div className='grid gap-2'>
                <Label>目标目录</Label>
                <div className='max-h-[320px] overflow-auto rounded-md border border-border/60 bg-muted/10 p-2'>
                  <div className='space-y-1'>
                    {moveTargetOptions.map((option) => (
                      <button
                        key={option.value ?? 'root'}
                        type='button'
                        className={cn(
                          'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent',
                          treeDialog.targetParentId === option.value
                            ? 'bg-accent text-accent-foreground'
                            : 'text-muted-foreground',
                        )}
                        style={{paddingLeft: `${8 + option.depth * 12}px`}}
                        onClick={() =>
                          setTreeDialog((prev) => ({
                            ...prev,
                            targetParentId: option.value,
                          }))
                        }
                      >
                        <Folder className='size-4 shrink-0' />
                        <span className='truncate'>{option.label}</span>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            ) : (
              <div className='grid gap-2'>
                <Label htmlFor='tree-dialog-name'>名称</Label>
                <Input
                  id='tree-dialog-name'
                  value={treeDialog.name}
                  onChange={(event) =>
                    setTreeDialog((prev) => ({
                      ...prev,
                      name: event.target.value,
                    }))
                  }
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      void handleTreeDialogSubmit();
                    }
                  }}
                  placeholder='仅支持字母、数字、点、下划线和短横线'
                />
              </div>
            )}
            <div className='flex justify-end gap-2'>
              <Button
                variant='outline'
                onClick={() =>
                  setTreeDialog({
                    open: false,
                    action: null,
                    targetNode: null,
                    name: '',
                    targetParentId: null,
                  })
                }
              >
                取消
              </Button>
              <Button onClick={() => void handleTreeDialogSubmit()}>
                确认
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {treeMenu.open ? (
        <div
          className='fixed z-50 min-w-[160px] rounded-md border bg-popover p-1 shadow-md'
          style={{left: treeMenu.x, top: treeMenu.y}}
        >
          {treeMenu.kind === 'root' || treeMenu.kind === 'folder' ? (
            <>
              <button
                type='button'
                className='flex w-full items-center rounded-sm px-2 py-1.5 text-sm hover:bg-accent'
                onClick={() => openTreeDialog('create-folder', treeMenu.node)}
              >
                新建目录
              </button>
              {treeMenu.kind === 'folder' ? (
                <button
                  type='button'
                  className='flex w-full items-center rounded-sm px-2 py-1.5 text-sm hover:bg-accent'
                  onClick={() => openTreeDialog('create-file', treeMenu.node)}
                >
                  新建文件
                </button>
              ) : null}
            </>
          ) : null}
          {treeMenu.kind !== 'root' ? (
            <button
              type='button'
              className='flex w-full items-center rounded-sm px-2 py-1.5 text-sm hover:bg-accent'
              onClick={() =>
                openTreeDialog(
                  'rename',
                  treeMenu.node,
                  treeMenu.node?.name || '',
                )
              }
            >
              重命名
            </button>
          ) : null}
          {treeMenu.kind !== 'root' ? (
            <button
              type='button'
              className='flex w-full items-center rounded-sm px-2 py-1.5 text-sm hover:bg-accent'
              onClick={() => openTreeDialog('move', treeMenu.node)}
            >
              移动到
            </button>
          ) : null}
          {treeMenu.kind !== 'root' ? (
            <button
              type='button'
              className='flex w-full items-center rounded-sm px-2 py-1.5 text-sm text-destructive hover:bg-accent'
              onClick={() => openTreeDialog('delete', treeMenu.node)}
            >
              <Trash2 className='mr-2 size-4' />
              删除
            </button>
          ) : null}
          <button
            type='button'
            className='flex w-full items-center rounded-sm px-2 py-1.5 text-sm hover:bg-accent'
            onClick={() => {
              setTreeMenu((prev) => ({...prev, open: false}));
              void loadWorkspace(selectedNodeId);
            }}
          >
            <RefreshCw className='mr-2 size-4' />
            刷新
          </button>
        </div>
      ) : null}
    </div>
  );
}

function TreeView({
  nodes,
  selectedNodeId,
  selectedFolderId,
  expandedFolderIds,
  onSelect,
  onContextMenu,
  depth = 0,
}: {
  nodes: SyncTaskTreeNode[];
  selectedNodeId: number | null;
  selectedFolderId: number | null;
  expandedFolderIds: number[];
  onSelect: (node: SyncTaskTreeNode) => void;
  onContextMenu: (
    event: MouseEvent,
    kind: 'folder' | 'file',
    node: SyncTaskTreeNode,
  ) => void;
  depth?: number;
}) {
  return (
    <div className='space-y-0 py-0.5'>
      {nodes.map((node) => {
        const selected =
          node.id === selectedNodeId || node.id === selectedFolderId;
        const isExpanded = expandedFolderIds.includes(node.id);
        const hasChildren = Boolean(node.children && node.children.length > 0);
        return (
          <div key={node.id}>
            <button
              type='button'
              className={`flex w-full items-center justify-between rounded-sm border border-transparent px-2 py-1 text-left text-xs transition hover:bg-muted/70 ${selected ? 'border-primary/20 bg-primary/10 text-primary' : ''}`}
              style={{paddingLeft: `${depth * 12 + 6}px`}}
              onClick={() => onSelect(node)}
              onContextMenu={(event) =>
                onContextMenu(event, node.node_type, node)
              }
            >
              <span className='flex min-w-0 items-center gap-2'>
                {node.node_type === 'folder' ? (
                  <>
                    {hasChildren ? (
                      isExpanded ? (
                        <ChevronDown className='size-3.5 shrink-0' />
                      ) : (
                        <ChevronRight className='size-3.5 shrink-0' />
                      )
                    ) : (
                      <span className='inline-block size-3.5 shrink-0' />
                    )}
                    <Folder className='size-3.5 shrink-0' />
                  </>
                ) : (
                  <>
                    <span className='inline-block size-3.5 shrink-0' />
                    <FileCode2 className='size-3.5 shrink-0' />
                  </>
                )}
                <span className='truncate'>{node.name}</span>
              </span>
              {node.node_type === 'file' ? (
                <Badge
                  variant='outline'
                  className='h-5 rounded-sm px-1.5 text-[10px]'
                >
                  {node.current_version > 0
                    ? `v${node.current_version}`
                    : 'draft'}
                </Badge>
              ) : null}
            </button>
            {hasChildren && isExpanded ? (
              <TreeView
                nodes={node.children || []}
                selectedNodeId={selectedNodeId}
                selectedFolderId={selectedFolderId}
                expandedFolderIds={expandedFolderIds}
                onSelect={onSelect}
                onContextMenu={onContextMenu}
                depth={depth + 1}
              />
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function SidebarIconTab({
  active,
  icon,
  label,
  onClick,
}: {
  active: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type='button'
          className={cn(
            'flex h-9 w-9 items-center justify-center rounded-md border transition-colors',
            active
              ? 'border-primary/40 bg-primary/10 text-primary'
              : 'border-transparent text-muted-foreground hover:bg-muted hover:text-foreground',
          )}
          onClick={onClick}
        >
          {icon}
        </button>
      </TooltipTrigger>
      <TooltipContent side='left'>{label}</TooltipContent>
    </Tooltip>
  );
}

function StudioSidebarShell({
  children,
  rail,
  className,
}: {
  children: ReactNode;
  rail: ReactNode;
  className?: string;
}) {
  return (
    <Card
      className={cn(
        'row-start-1 gap-0 overflow-hidden border-border/60 bg-background/75 py-0 shadow-sm',
        className,
      )}
    >
      <CardContent className='grid h-full min-h-0 min-w-0 grid-cols-[minmax(0,1fr)_40px] p-0'>
        <div className='min-h-0 min-w-0 overflow-hidden'>
          <ScrollArea className='h-full'>
            <div className='min-w-0 p-3'>{children}</div>
          </ScrollArea>
        </div>
        <div className='flex min-h-0 w-10 shrink-0 flex-col items-center gap-2 border-l border-border/50 bg-muted/10 py-3'>
          {rail}
        </div>
      </CardContent>
    </Card>
  );
}

function SettingsSidebarPanel({
  executionMode,
  clusterId,
  clusters,
  detectedVariables,
  customVariableRows,
  editingCustomVariableId,
  customVariableDraft,
  onExecutionModeChange,
  onClusterChange,
  onStartEditCustomVariableRow,
  onCustomVariableDraftChange,
  onSaveCustomVariableRow,
  onCancelEditCustomVariableRow,
  onAddCustomVariableRow,
  onDeleteCustomVariableRow,
}: {
  executionMode: ExecutionMode;
  clusterId: string;
  clusters: ClusterInfo[];
  detectedVariables: string[];
  customVariableRows: VariableRow[];
  editingCustomVariableId: string | null;
  customVariableDraft: VariableDraft;
  onExecutionModeChange: (value: ExecutionMode) => void;
  onClusterChange: (value: string) => void;
  onStartEditCustomVariableRow: (rowId: string) => void;
  onCustomVariableDraftChange: (field: keyof VariableDraft, value: string) => void;
  onSaveCustomVariableRow: (rowId: string) => void;
  onCancelEditCustomVariableRow: () => void;
  onAddCustomVariableRow: () => void;
  onDeleteCustomVariableRow: (rowId: string) => void;
}) {
  return (
    <div className='mx-auto min-w-0 max-w-[236px] space-y-4'>
      <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
        <div className='mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground'>
          设置
        </div>
        <div className='space-y-2'>
          <Label className='text-xs'>执行模式</Label>
          <Select
            value={executionMode}
            onValueChange={(value) =>
              onExecutionModeChange(value as ExecutionMode)
            }
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent className='w-[var(--radix-select-trigger-width)] min-w-0'>
              <SelectItem value='cluster'>Cluster</SelectItem>
              <SelectItem value='local'>Local</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {executionMode === 'cluster' ? (
        <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
          <Label className='mb-2 block text-xs'>Zeta 集群</Label>
          <Select
            value={clusterId || '__empty__'}
            onValueChange={onClusterChange}
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder='请选择集群' />
            </SelectTrigger>
            <SelectContent className='w-[var(--radix-select-trigger-width)] min-w-0'>
              <SelectItem value='__empty__'>未选择</SelectItem>
              {clusters.map((cluster) => (
                <SelectItem key={cluster.id} value={String(cluster.id)}>
                  {cluster.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : null}

      <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
        <div className='mb-2 flex items-center justify-between gap-2'>
          <Label className='block text-xs'>自定义变量</Label>
          <Button
            type='button'
            size='sm'
            variant='outline'
            className='h-7 px-2 text-xs'
            onClick={onAddCustomVariableRow}
          >
            <Plus className='mr-1 size-3.5' />
            新增
          </Button>
        </div>
        <div className='space-y-2'>
          {customVariableRows.map((row) => {
            const isEditing = editingCustomVariableId === row.id;
            return (
              <div
                key={row.id}
                className='rounded-lg border border-border/50 bg-background/70 p-3'
              >
                {isEditing ? (
                  <div className='space-y-3'>
                    <div className='space-y-1.5'>
                      <Label className='text-[11px] text-muted-foreground'>key</Label>
                      <Input
                        value={customVariableDraft.key}
                        onChange={(event) =>
                          onCustomVariableDraftChange('key', event.target.value)
                        }
                        className='h-8 text-xs'
                        placeholder='key'
                      />
                    </div>
                    <div className='space-y-1.5'>
                      <Label className='text-[11px] text-muted-foreground'>value</Label>
                      <Input
                        value={customVariableDraft.value}
                        onChange={(event) =>
                          onCustomVariableDraftChange('value', event.target.value)
                        }
                        className='h-8 text-xs'
                        placeholder='value'
                      />
                    </div>
                    <div className='grid grid-cols-2 gap-2'>
                      <Button
                        type='button'
                        size='sm'
                        variant='outline'
                        className='h-8 text-xs'
                        onClick={onCancelEditCustomVariableRow}
                      >
                        取消
                      </Button>
                      <Button
                        type='button'
                        size='sm'
                        className='h-8 text-xs'
                        onClick={() => onSaveCustomVariableRow(row.id)}
                      >
                        保存
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className='flex items-start justify-between gap-2'>
                    <div className='min-w-0 flex-1 space-y-2'>
                      <div className='flex min-w-0 items-center gap-2'>
                        <Badge variant='outline' className='shrink-0'>
                          {row.key || '未命名'}
                        </Badge>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className='min-w-0 flex-1 truncate text-xs text-muted-foreground'>
                              {row.value || '-'}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent className='max-w-[320px] break-all'>
                            {row.value || '-'}
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </div>
                    <div className='flex shrink-0 items-center gap-1'>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type='button'
                            size='icon'
                            variant='ghost'
                            className='size-8'
                            onClick={() => onStartEditCustomVariableRow(row.id)}
                          >
                            <Pencil className='size-4' />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>编辑</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type='button'
                            size='icon'
                            variant='ghost'
                            className='size-8 text-destructive hover:text-destructive'
                            onClick={() => onDeleteCustomVariableRow(row.id)}
                          >
                            <Trash2 className='size-4' />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>删除</TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
        <Label className='mb-2 block text-xs'>脚本识别变量</Label>
        <div className='flex flex-wrap gap-2'>
          {detectedVariables.length > 0 ? (
            detectedVariables.map((variable) => (
              <Badge key={variable} variant='outline'>
                {`{{${variable}}}`}
              </Badge>
            ))
          ) : (
            <span className='text-xs text-muted-foreground'>
              当前脚本未识别到变量占位符。
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

function GlobalVariablesSidebarPanel({
  variables,
  total,
  page,
  pageSize,
  onPageChange,
  editingId,
  onStartEdit,
  onCancelEdit,
  onSave,
  onDelete,
  onCopy,
}: {
  variables: SyncGlobalVariable[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  editingId: number | null;
  onStartEdit: (id: number | null) => void;
  onCancelEdit: () => void;
  onSave: (
    item: SyncGlobalVariable | null,
    payload: {key: string; value: string; description: string},
  ) => void;
  onDelete: (id: number) => void;
  onCopy: (value: string) => void;
}) {
  const [draft, setDraft] = useState<{
    key: string;
    value: string;
    description: string;
  }>({key: '', value: '', description: ''});

  useEffect(() => {
    if (editingId === null) {
      setDraft({key: '', value: '', description: ''});
      return;
    }
    const target = variables.find((item) => item.id === editingId);
    if (!target) {
      return;
    }
    setDraft({
      key: target.key,
      value: target.value,
      description: target.description || '',
    });
  }, [editingId, variables]);

  return (
    <div className='mx-auto min-w-0 max-w-[236px] space-y-3'>
      <div className='sticky top-0 z-10 min-w-0 rounded-lg border border-border/50 bg-background/95 p-3 backdrop-blur supports-[backdrop-filter]:bg-background/85'>
        <div className='mb-3 flex min-w-0 flex-col gap-2'>
          <div className='min-w-0 space-y-1'>
            <div className='text-[11px] uppercase tracking-wide text-muted-foreground'>
              全局变量
            </div>
            <div className='text-xs text-muted-foreground'>
              已声明的 {'{{var}}'} 会在运行前自动替换。
            </div>
          </div>
          <Button
            size='sm'
            variant='outline'
            className='h-8 w-full text-xs'
            onClick={() => onStartEdit(null)}
          >
            <Plus className='mr-1 size-3.5' />
            新建
          </Button>
        </div>
        <div className='min-w-0 grid gap-2'>
          <div className='grid min-w-0 gap-2'>
            <Input
              value={draft.key}
              onChange={(event) =>
                setDraft((current) => ({...current, key: event.target.value}))
              }
              className='h-8 min-w-0 text-xs'
              placeholder='key'
            />
            <Input
              value={draft.value}
              onChange={(event) =>
                setDraft((current) => ({...current, value: event.target.value}))
              }
              className='h-8 min-w-0 text-xs'
              placeholder='value'
            />
          </div>
          <Input
            value={draft.description}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                description: event.target.value,
              }))
            }
            className='h-8 text-xs'
            placeholder='描述（可选）'
          />
          <div className='grid min-w-0 grid-cols-1 gap-2'>
            <Button
              size='sm'
              variant='outline'
              className='h-8 min-w-0 text-xs'
              onClick={onCancelEdit}
            >
              取消
            </Button>
            <Button
              size='sm'
              className='h-8 min-w-0 text-xs'
              onClick={() =>
                onSave(
                  editingId === null
                    ? null
                    : variables.find((item) => item.id === editingId) || null,
                  draft,
                )
              }
            >
              保存
            </Button>
          </div>
        </div>
      </div>

      <div className='space-y-2 pb-2'>
        {variables.length === 0 ? (
          <div className='text-sm text-muted-foreground'>暂无全局变量。</div>
        ) : (
          variables.map((item) => (
            <div
              key={item.id}
              className='rounded-lg border border-border/50 bg-background/70 p-3'
            >
              <div className='flex items-start justify-between gap-2'>
                <div className='min-w-0 space-y-2'>
                  <div className='flex items-center gap-2'>
                    <Badge variant='outline'>{item.key}</Badge>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <button
                          type='button'
                          className='max-w-[150px] truncate text-left text-xs text-muted-foreground'
                        >
                          {item.value}
                        </button>
                      </TooltipTrigger>
                      <TooltipContent className='max-w-[320px] break-all'>
                        {item.value || '-'}
                      </TooltipContent>
                    </Tooltip>
                  </div>
                  <div className='text-xs text-muted-foreground'>
                    {item.description || '无描述'}
                  </div>
                </div>
                <div className='flex items-center gap-1'>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        size='icon'
                        variant='ghost'
                        className='size-8'
                        onClick={() => onCopy(item.value)}
                      >
                        <Copy className='size-4' />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>复制值</TooltipContent>
                  </Tooltip>
                  <DropdownMenu>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <DropdownMenuTrigger asChild>
                          <Button
                            size='icon'
                            variant='ghost'
                            className='size-8'
                          >
                            <MoreHorizontal className='size-4' />
                          </Button>
                        </DropdownMenuTrigger>
                      </TooltipTrigger>
                      <TooltipContent>更多操作</TooltipContent>
                    </Tooltip>
                    <DropdownMenuContent align='end'>
                      <DropdownMenuItem onClick={() => onStartEdit(item.id)}>
                        <Pencil className='mr-2 size-4' />
                        编辑
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        className='text-destructive focus:text-destructive'
                        onClick={() => onDelete(item.id)}
                      >
                        <Trash2 className='mr-2 size-4' />
                        删除
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            </div>
          ))
        )}
      </div>
      <SimplePagination
        total={total}
        page={page}
        pageSize={pageSize}
        onPageChange={onPageChange}
      />
    </div>
  );
}

function VersionSidebarPanel({
  taskId,
  currentVersion,
  versions,
  total,
  page,
  pageSize,
  onPageChange,
  onPreview,
  onCompare,
  onRollback,
  onDelete,
}: {
  taskId?: number;
  currentVersion: number;
  versions: SyncTaskVersion[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPreview: (version: SyncTaskVersion) => void;
  onCompare: (version: SyncTaskVersion) => void;
  onRollback: (versionId: number) => void;
  onDelete: (versionId: number) => void;
}) {
  if (!taskId) {
    return (
      <div className='text-sm text-muted-foreground'>
        请选择文件后查看版本历史。
      </div>
    );
  }
  return (
    <div className='space-y-3'>
      <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
        <div className='flex items-center justify-between gap-2'>
          <div>
            <div className='text-[11px] uppercase tracking-wide text-muted-foreground'>
              版本管理
            </div>
            <div className='mt-1 text-lg font-semibold'>v{currentVersion}</div>
          </div>
          <Badge variant='outline'>{versions.length} 条</Badge>
        </div>
        <p className='mt-2 text-xs leading-5 text-muted-foreground'>
          每次点击保存都会生成一个新版本；预览、对比会使用弹窗展开，回滚与删除直接作用于该版本记录。
        </p>
      </div>
      <div className='space-y-2'>
        {versions.length > 0 ? (
          versions.map((version) => (
            <div
              key={version.id}
              className='rounded-lg border border-border/50 bg-background/70 p-3'
            >
              <div className='flex items-center justify-between gap-2'>
                <div>
                  <div className='text-sm font-medium'>v{version.version}</div>
                  <div className='text-[11px] text-muted-foreground'>
                    {new Date(version.created_at).toLocaleString()}
                  </div>
                </div>
                <Badge
                  variant={
                    version.version === currentVersion ? 'secondary' : 'outline'
                  }
                >
                  #{version.id}
                </Badge>
              </div>
              <div className='mt-3 grid grid-cols-2 gap-2'>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 text-xs'
                  onClick={() => onPreview(version)}
                >
                  预览
                </Button>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 text-xs'
                  onClick={() => onCompare(version)}
                >
                  对比
                </Button>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 text-xs'
                  onClick={() => onRollback(version.id)}
                >
                  回滚
                </Button>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 text-xs'
                  onClick={() => onDelete(version.id)}
                >
                  删除
                </Button>
              </div>
            </div>
          ))
        ) : (
          <div className='text-sm text-muted-foreground'>暂无版本历史。</div>
        )}
      </div>
      <SimplePagination
        total={total}
        page={page}
        pageSize={pageSize}
        onPageChange={onPageChange}
      />
    </div>
  );
}

function SimplePagination({
  total,
  page,
  pageSize,
  onPageChange,
}: {
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)));
  if (total <= pageSize) {
    return null;
  }
  return (
    <div className='flex items-center justify-between gap-2 rounded-lg border border-border/50 bg-muted/10 px-3 py-2 text-xs text-muted-foreground'>
      <span>
        第 {page} / {totalPages} 页，共 {total} 条
      </span>
      <div className='flex items-center gap-2'>
        <Button
          size='sm'
          variant='outline'
          className='h-7 px-2 text-xs'
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          上一页
        </Button>
        <Button
          size='sm'
          variant='outline'
          className='h-7 px-2 text-xs'
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          下一页
        </Button>
      </div>
    </div>
  );
}

function ConsolePanel({
  job,
  logsResult,
  loading,
  filterMode,
  onFilterChange,
  onExpand,
}: {
  job: SyncJobInstance | null;
  logsResult: SyncJobLogsResult | null;
  loading: boolean;
  filterMode: LogFilterMode;
  onFilterChange: (mode: LogFilterMode) => void;
  onExpand: () => void;
}) {
  if (!job) {
    return <div className='text-sm text-muted-foreground'>暂无日志输出。</div>;
  }
  const renderedLines = buildDisplayLogLines(logsResult?.logs || '', 800);
  return (
    <div className='flex h-full min-h-0 min-w-0 flex-col gap-2'>
      <div className='flex flex-wrap items-center gap-2 rounded-lg border border-border/50 bg-background/70 px-3 py-2 text-xs'>
        <Badge variant='outline'>#{job.id}</Badge>
        <Badge variant='outline'>{job.run_type}</Badge>
        <Badge
          variant='outline'
          className={cn(
            'rounded-sm border px-2 py-0.5 text-[11px]',
            getJobStatusBadgeClass(job.status),
          )}
        >
          {getJobStatusLabel(job.status)}
        </Badge>
        <Badge variant='outline'>
          {getEngineAPIMode(job) === 'v1'
            ? 'Legacy REST V1'
            : submitSpecExecutionMode(job.submit_spec) === 'local'
              ? 'Local Agent'
              : 'REST V2'}
        </Badge>
        <span className='min-w-0 flex-1 truncate text-muted-foreground'>
          {getEngineEndpointLabel(job)}
        </span>
        <span className='text-muted-foreground'>
          {loading
            ? '加载中...'
            : logsResult?.updated_at
              ? new Date(logsResult.updated_at).toLocaleTimeString()
              : '-'}
        </span>
      </div>
      <div className='flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-lg border border-border/50 bg-background/70'>
        <div className='sticky top-0 z-10 shrink-0 border-b border-border/50 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/85'>
          <div className='grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-3 py-2 text-xs text-muted-foreground'>
            <div className='flex min-w-0 items-center gap-2 overflow-hidden'>
              <span className='shrink-0'>实时日志</span>
              {job.error_message ? (
                <Badge
                  className='rounded-sm border-red-500/30 bg-red-500/10 text-[10px] text-red-600 dark:text-red-400'
                  variant='outline'
                >
                  存在错误
                </Badge>
              ) : null}
            </div>
            <div className='flex items-center justify-self-end gap-2 whitespace-nowrap'>
              <div className='flex items-center gap-1 rounded-md border border-border/50 bg-background px-1 py-1'>
                {(['all', 'warn', 'error'] as LogFilterMode[]).map((mode) => (
                  <button
                    key={mode}
                    type='button'
                    className={cn(
                      'rounded px-2 py-0.5 text-[11px]',
                      filterMode === mode
                        ? 'bg-primary/10 text-primary'
                        : 'text-muted-foreground',
                    )}
                    onClick={() => onFilterChange(mode)}
                  >
                    {mode === 'all' ? '全部' : mode.toUpperCase()}
                  </button>
                ))}
              </div>
              <Button
                size='sm'
                variant='ghost'
                className='h-7 px-1.5 text-xs'
                onClick={onExpand}
              >
                <Maximize2 className='mr-1 size-3.5' />
                展开
              </Button>
            </div>
          </div>
          <div className='grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-t border-border/50 px-3 py-2 text-[11px] text-muted-foreground'>
            <div className='flex min-w-0 items-center gap-3 overflow-hidden'>
              <span className='truncate'>
                Platform: {job.platform_job_id || '-'}
              </span>
              <span className='truncate'>
                Engine: {job.engine_job_id || '-'}
              </span>
            </div>
            <span className='justify-self-end whitespace-nowrap'>
              主视图聚焦最近日志，展开后可查看完整结果
            </span>
          </div>
        </div>
        <div className='min-h-0 min-w-0 flex-1 overflow-auto p-3 font-mono text-xs'>
          {renderedLines.length > 0 ? (
            renderedLines.map((line, index) => (
              <div
                key={`${index}-${line.slice(0, 24)}`}
                className={cn(
                  'max-w-full whitespace-pre-wrap break-all',
                  getLogLineClass(line),
                )}
              >
                {line}
              </div>
            ))
          ) : (
            <div className='text-muted-foreground'>当前暂无日志输出。</div>
          )}
        </div>
      </div>
    </div>
  );
}

function JobRunsPanel({
  jobs,
  selectedJobId,
  onSelectJob,
  onRecover,
  onCancel,
  onViewMetrics,
}: {
  jobs: SyncJobInstance[];
  selectedJobId: number | null;
  onSelectJob: (jobId: number) => void;
  onRecover: (jobId: number) => void;
  onCancel: (jobId: number) => void;
  onViewMetrics: (job: SyncJobInstance) => void;
}) {
  if (jobs.length === 0) {
    return (
      <div className='text-sm text-muted-foreground'>暂无任务运行记录。</div>
    );
  }
  return (
    <div className='h-full overflow-auto rounded-lg border border-border/50 bg-background/70'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>任务</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>通道</TableHead>
            <TableHead>指标</TableHead>
            <TableHead className='text-right'>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.map((job) => {
            const summary = extractJobMetricSummary(job);
            return (
              <TableRow
                key={job.id}
                className={cn(selectedJobId === job.id ? 'bg-primary/5' : '')}
                onClick={() => onSelectJob(job.id)}
              >
                <TableCell>
                  <div className='font-medium'>#{job.id}</div>
                  <div className='text-xs text-muted-foreground'>
                    {job.platform_job_id || '-'}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge
                    variant='outline'
                    className={cn(
                      'rounded-sm border px-2 py-0.5 text-[11px]',
                      getJobStatusBadgeClass(job.status),
                    )}
                  >
                    {getJobStatusLabel(job.status)}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant='outline' className='rounded-sm text-[11px]'>
                    {submitSpecExecutionMode(job.submit_spec) === 'local'
                      ? 'Local Agent'
                      : getEngineAPIMode(job) === 'v1'
                        ? 'Legacy REST V1'
                        : 'REST V2'}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div className='space-y-0.5 text-xs'>
                    <div>读 {formatMetricValue(summary.readCount)}</div>
                    <div>写 {formatMetricValue(summary.writeCount)}</div>
                    <div>
                      均速 {formatMetricValue(summary.averageSpeed, 1)}/s
                    </div>
                  </div>
                </TableCell>
                <TableCell className='text-right'>
                  <div className='flex justify-end gap-2'>
                    <Button
                      size='icon'
                      variant='outline'
                      className='size-8'
                      onClick={(event) => {
                        event.stopPropagation();
                        onViewMetrics(job);
                      }}
                    >
                      <BarChart3 className='size-4' />
                    </Button>
                    {job.run_type !== 'preview' ? (
                      <Button
                        size='sm'
                        variant='outline'
                        className='h-8 text-xs'
                        disabled={
                          submitSpecExecutionMode(job.submit_spec) === 'local'
                        }
                        onClick={(event) => {
                          event.stopPropagation();
                          onRecover(job.id);
                        }}
                      >
                        恢复
                      </Button>
                    ) : null}
                    {job.status === 'pending' || job.status === 'running' ? (
                      <Button
                        size='sm'
                        variant='outline'
                        className='h-8 text-xs'
                        onClick={(event) => {
                          event.stopPropagation();
                          onCancel(job.id);
                        }}
                      >
                        取消
                      </Button>
                    ) : null}
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

function PreviewWorkspacePanel({
  job,
  datasets,
  selectedDatasetName,
  previewPage,
  onSelectDataset,
  onChangePage,
}: {
  job: SyncJobInstance | null;
  datasets: SyncPreviewDataset[];
  selectedDatasetName: string;
  previewPage: number;
  onSelectDataset: (name: string) => void;
  onChangePage: (page: number) => void;
}) {
  if (!job) {
    return <div className='text-sm text-muted-foreground'>暂无预览任务。</div>;
  }
  const activeDataset =
    datasets.find((dataset) => dataset.name === selectedDatasetName) ||
    datasets[0] ||
    null;
  const columns = activeDataset?.columns || [];
  const rows = (activeDataset?.rows || []) as Array<Record<string, unknown>>;
  const pageSize = Math.max(activeDataset?.page_size || 20, 1);
  const total = Math.max(activeDataset?.total || rows.length, rows.length);
  const totalPages = Math.max(Math.ceil(total / pageSize), 1);
  const currentPage = Math.min(Math.max(previewPage, 1), totalPages);
  const pageRows = rows.slice(
    (currentPage - 1) * pageSize,
    currentPage * pageSize,
  );
  return (
    <div className='grid h-full min-h-0 gap-3 lg:grid-cols-[220px_minmax(0,1fr)]'>
      <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
        <div className='mb-3 text-sm font-medium'>Catalog</div>
        <div className='space-y-2'>
          {datasets.length > 0 ? (
            datasets.map((dataset) => (
              <button
                key={dataset.name}
                type='button'
                className={cn(
                  'flex w-full items-center justify-between rounded-md border px-3 py-2 text-left text-sm',
                  dataset.name === activeDataset?.name
                    ? 'border-primary/30 bg-primary/5'
                    : 'border-border/50 bg-background/60',
                )}
                onClick={() => onSelectDataset(dataset.name)}
              >
                <span className='truncate'>{dataset.name}</span>
                <Badge variant='outline'>{(dataset.rows || []).length}</Badge>
              </button>
            ))
          ) : (
            <div className='rounded-md border border-border/50 bg-background/60 px-3 py-2 text-sm text-muted-foreground'>
              暂无数据集
            </div>
          )}
          <div className='rounded-md border border-border/50 bg-muted/20 p-3 text-xs text-muted-foreground'>
            <div>Platform JobID: {job.platform_job_id || '-'}</div>
            <div className='mt-2'>Columns: {columns.length}</div>
            <div className='mt-2 break-all whitespace-pre-wrap'>
              {JSON.stringify(activeDataset?.catalog || {}, null, 2)}
            </div>
          </div>
        </div>
      </div>
      <div className='flex min-h-0 flex-col rounded-lg border border-border/50 bg-background/70'>
        <div className='flex items-center justify-between border-b border-border/50 px-3 py-2 text-sm font-medium'>
          <span>数据表格</span>
          <div className='flex items-center gap-2 text-xs text-muted-foreground'>
            <Button
              size='sm'
              variant='outline'
              className='h-7 px-2 text-xs'
              onClick={() => onChangePage(Math.max(currentPage - 1, 1))}
              disabled={currentPage <= 1}
            >
              上一页
            </Button>
            <span>
              {currentPage} / {totalPages}
            </span>
            <Button
              size='sm'
              variant='outline'
              className='h-7 px-2 text-xs'
              onClick={() =>
                onChangePage(Math.min(currentPage + 1, totalPages))
              }
              disabled={currentPage >= totalPages}
            >
              下一页
            </Button>
          </div>
        </div>
        {columns.length > 0 ? (
          <div className='min-h-0 flex-1 overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  {columns.map((column) => (
                    <TableHead key={column}>{column}</TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {pageRows.length > 0 ? (
                  pageRows.map((row, index) => (
                    <TableRow key={index}>
                      {columns.map((column) => (
                        <TableCell key={`${index}-${column}`}>
                          {formatCellValue(row[column])}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell
                      colSpan={columns.length}
                      className='text-center text-muted-foreground'
                    >
                      当前没有回流数据，先展示表头结构。
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        ) : (
          <pre className='min-h-0 flex-1 overflow-auto p-3 text-xs text-muted-foreground'>
            {JSON.stringify(job.result_preview || {}, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}

function ValidationResultPanel({result}: {result: SyncValidateResult | null}) {
  if (!result) {
    return <div className='text-sm text-muted-foreground'>暂无校验结果。</div>;
  }
  const checks = result.checks || [];
  return (
    <div className='grid max-h-[70vh] gap-4 overflow-auto pr-1 lg:grid-cols-[minmax(0,1fr)_360px]'>
      <div className='space-y-4'>
        <div className='rounded-lg border border-border/60 bg-background/80 p-4'>
          <div className='flex items-center justify-between gap-3'>
            <div className='text-sm font-medium'>结论</div>
            <Badge
              variant='outline'
              className={cn(
                'rounded-sm border px-2 py-0.5 text-[11px]',
                result.valid
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300'
                  : 'border-red-200 bg-red-50 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300',
              )}
            >
              {result.valid ? '通过' : '未通过'}
            </Badge>
          </div>
          <div className='mt-2 text-sm text-muted-foreground'>
            {result.summary}
          </div>
        </div>

        <div className='grid gap-4 lg:grid-cols-2'>
          <div className='rounded-lg border border-border/60 bg-background/80 p-4'>
            <div className='mb-3 text-sm font-medium'>错误</div>
            {result.errors.length > 0 ? (
              <div className='space-y-2'>
                {result.errors.map((item, index) => (
                  <div
                    key={`${item}-${index}`}
                    className='rounded-md border border-destructive/20 bg-destructive/5 px-3 py-2 text-sm text-destructive'
                  >
                    {item}
                  </div>
                ))}
              </div>
            ) : (
              <div className='text-sm text-muted-foreground'>暂无错误。</div>
            )}
          </div>
          <div className='rounded-lg border border-border/60 bg-background/80 p-4'>
            <div className='mb-3 text-sm font-medium'>警告</div>
            {result.warnings.length > 0 ? (
              <div className='space-y-2'>
                {result.warnings.map((item, index) => (
                  <div
                    key={`${item}-${index}`}
                    className='rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300'
                  >
                    {item}
                  </div>
                ))}
              </div>
            ) : (
              <div className='text-sm text-muted-foreground'>暂无警告。</div>
            )}
          </div>
        </div>
      </div>

      <div className='rounded-lg border border-border/60 bg-background/80 p-4'>
        <div className='mb-3 text-sm font-medium'>连接检查</div>
        {checks.length > 0 ? (
          <div className='space-y-3'>
            {checks.map((check, index) => (
              <div
                key={`${check.node_id}-${check.connector_type}-${index}`}
                className='rounded-lg border border-border/50 bg-muted/15 p-3'
              >
                <div className='flex items-start justify-between gap-3'>
                  <div className='space-y-1'>
                    <div className='text-sm font-medium'>
                      {check.connector_type}
                    </div>
                    <div className='text-xs text-muted-foreground'>
                      {check.node_id}
                    </div>
                  </div>
                  <Badge
                    variant='outline'
                    className={cn(
                      'rounded-sm capitalize',
                      check.status === 'success'
                        ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300'
                        : check.status === 'failed'
                          ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300'
                          : 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-500/30 dark:bg-slate-500/10 dark:text-slate-300',
                    )}
                  >
                    {check.status}
                  </Badge>
                </div>
                {check.target ? (
                  <div className='mt-2 break-all rounded-md bg-muted/30 px-2 py-1 font-mono text-[11px] text-muted-foreground'>
                    {check.target}
                  </div>
                ) : null}
                <div className='mt-2 text-sm text-muted-foreground'>
                  {check.message}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className='text-sm text-muted-foreground'>
            当前没有可展示的连接检查结果。
          </div>
        )}
      </div>
    </div>
  );
}

function MetricsDialogContent({job}: {job: SyncJobInstance | null}) {
  const metricGroups = buildMetricGroups(job?.result_preview?.metrics);
  if (!job) {
    return <div className='text-sm text-muted-foreground'>暂无指标数据。</div>;
  }
  if (metricGroups.length === 0) {
    return (
      <div className='text-sm text-muted-foreground'>
        当前任务暂无指标输出。
      </div>
    );
  }
  return (
    <div className='grid max-h-[70vh] gap-4 overflow-auto pr-1 lg:grid-cols-2'>
      {metricGroups.map((group) => (
        <div
          key={group.key}
          className='overflow-hidden rounded-lg border border-border/60 bg-background/80'
        >
          <div className='border-b border-border/50 bg-muted/20 px-3 py-2 text-sm font-medium'>
            {group.title}
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>指标</TableHead>
                <TableHead>值</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {group.items.map((item) => (
                <TableRow key={item.key}>
                  <TableCell className='font-mono text-xs'>
                    {item.key}
                  </TableCell>
                  <TableCell className='text-xs'>
                    {formatMetricDisplayValue(item.value)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ))}
    </div>
  );
}

function VirtualizedLogViewer({
  lines,
  height,
  emptyText,
}: {
  lines: string[];
  height: number;
  emptyText: string;
}) {
  const rowHeight = 20;
  const overscan = 24;
  const [scrollTop, setScrollTop] = useState(0);
  const startIndex = Math.max(Math.floor(scrollTop / rowHeight) - overscan, 0);
  const visibleCount = Math.ceil(height / rowHeight) + overscan * 2;
  const endIndex = Math.min(startIndex + visibleCount, lines.length);
  const visibleLines = lines.slice(startIndex, endIndex);
  if (lines.length === 0) {
    return (
      <div className='rounded-lg border border-border/60 bg-background/80 p-4 text-sm text-muted-foreground'>
        {emptyText}
      </div>
    );
  }
  return (
    <div
      className='overflow-auto rounded-lg border border-border/60 bg-background/80 p-0 font-mono text-xs'
      style={{height}}
      onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
    >
      <div style={{height: lines.length * rowHeight, position: 'relative'}}>
        <div
          style={{
            position: 'absolute',
            top: startIndex * rowHeight,
            left: 0,
            right: 0,
          }}
          className='px-4 py-3'
        >
          {visibleLines.map((line, index) => (
            <div
              key={`${startIndex + index}-${line.slice(0, 24)}`}
              className={cn('h-5 whitespace-pre', getLogLineClass(line))}
            >
              {line || ' '}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
