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

import * as React from 'react';
import {motion, type Transition} from 'motion/react';

import {cn} from '@/lib/utils';

type GradientTextProps = React.ComponentProps<'span'> & {
  text: string;
  gradient?: string;
  neon?: boolean;
  transition?: Transition;
};

function GradientText({
  text,
  className,
  gradient = 'linear-gradient(90deg, #3b82f6 0%, #a855f7 20%, #ec4899 50%, #a855f7 80%, #3b82f6 100%)',
  neon = false,
  transition = {duration: 50, repeat: Infinity, ease: 'linear'},
  ...props
}: GradientTextProps) {
  const baseStyle: React.CSSProperties = {
    backgroundImage: gradient,
  };

  return (
    <span
      data-slot='gradient-text'
      className={cn('relative inline-block', className)}
      {...props}
    >
      <motion.span
        className='m-0 text-transparent bg-clip-text bg-[length:700%_100%] bg-[position:0%_0%]'
        style={baseStyle}
        initial={{backgroundPosition: '0% 0%'}}
        animate={{backgroundPosition: '500% 100%'}}
        transition={transition}
      >
        {text}
      </motion.span>

      {neon && (
        <motion.span
          className='m-0 absolute top-0 left-0 text-transparent bg-clip-text blur-[8px] mix-blend-plus-lighter bg-[length:700%_100%] bg-[position:0%_0%]'
          style={baseStyle}
          initial={{backgroundPosition: '0% 0%'}}
          animate={{backgroundPosition: '500% 100%'}}
          transition={transition}
        >
          {text}
        </motion.span>
      )}
    </span>
  );
}

export {GradientText, type GradientTextProps};
