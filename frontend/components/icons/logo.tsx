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

import React from 'react';

interface LogoProps {
  width?: number;
  height?: number;
  className?: string;
}

function LinuxDo({width = 120, height = 120, className}: LogoProps) {
  return (
    <svg
      width={width}
      height={height}
      viewBox='0 0 120 120'
      xmlns='http://www.w3.org/2000/svg'
      className={className}
      style={{minWidth: width, minHeight: height, flexShrink: 0}}
      fill='none'
    >
      <clipPath id='a'>
        <circle cx='60' cy='60' r='47' />
      </clipPath>
      <circle fill='#f0f0f0' cx='60' cy='60' r='50' />
      <rect
        fill='#1c1c1e'
        clipPath='url(#a)'
        x='10'
        y='10'
        width='100'
        height='30'
      />
      <rect
        fill='#f0f0f0'
        clipPath='url(#a)'
        x='10'
        y='40'
        width='100'
        height='40'
      />
      <rect
        fill='#ffb003'
        clipPath='url(#a)'
        x='10'
        y='80'
        width='100'
        height='30'
      />
    </svg>
  );
}

// 统一导出所有图标
export {LinuxDo};
