import type React from 'react';

/**
 * Keyboard handler for non-button elements that act as buttons (role="button"):
 * Enter and Space run the action, like a native button. Keys from nested
 * interactive children (links, inputs, buttons) are left alone.
 */
export const onActivateKey =
  (action: () => void) =>
  (e: React.KeyboardEvent<HTMLElement>) => {
    if (e.target !== e.currentTarget) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      action();
    }
  };
