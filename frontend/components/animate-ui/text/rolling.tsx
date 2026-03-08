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
  type UseInViewOptions,
  type Transition,
} from 'motion/react';
import {easeOut} from 'motion';

const ENTRY_ANIMATION = {
  initial: {rotateX: 0},
  animate: {rotateX: 90},
};

const EXIT_ANIMATION = {
  initial: {rotateX: 90},
  animate: {rotateX: 0},
};

const formatCharacter = (char: string) => (char === ' ' ? '\u00A0' : char);

type RollingTextProps = Omit<React.ComponentProps<'span'>, 'children'> & {
  transition?: Transition;
  inView?: boolean;
  inViewMargin?: UseInViewOptions['margin'];
  inViewOnce?: boolean;
  text: string;
  loop?: boolean;
  loopDelay?: number;
};

function RollingText({
  ref,
  transition = {duration: 0.5, delay: 0.1, ease: easeOut},
  inView = false,
  inViewMargin = '0px',
  inViewOnce = true,
  text,
  loop = false,
  loopDelay = 3000,
  ...props
}: RollingTextProps) {
  const localRef = React.useRef<HTMLSpanElement>(null);
  React.useImperativeHandle(ref, () => localRef.current!);

  const inViewResult = useInView(localRef, {
    once: inViewOnce,
    margin: inViewMargin,
  });

  const [animationCount, setAnimationCount] = React.useState(0);
  const [isAnimating, setIsAnimating] = React.useState(false);

  const isInView = !inView || inViewResult;

  React.useEffect(() => {
    if (isInView) {
      setIsAnimating(true);
    }
  }, [isInView]);

  React.useEffect(() => {
    if (!loop || !isInView) return;

    const intervalId = setInterval(() => {
      setIsAnimating(false);

      setTimeout(() => {
        setAnimationCount((prev) => prev + 1);
        setIsAnimating(true);
      }, 50);
    }, loopDelay);

    return () => clearInterval(intervalId);
  }, [loop, loopDelay, isInView]);

  const characters = React.useMemo(() => text.split(''), [text]);

  return (
    <span data-slot='rolling-text' {...props} ref={localRef}>
      {characters.map((char, idx) => (
        <span
          key={`${idx}-${animationCount}`}
          className='relative inline-block perspective-[9999999px] transform-3d w-auto'
          aria-hidden='true'
        >
          <motion.span
            className='absolute inline-block backface-hidden origin-[50%_25%]'
            initial={ENTRY_ANIMATION.initial}
            animate={
              isAnimating ? ENTRY_ANIMATION.animate : ENTRY_ANIMATION.initial
            }
            transition={{
              ...transition,
              delay: idx * (transition?.delay ?? 0),
            }}
          >
            {formatCharacter(char)}
          </motion.span>
          <motion.span
            className='absolute inline-block backface-hidden origin-[50%_100%]'
            initial={EXIT_ANIMATION.initial}
            animate={
              isAnimating ? EXIT_ANIMATION.animate : EXIT_ANIMATION.initial
            }
            transition={{
              ...transition,
              delay: idx * (transition?.delay ?? 0) + 0.3,
            }}
          >
            {formatCharacter(char)}
          </motion.span>
          <span className='invisible'>{formatCharacter(char)}</span>
        </span>
      ))}

      <span className='sr-only'>{text}</span>
    </span>
  );
}

export {RollingText, type RollingTextProps};
