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
import {
  motion,
  useInView,
  type Transition,
  type UseInViewOptions,
} from 'motion/react';

type WritingTextProps = Omit<React.ComponentProps<'span'>, 'children'> & {
  transition?: Transition;
  inView?: boolean;
  inViewMargin?: UseInViewOptions['margin'];
  inViewOnce?: boolean;
  spacing?: number | string;
  text: string;
};

function WritingText({
  ref,
  inView = false,
  inViewMargin = '0px',
  inViewOnce = true,
  spacing = 5,
  text,
  transition = {type: 'spring', bounce: 0, duration: 2, delay: 0.5},
  ...props
}: WritingTextProps) {
  const localRef = React.useRef<HTMLSpanElement>(null);
  React.useImperativeHandle(ref, () => localRef.current as HTMLSpanElement);

  const inViewResult = useInView(localRef, {
    once: inViewOnce,
    margin: inViewMargin,
  });
  const isInView = !inView || inViewResult;

  const words = React.useMemo(() => text.split(' '), [text]);

  return (
    <span ref={localRef} data-slot='writing-text' {...props}>
      {words.map((word, index) => (
        <motion.span
          key={index}
          className='inline-block will-change-transform will-change-opacity'
          style={{marginRight: spacing}}
          initial={{opacity: 0, y: 10}}
          animate={isInView ? {opacity: 1, y: 0} : undefined}
          transition={{
            ...transition,
            delay: index * (transition?.delay ?? 0),
          }}
        >
          {word}{' '}
        </motion.span>
      ))}
    </span>
  );
}

export {WritingText, type WritingTextProps};
