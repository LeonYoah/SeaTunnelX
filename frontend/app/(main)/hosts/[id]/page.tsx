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

/**
 * Host Detail Page
 * 主机详情页面
 */

import {Suspense} from 'react';
import Link from 'next/link';
import {HostDetailPage} from '@/components/common/host/HostDetailPage';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '主机详情',
};

interface HostDetailPageProps {
  params: Promise<{id: string}>;
}

export default async function HostDetailRoute({params}: HostDetailPageProps) {
  const {id} = await params;
  const hostId = parseInt(id, 10);

  if (isNaN(hostId) || hostId < 1) {
    return (
      <div className='space-y-4'>
        <p className='text-destructive'>无效的主机 ID</p>
        <Link href='/hosts' className='text-primary underline mt-4 inline-block'>
          返回主机列表
        </Link>
      </div>
    );
  }

  return (
    <Suspense>
      <HostDetailPage hostId={hostId} />
    </Suspense>
  );
}
