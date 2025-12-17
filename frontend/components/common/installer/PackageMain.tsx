/**
 * Package Management Main Component
 * 安装包管理主组件
 */

'use client';

import { useState } from 'react';
import { usePackages } from '@/hooks/use-installer';
import { PackageTable } from './PackageTable';
import { UploadPackageDialog } from './UploadPackageDialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Upload, RefreshCw, Package, Cloud, HardDrive } from 'lucide-react';
import { useTranslations } from 'next-intl';

export function PackageMain() {
  const t = useTranslations();
  const { packages, loading, error, refresh, uploadPackage, deletePackage } = usePackages();
  const [uploadDialogOpen, setUploadDialogOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<'online' | 'local'>('online');

  const handleUpload = async (file: File, version: string) => {
    await uploadPackage(file, version);
    setUploadDialogOpen(false);
  };

  const handleDelete = async (version: string) => {
    if (confirm(t('installer.confirmDeletePackage', { version }))) {
      await deletePackage(version);
    }
  };

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header / 头部 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Package className="h-6 w-6" />
            {t('installer.packageManagement')}
          </h1>
          <p className="text-muted-foreground mt-1">
            {t('installer.packageManagementDesc')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={refresh} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            {t('common.refresh')}
          </Button>
          <Button size="sm" onClick={() => setUploadDialogOpen(true)}>
            <Upload className="h-4 w-4 mr-2" />
            {t('installer.uploadPackage')}
          </Button>
        </div>
      </div>

      {/* Error display / 错误显示 */}
      {error && (
        <Card className="border-destructive">
          <CardContent className="pt-6">
            <p className="text-destructive">{error}</p>
          </CardContent>
        </Card>
      )}

      {/* Stats cards / 统计卡片 */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('installer.availableVersions')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {packages?.versions.length || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('installer.localPackages')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {packages?.local_packages.length || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('installer.recommendedVersion')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-primary">
              {packages?.recommended_version || '-'}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Package tabs / 安装包标签页 */}
      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'online' | 'local')}>
        <TabsList>
          <TabsTrigger value="online" className="flex items-center gap-2">
            <Cloud className="h-4 w-4" />
            {t('installer.onlineVersions')}
          </TabsTrigger>
          <TabsTrigger value="local" className="flex items-center gap-2">
            <HardDrive className="h-4 w-4" />
            {t('installer.localPackages')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="online" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>{t('installer.onlineVersions')}</CardTitle>
              <CardDescription>
                {t('installer.onlineVersionsDesc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <PackageTable
                type="online"
                versions={packages?.versions || []}
                recommendedVersion={packages?.recommended_version}
                loading={loading}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="local" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>{t('installer.localPackages')}</CardTitle>
              <CardDescription>
                {t('installer.localPackagesDesc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <PackageTable
                type="local"
                localPackages={packages?.local_packages || []}
                loading={loading}
                onDelete={handleDelete}
              />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Upload dialog / 上传对话框 */}
      <UploadPackageDialog
        open={uploadDialogOpen}
        onOpenChange={setUploadDialogOpen}
        onUpload={handleUpload}
        existingVersions={packages?.versions || []}
      />
    </div>
  );
}
