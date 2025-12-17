/**
 * Plugin Detail Dialog Component
 * 插件详情对话框组件
 */

'use client';

import { useTranslations } from 'next-intl';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Database, Upload, Shuffle, ExternalLink, Package, FolderOpen } from 'lucide-react';
import type { Plugin, PluginCategory } from '@/lib/services/plugin';

interface PluginDetailDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  plugin: Plugin;
}

/**
 * Get category icon
 * 获取分类图标
 */
function getCategoryIcon(category: PluginCategory) {
  switch (category) {
    case 'source':
      return <Database className="h-5 w-5" />;
    case 'sink':
      return <Upload className="h-5 w-5" />;
    case 'transform':
      return <Shuffle className="h-5 w-5" />;
    default:
      return <Database className="h-5 w-5" />;
  }
}

/**
 * Get category color
 * 获取分类颜色
 */
function getCategoryColor(category: PluginCategory): string {
  switch (category) {
    case 'source':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300';
    case 'sink':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
    case 'transform':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300';
  }
}

/**
 * Plugin Detail Dialog Component
 * 插件详情对话框组件 - 展示插件的完整信息
 */
export function PluginDetailDialog({ open, onOpenChange, plugin }: PluginDetailDialogProps) {
  const t = useTranslations();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh]">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className={`p-2 rounded-lg ${getCategoryColor(plugin.category)}`}>
              {getCategoryIcon(plugin.category)}
            </div>
            <div>
              <DialogTitle className="text-xl">
                {plugin.display_name || plugin.name}
              </DialogTitle>
              <DialogDescription className="text-sm">
                {plugin.name}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <ScrollArea className="max-h-[60vh]">
          <div className="space-y-6 pr-4">
            {/* Basic Info / 基本信息 */}
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant="outline" className={getCategoryColor(plugin.category)}>
                  {t(`plugin.category.${plugin.category}`)}
                </Badge>
                <Badge variant="secondary">v{plugin.version}</Badge>
              </div>
              
              <p className="text-sm text-muted-foreground">
                {plugin.description || t('plugin.noDescription')}
              </p>

              {plugin.doc_url && (
                <Button variant="outline" size="sm" asChild>
                  <a href={plugin.doc_url} target="_blank" rel="noopener noreferrer">
                    <ExternalLink className="h-4 w-4 mr-2" />
                    {t('plugin.viewDocumentation')}
                  </a>
                </Button>
              )}
            </div>

            <Separator />

            {/* Maven Coordinates / Maven 坐标 */}
            <div className="space-y-3">
              <h4 className="font-medium flex items-center gap-2">
                <Package className="h-4 w-4" />
                {t('plugin.mavenCoordinates')}
              </h4>
              <div className="bg-muted p-3 rounded-md font-mono text-sm">
                <div><span className="text-muted-foreground">groupId:</span> {plugin.group_id}</div>
                <div><span className="text-muted-foreground">artifactId:</span> {plugin.artifact_id}</div>
                <div><span className="text-muted-foreground">version:</span> {plugin.version}</div>
              </div>
            </div>

            <Separator />

            {/* Install Path / 安装路径 */}
            <div className="space-y-3">
              <h4 className="font-medium flex items-center gap-2">
                <FolderOpen className="h-4 w-4" />
                {t('plugin.installPath')}
              </h4>
              <div className="text-sm space-y-2">
                <div className="flex items-center gap-2">
                  <Badge variant="outline">{t('plugin.connectorPath')}</Badge>
                  <code className="text-xs bg-muted px-2 py-1 rounded">connectors/</code>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="outline">{t('plugin.libPath')}</Badge>
                  <code className="text-xs bg-muted px-2 py-1 rounded">lib/</code>
                </div>
              </div>
            </div>

            {/* Dependencies / 依赖库 */}
            {plugin.dependencies && plugin.dependencies.length > 0 && (
              <>
                <Separator />
                <div className="space-y-3">
                  <h4 className="font-medium">
                    {t('plugin.dependencies')} ({plugin.dependencies.length})
                  </h4>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('plugin.artifactId')}</TableHead>
                        <TableHead>{t('plugin.version')}</TableHead>
                        <TableHead>{t('plugin.targetDir')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {plugin.dependencies.map((dep, index) => (
                        <TableRow key={index}>
                          <TableCell className="font-mono text-xs">
                            {dep.group_id}:{dep.artifact_id}
                          </TableCell>
                          <TableCell className="font-mono text-xs">
                            {dep.version}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline" className="text-xs">
                              {dep.target_dir}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </>
            )}

            {/* Version Note / 版本说明 */}
            <Separator />
            <div className="text-xs text-muted-foreground bg-muted/50 p-3 rounded-md">
              {t('plugin.versionNote')}
            </div>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}
