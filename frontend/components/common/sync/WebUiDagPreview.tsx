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

import {useMemo, useState} from 'react';
import {GitBranch} from 'lucide-react';
import type {
  SyncWebUIDagEdge,
  SyncWebUIDagPreviewJob,
  SyncWebUIDagVertexInfo,
} from '@/lib/services/sync';
import {Badge} from '@/components/ui/badge';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {ScrollArea} from '@/components/ui/scroll-area';
import {cn} from '@/lib/utils';

interface PositionedNode extends SyncWebUIDagVertexInfo {
  level: number;
  row: number;
  x: number;
  y: number;
  height: number;
}

const NODE_WIDTH = 220;
const NODE_MIN_HEIGHT = 120;
const COLUMN_GAP = 92;
const ROW_GAP = 44;
const PADDING = 28;
const CANVAS_RIGHT_PADDING = 96;

function normalizeVertices(
  job: SyncWebUIDagPreviewJob,
): SyncWebUIDagVertexInfo[] {
  return Object.values(job.jobDag?.vertexInfoMap || {}).sort(
    (left, right) => left.vertexId - right.vertexId,
  );
}

function normalizeEdges(job: SyncWebUIDagPreviewJob): SyncWebUIDagEdge[] {
  return Object.entries(job.jobDag?.pipelineEdges || {})
    .sort(([left], [right]) => Number(left) - Number(right))
    .flatMap(([, edges]) => edges || []);
}

function computeNodeLevels(
  vertices: SyncWebUIDagVertexInfo[],
  edges: SyncWebUIDagEdge[],
): PositionedNode[] {
  const incoming = new Map<number, number>();
  const outgoing = new Map<number, number[]>();
  const vertexByID = new Map<number, SyncWebUIDagVertexInfo>();

  for (const vertex of vertices) {
    incoming.set(vertex.vertexId, 0);
    outgoing.set(vertex.vertexId, []);
    vertexByID.set(vertex.vertexId, vertex);
  }

  for (const edge of edges) {
    outgoing.set(edge.inputVertexId, [
      ...(outgoing.get(edge.inputVertexId) || []),
      edge.targetVertexId,
    ]);
    incoming.set(
      edge.targetVertexId,
      (incoming.get(edge.targetVertexId) || 0) + 1,
    );
  }

  const queue = vertices
    .filter((vertex) => (incoming.get(vertex.vertexId) || 0) === 0)
    .map((vertex) => vertex.vertexId);

  const levels = new Map<number, number>();
  for (const vertex of vertices) {
    levels.set(vertex.vertexId, 0);
  }

  while (queue.length > 0) {
    const current = queue.shift()!;
    const currentLevel = levels.get(current) || 0;
    for (const next of outgoing.get(current) || []) {
      levels.set(next, Math.max(levels.get(next) || 0, currentLevel + 1));
      incoming.set(next, Math.max(0, (incoming.get(next) || 1) - 1));
      if ((incoming.get(next) || 0) === 0) {
        queue.push(next);
      }
    }
  }

  const rowsByLevel = new Map<number, number>();
  const yOffsetByLevel = new Map<number, number>();
  return vertices.map((vertex) => {
    const level = levels.get(vertex.vertexId) || 0;
    const row = rowsByLevel.get(level) || 0;
    const height = estimateNodeHeight(vertex);
    const y = yOffsetByLevel.get(level) || PADDING;
    rowsByLevel.set(level, row + 1);
    yOffsetByLevel.set(level, y + height + ROW_GAP);
    return {
      ...vertex,
      level,
      row,
      x: PADDING + level * (NODE_WIDTH + COLUMN_GAP),
      y,
      height,
    };
  });
}

function nodeTone(type: string): string {
  switch (type) {
    case 'source':
      return 'border-emerald-500/40 bg-emerald-500/5';
    case 'sink':
      return 'border-amber-500/40 bg-amber-500/5';
    default:
      return 'border-blue-500/40 bg-blue-500/5';
  }
}

function nodeBadgeTone(type: string): string {
  switch (type) {
    case 'source':
      return 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300';
    case 'sink':
      return 'bg-amber-500/15 text-amber-700 dark:text-amber-300';
    default:
      return 'bg-blue-500/15 text-blue-700 dark:text-blue-300';
  }
}

function normalizeTablePaths(paths?: string[]): string[] {
  return (paths || []).filter(Boolean);
}

