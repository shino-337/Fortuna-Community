import React from 'react';
import {
  PageLayout as BasePageLayout,
  PageLayoutProps as BasePageLayoutProps,
} from '../../components/PageLayout';
import { SPACING } from '../tokens/spacing';

export interface PageLayoutProps extends BasePageLayoutProps {}

export const PageLayout: React.FC<PageLayoutProps> = ({ compact, fillHeight, ...props }) => {
  return (
    <div
      className={`w-full max-w-none ${SPACING.pageX} ${fillHeight ? 'flex flex-col flex-1 min-h-0' : compact ? 'space-y-1' : SPACING.sectionY}`}
    >
      <BasePageLayout compact={compact} fillHeight={fillHeight} {...props} />
    </div>
  );
};

