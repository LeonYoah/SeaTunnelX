/**
 * Audit Log Page
 * 审计日志页面
 */

import {Suspense} from 'react';
import {AuditLogMain} from '@/components/common/audit';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '审计日志',
};

export default function AuditLogsPage() {
  return (
    <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
      <Suspense>
        <AuditLogMain />
      </Suspense>
    </div>
  );
}
