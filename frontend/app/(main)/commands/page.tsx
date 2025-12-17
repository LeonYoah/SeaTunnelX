/**
 * Command Log Page
 * 命令记录页面
 */

import {Suspense} from 'react';
import {CommandMain} from '@/components/common/audit';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '命令记录',
};

export default function CommandsPage() {
  return (
    <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
      <Suspense>
        <CommandMain />
      </Suspense>
    </div>
  );
}