function estimateNodeHeight(node: SyncWebUIDagVertexInfo): number {
  const connectorLines = Math.max(
    1,
    Math.ceil((node.connectorType?.length || 0) / 22),
  );
  const tableCount = Math.max(1, normalizeTablePaths(node.tablePaths).length);
  const headerHeight = 72;
  const connectorHeight = connectorLines * 18;
  const tableSectionLabelHeight = 20;
  const tableButtonHeight = tableCount * 30;
  const tableButtonGap = Math.max(0, tableCount - 1) * 6;
  const bottomPadding = 22;
  return Math.max(
    NODE_MIN_HEIGHT,
    headerHeight +
      connectorHeight +
      tableSectionLabelHeight +
      tableButtonHeight +
      tableButtonGap +
      bottomPadding,
  );
}

export function WebUiDagPreview({job}: {job: SyncWebUIDagPreviewJob}) {
  const [selectedTableDetail, setSelectedTableDetail] = useState<{
    nodeLabel: string;
    tablePath: string;
    columns: string[];
  } | null>(null);
  const vertices = useMemo(() => normalizeVertices(job), [job]);
  const edges = useMemo(() => normalizeEdges(job), [job]);
  const positionedNodes = useMemo(
    () => computeNodeLevels(vertices, edges),
    [vertices, edges],
  );
  const nodeByID = useMemo(
    () =>
      new Map(positionedNodes.map((node) => [node.vertexId, node] as const)),
    [positionedNodes],
  );
  const width =
    positionedNodes.length === 0
      ? 0
      : Math.max(...positionedNodes.map((node) => node.x)) +
        NODE_WIDTH +
        PADDING +
        CANVAS_RIGHT_PADDING;
  const height =
    positionedNodes.length === 0
      ? 0
      : Math.max(...positionedNodes.map((node) => node.y + node.height)) +
        PADDING;

  return (
    <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]'>
      <Card className='overflow-hidden'>
        <CardHeader className='border-b pb-3'>
          <CardTitle className='flex items-center gap-2 text-sm'>
            <GitBranch className='h-4 w-4' />
            DAG
          </CardTitle>
        </CardHeader>
        <CardContent className='p-0'>
          <div className='h-[560px] w-full overflow-x-auto overflow-y-auto'>
            <div
              className='relative min-h-[560px] min-w-max bg-muted/15'
              style={{
                width: Math.max(width, 820),
                height: Math.max(height, 560),
              }}
            >
              <svg
                className='absolute inset-0'
                width={Math.max(width, 820)}
                height={Math.max(height, 560)}
              >
                <defs>
                  <marker
                    id='sync-dag-arrow'
                    markerWidth='8'
                    markerHeight='8'
                    refX='7'
                    refY='4'
                    orient='auto'
                  >
                    <path
                      d='M0,0 L8,4 L0,8 z'
                      className='fill-muted-foreground/60'
                    />
                  </marker>
                </defs>
                {edges.map((edge, index) => {
                  const source = nodeByID.get(edge.inputVertexId);
                  const target = nodeByID.get(edge.targetVertexId);
                  if (!source || !target) {
                    return null;
                  }
                  const x1 = source.x + NODE_WIDTH;
                  const y1 = source.y + source.height / 2;
                  const x2 = target.x;
                  const y2 = target.y + target.height / 2;
                  const midX = x1 + (x2 - x1) / 2;
                  const path = `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`;
                  return (
                    <path
                      key={`${edge.inputVertexId}-${edge.targetVertexId}-${index}`}
                      d={path}
                      fill='none'
                      stroke='currentColor'
                      strokeWidth='2'
                      className='text-muted-foreground/60'
                      markerEnd='url(#sync-dag-arrow)'
                    />
                  );
                })}
              </svg>

              {positionedNodes.map((node) => (
                <div
                  key={node.vertexId}
                  className={cn(
                    'absolute rounded-xl border shadow-sm backdrop-blur-sm',
                    nodeTone(node.type),
                  )}
                  style={{
                    left: node.x,
                    top: node.y,
                    width: NODE_WIDTH,
                    minHeight: node.height,
                  }}
                >
                  <div className='flex h-full flex-col gap-3 p-4'>
                    <div className='flex items-center justify-between gap-2'>
                      <Badge
                        variant='secondary'
                        className={cn(
                          'border-transparent text-[11px] uppercase tracking-wide',
                          nodeBadgeTone(node.type),
                        )}
                      >
                        {node.type}
                      </Badge>
                      <span className='text-[11px] text-muted-foreground'>
                        #{node.vertexId}
                      </span>
                    </div>
                    <div className='space-y-1'>
                      <div className='line-clamp-2 text-sm font-medium'>
                        {node.connectorType}
                      </div>
                      <div className='space-y-2'>
                        <div className='text-[11px] font-medium uppercase tracking-wide text-muted-foreground'>
                          TablePaths
                        </div>
                        <div className='space-y-1.5'>
                          {normalizeTablePaths(node.tablePaths).length > 0 ? (
                            normalizeTablePaths(node.tablePaths).map((path) => (
                              <button
                                key={`${node.vertexId}-${path}`}
                                type='button'
                                title={path}
                                className='flex w-full items-center rounded-md border border-border/50 bg-background/80 px-2 py-1 text-left text-[11px] hover:bg-background'
                                onClick={() =>
                                  setSelectedTableDetail({
                                    nodeLabel: `#${node.vertexId} ${node.connectorType}`,
                                    tablePath: path,
                                    columns: node.tableColumns?.[path] || [],
                                  })
                                }
                              >
                                <span className='block truncate'>{path}</span>
                              </button>
                            ))
                          ) : (
                            <Badge
                              variant='secondary'
                              className='rounded-md border border-border/50 bg-background/80 text-[11px]'
                            >
                              default
                            </Badge>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className='space-y-4'>
        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm'>预览摘要</CardTitle>
          </CardHeader>
          <CardContent className='space-y-2 text-sm'>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>Job</span>
              <span>{job.jobName}</span>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>Status</span>
              <Badge variant='outline'>{job.jobStatus}</Badge>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>Nodes</span>
              <span>{vertices.length}</span>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>Edges</span>
              <span>{edges.length}</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm'>节点列表</CardTitle>
          </CardHeader>
          <CardContent className='space-y-2'>
            {vertices.map((vertex) => (
              <div
                key={vertex.vertexId}
                className='rounded-lg border border-border/60 p-3'
              >
                <div className='flex items-center justify-between gap-2'>
                  <div className='text-sm font-medium'>
                    {vertex.connectorType}
                  </div>
                  <Badge variant='outline' className='capitalize'>
                    {vertex.type}
                  </Badge>
                </div>
                <div className='mt-3 space-y-2'>
                  <div className='text-[11px] font-medium uppercase tracking-wide text-muted-foreground'>
                    TablePaths
                  </div>
                  <div className='space-y-1.5'>
                    {normalizeTablePaths(vertex.tablePaths).length > 0 ? (
                      normalizeTablePaths(vertex.tablePaths).map((path) => (
                        <button
                          key={`${vertex.vertexId}-${path}`}
                          type='button'
                          title={path}
                          className='flex w-full items-center rounded-md border border-border/50 bg-background/80 px-2 py-1 text-left text-[11px] hover:bg-background'
                          onClick={() =>
                            setSelectedTableDetail({
                              nodeLabel: `#${vertex.vertexId} ${vertex.connectorType}`,
                              tablePath: path,
                              columns: vertex.tableColumns?.[path] || [],
                            })
                          }
                        >
                          <span className='block truncate'>{path}</span>
                        </button>
                      ))
                    ) : (
                      <Badge
                        variant='secondary'
                        className='rounded-md border border-border/50 bg-background/80 text-[11px]'
                      >
                        default
                      </Badge>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
      <Dialog
        open={Boolean(selectedTableDetail)}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedTableDetail(null);
          }
        }}
      >
        <DialogContent className='sm:max-w-[720px]'>
          <DialogHeader>
            <DialogTitle>
              {selectedTableDetail?.tablePath || 'Table Detail'}
            </DialogTitle>
          </DialogHeader>
          <div className='space-y-3'>
            <div className='text-sm text-muted-foreground'>
              {selectedTableDetail?.nodeLabel}
            </div>
            <div className='rounded-lg border border-border/60 bg-background/80 p-3'>
              <div className='mb-2 text-sm font-medium'>Columns</div>
              {selectedTableDetail?.columns?.length ? (
                <div className='flex flex-wrap gap-2'>
                  {selectedTableDetail.columns.map((column) => (
                    <Badge
                      key={column}
                      variant='secondary'
                      className='rounded-md'
                    >
                      {column}
                    </Badge>
                  ))}
                </div>
              ) : (
                <div className='text-sm text-muted-foreground'>
                  No schema columns available
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
