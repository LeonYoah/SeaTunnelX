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

import {describe, expect, it} from 'vitest';
import {localizeDiagnosticsText} from '../text-utils';

describe('localizeDiagnosticsText', () => {
  it('picks the full english summary for bilingual inspection counts', () => {
    const zh =
      '集群 A 在最近 230 分钟巡检中生成 0 条发现（严重 0 / 告警 0 / 信息 0）';
    const en =
      'Cluster A inspection for the last 230 minutes generated 0 findings (0 critical / 0 warning / 0 info)';
    const bilingual = `${zh} / ${en}`;

    expect(localizeDiagnosticsText(bilingual, 'en')).toBe(en);
    expect(localizeDiagnosticsText(bilingual, 'zh')).toBe(zh);
  });
});
