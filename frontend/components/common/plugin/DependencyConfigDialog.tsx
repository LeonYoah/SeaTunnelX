/**
 * Plugin Dependency Configuration Dialog
 * 插件依赖配置对话框
 */

'use client';

import { useState, useEffect, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { toast } from 'sonner';
import { Plus, Trash2, Package, RefreshCw, FileCode } from 'lucide-react';
import { PluginService } from '@/lib/services/plugin';
import type { PluginDependencyConfig } from '@/lib/services/plugin';

interface DependencyConfigDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pluginName: string;
}

/**
 * Dialog for configuring plugin dependencies
 * 配置插件依赖的对话框
 */
export function DependencyConfigDialog({
  open,
  onOpenChange,
  pluginName,
}: DependencyConfigDialogProps) {
  const t = useTranslations();

  // Dependencies state / 依赖状态
  const [dependencies, setDependencies] = useState<PluginDependencyConfig[]>([]);
  const [loading, setLoading] = useState(false);

  // Add form state / 添加表单状态
  const [showAddForm, setShowAddForm] = useState(false);
  const [groupId, setGroupId] = useState('');
  const [artifactId, setArtifactId] = useState('');
  const [version, setVersion] = useState('');
  const [adding, setAdding] = useState(false);

  // Maven XML parse state / Maven XML 解析状态
  const [showXmlParse, setShowXmlParse] = useState(false);
  const [mavenXml, setMavenXml] = useState('');

  /**
   * Load dependencies / 加载依赖
   */
  const loadDependencies = useCallback(async () => {
    if (!pluginName) return;
    setLoading(true);
    try {
      const deps = await PluginService.listDependencies(pluginName);
      setDependencies(deps || []);
    } catch (err) {
      console.error('Failed to load dependencies:', err);
      setDependencies([]);
    } finally {
      setLoading(false);
    }
  }, [pluginName]);

  // Load dependencies when dialog opens / 对话框打开时加载依赖
  useEffect(() => {
    if (open && pluginName) {
      loadDependencies();
    }
  }, [open, pluginName, loadDependencies]);

  /**
   * Handle add dependency / 处理添加依赖
   */
  const handleAddDependency = async () => {
    if (!groupId.trim() || !artifactId.trim() || !version.trim()) {
      toast.error(t('plugin.allFieldsRequired'));
      return;
    }

    setAdding(true);
    try {
      await PluginService.addDependency(pluginName, {
        group_id: groupId.trim(),
        artifact_id: artifactId.trim(),
        version: version.trim(),
      });
      toast.success(t('plugin.addDependencySuccess'));
      // Reset form / 重置表单
      setGroupId('');
      setArtifactId('');
      setVersion('');
      setShowAddForm(false);
      // Reload dependencies / 重新加载依赖
      loadDependencies();
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.addDependencyFailed');
      toast.error(errorMsg);
    } finally {
      setAdding(false);
    }
  };

  /**
   * Handle delete dependency / 处理删除依赖
   */
  const handleDeleteDependency = async (dep: PluginDependencyConfig) => {
    try {
      await PluginService.deleteDependency(pluginName, dep.id);
      toast.success(t('plugin.deleteDependencySuccess'));
      loadDependencies();
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.deleteDependencyFailed');
      toast.error(errorMsg);
    }
  };

  /**
   * Parse Maven XML and extract dependency info
   * 解析 Maven XML 并提取依赖信息
   */
  const parseMavenXml = (xml: string): { groupId: string; artifactId: string; version: string } | null => {
    try {
      // Extract groupId / 提取 groupId
      const groupIdMatch = xml.match(/<groupId>\s*([^<]+)\s*<\/groupId>/);
      // Extract artifactId / 提取 artifactId
      const artifactIdMatch = xml.match(/<artifactId>\s*([^<]+)\s*<\/artifactId>/);
      // Extract version / 提取 version
      const versionMatch = xml.match(/<version>\s*([^<]+)\s*<\/version>/);

      if (!groupIdMatch || !artifactIdMatch) {
        return null;
      }

      return {
        groupId: groupIdMatch[1].trim(),
        artifactId: artifactIdMatch[1].trim(),
        version: versionMatch ? versionMatch[1].trim() : '',
      };
    } catch {
      return null;
    }
  };

  /**
   * Handle parse Maven XML / 处理解析 Maven XML
   */
  const handleParseMavenXml = () => {
    if (!mavenXml.trim()) {
      toast.error(t('plugin.mavenXmlEmpty'));
      return;
    }

    const parsed = parseMavenXml(mavenXml);
    if (!parsed) {
      toast.error(t('plugin.mavenXmlParseError'));
      return;
    }

    // Fill form with parsed values / 用解析的值填充表单
    setGroupId(parsed.groupId);
    setArtifactId(parsed.artifactId);
    setVersion(parsed.version);
    setShowXmlParse(false);
    setShowAddForm(true);
    setMavenXml('');
    toast.success(t('plugin.mavenXmlParseSuccess'));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Package className="h-5 w-5" />
            {t('plugin.dependencies')} - {pluginName}
          </DialogTitle>
          <DialogDescription>
            {t('plugin.dependenciesDesc')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Dependencies table / 依赖表格 */}
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : dependencies.length === 0 && !showAddForm ? (
            <div className="text-center py-8 text-muted-foreground">
              <Package className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>{t('plugin.noDependencies')}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('plugin.groupId')}</TableHead>
                  <TableHead>{t('plugin.artifactId')}</TableHead>
                  <TableHead>{t('plugin.version')}</TableHead>
                  <TableHead className="w-[80px]">{t('common.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {dependencies.map((dep) => (
                  <TableRow key={dep.id}>
                    <TableCell className="font-mono text-sm">{dep.group_id}</TableCell>
                    <TableCell className="font-mono text-sm">{dep.artifact_id}</TableCell>
                    <TableCell className="font-mono text-sm">
                      {dep.version || <span className="text-muted-foreground">-</span>}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => handleDeleteDependency(dep)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}

          {/* Maven XML parse form / Maven XML 解析表单 */}
          {showXmlParse && (
            <div className="border rounded-lg p-4 space-y-4 bg-muted/30">
              <h4 className="font-medium flex items-center gap-2">
                <FileCode className="h-4 w-4" />
                {t('plugin.parseMavenXml')}
              </h4>
              <p className="text-sm text-muted-foreground">
                {t('plugin.parseMavenXmlDesc')}
              </p>
              <Textarea
                value={mavenXml}
                onChange={(e) => setMavenXml(e.target.value)}
                placeholder={`<dependency>
  <groupId>com.mysql</groupId>
  <artifactId>mysql-connector-j</artifactId>
  <version>8.0.33</version>
</dependency>`}
                className="font-mono text-sm min-h-[120px]"
              />
              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowXmlParse(false);
                    setMavenXml('');
                  }}
                >
                  {t('common.cancel')}
                </Button>
                <Button onClick={handleParseMavenXml}>
                  {t('plugin.parseAndFill')}
                </Button>
              </div>
            </div>
          )}

          {/* Add form / 添加表单 */}
          {showAddForm && (
            <div className="border rounded-lg p-4 space-y-4 bg-muted/30">
              <h4 className="font-medium">{t('plugin.addDependencyDesc')}</h4>
              <div className="grid grid-cols-3 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="groupId">{t('plugin.groupId')} *</Label>
                  <Input
                    id="groupId"
                    value={groupId}
                    onChange={(e) => setGroupId(e.target.value)}
                    placeholder={t('plugin.groupIdPlaceholder')}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="artifactId">{t('plugin.artifactId')} *</Label>
                  <Input
                    id="artifactId"
                    value={artifactId}
                    onChange={(e) => setArtifactId(e.target.value)}
                    placeholder={t('plugin.artifactIdPlaceholder')}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="version">{t('plugin.version')} *</Label>
                  <Input
                    id="version"
                    value={version}
                    onChange={(e) => setVersion(e.target.value)}
                    placeholder={t('plugin.versionPlaceholderRequired')}
                  />
                </div>
              </div>
              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowAddForm(false);
                    setGroupId('');
                    setArtifactId('');
                    setVersion('');
                  }}
                >
                  {t('common.cancel')}
                </Button>
                <Button onClick={handleAddDependency} disabled={adding}>
                  {adding ? t('common.saving') : t('common.save')}
                </Button>
              </div>
            </div>
          )}
        </div>

        <DialogFooter>
          {!showAddForm && !showXmlParse && (
            <>
              <Button variant="outline" onClick={() => setShowXmlParse(true)}>
                <FileCode className="h-4 w-4 mr-2" />
                {t('plugin.parseMavenXml')}
              </Button>
              <Button variant="outline" onClick={() => setShowAddForm(true)}>
                <Plus className="h-4 w-4 mr-2" />
                {t('plugin.addDependency')}
              </Button>
            </>
          )}
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('common.close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
