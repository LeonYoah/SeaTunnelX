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

import {Suspense} from 'react';
import {useSearchParams} from 'next/navigation';
import {InstallWizard} from '@/components/common/installer';

function E2EInstallWizardContent() {
  const searchParams = useSearchParams();
  const hostId = Number(searchParams.get('hostId') || '1') || 1;
  const hostName = searchParams.get('hostName') || 'E2E Host';
  const initialVersion = searchParams.get('initialVersion') || '2.3.13';
  const initialInstallDir =
    searchParams.get('initialInstallDir') || undefined;
  const initialClusterPort =
    Number(searchParams.get('initialClusterPort') || '0') || undefined;
  const initialHttpPort =
    Number(searchParams.get('initialHttpPort') || '0') || undefined;

  return (
    <InstallWizard
      open
      onOpenChange={() => {}}
      hostId={hostId}
      hostName={hostName}
      initialVersion={initialVersion}
      initialInstallDir={initialInstallDir}
      initialClusterPort={initialClusterPort}
      initialHttpPort={initialHttpPort}
    />
  );
}

export default function E2EInstallWizardPage() {
  return (
    <Suspense fallback={null}>
      <E2EInstallWizardContent />
    </Suspense>
  );
}
