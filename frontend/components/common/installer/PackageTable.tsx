/**
 * Package Table Component
 * 安装包表格组件
 */

'use client';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';
import { Download, Trash2, MoreHorizontal, Star, ExternalLink } from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { PackageInfo, MirrorSource } from '@/lib/services/installer/types';

interface PackageTableProps {
  type: 'online' | 'local';
  versions?: string[];
  localPackages?: PackageInfo[];
  recommendedVersion?: string;
  loading?: boolean;
  onDelete?: (version: string) => void;
}

// Mirror source labels / 镜像源标签
const mirrorLabels: Record<MirrorSource, string> = {
  aliyun: '阿里云 Aliyun',
  huaweicloud: '华为云 HuaweiCloud',
  apache: 'Apache Archive',
};

// Format file size / 格式化文件大小
function formatFileSize(bytes: number): string {
  if (bytes === 0) {
    return '0 B';
  }
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Format date / 格式化日期
function formatDate(dateStr?: string): string {
  if (!dateStr) {
    return '-';
  }
  return new Date(dateStr).toLocaleString();
}

export function PackageTable({
  type,
  versions = [],
  localPackages = [],
  recommendedVersion,
  loading,
  onDelete,
}: PackageTableProps) {
  const t = useTranslations();

  if (loading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  // Online versions table / 在线版本表格
  if (type === 'online') {
    if (versions.length === 0) {
      return (
        <div className="text-center py-8 text-muted-foreground">
          {t('installer.noVersionsAvailable')}
        </div>
      );
    }

    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('installer.version')}</TableHead>
            <TableHead>{t('installer.status')}</TableHead>
            <TableHead>{t('installer.downloadLinks')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {versions.map((version) => (
            <TableRow key={version}>
              <TableCell className="font-medium">
                <div className="flex items-center gap-2">
                  {version}
                  {version === recommendedVersion && (
                    <Badge variant="default" className="flex items-center gap-1">
                      <Star className="h-3 w-3" />
                      {t('installer.recommended')}
                    </Badge>
                  )}
                </div>
              </TableCell>
              <TableCell>
                <Badge variant="outline">{t('installer.available')}</Badge>
              </TableCell>
              <TableCell>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm">
                      <Download className="h-4 w-4 mr-2" />
                      {t('installer.download')}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {Object.entries(mirrorLabels).map(([mirror, label]) => (
                      <DropdownMenuItem key={mirror} asChild>
                        <a
                          href={`https://mirrors.aliyun.com/apache/seatunnel/${version}/apache-seatunnel-${version}-bin.tar.gz`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-2"
                        >
                          <ExternalLink className="h-4 w-4" />
                          {label}
                        </a>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    );
  }

  // Local packages table / 本地安装包表格
  if (localPackages.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        {t('installer.noLocalPackages')}
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('installer.version')}</TableHead>
          <TableHead>{t('installer.fileName')}</TableHead>
          <TableHead>{t('installer.fileSize')}</TableHead>
          <TableHead>{t('installer.uploadedAt')}</TableHead>
          <TableHead className="w-[100px]">{t('common.actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {localPackages.map((pkg) => (
          <TableRow key={pkg.version}>
            <TableCell className="font-medium">{pkg.version}</TableCell>
            <TableCell className="font-mono text-sm">{pkg.file_name}</TableCell>
            <TableCell>{formatFileSize(pkg.file_size)}</TableCell>
            <TableCell>{formatDate(pkg.uploaded_at)}</TableCell>
            <TableCell>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon">
                    <MoreHorizontal className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    className="text-destructive"
                    onClick={() => onDelete?.(pkg.version)}
                  >
                    <Trash2 className="h-4 w-4 mr-2" />
                    {t('common.delete')}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
