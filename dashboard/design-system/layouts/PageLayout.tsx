import React from 'react';
import {
  PageLayout as BasePageLayout,
  PageLayoutProps as BasePageLayoutProps,
} from '../../components/PageLayout';
import { SPACING } from '../tokens/spacing';

export interface PageLayoutProps extends BasePageLayoutProps {}

export const PageLayout: React.FC<PageLayoutProps> = (props) => {
  return (
    <div className={`max-w-[1600px] mx-auto ${SPACING.pageX} ${SPACING.sectionY}`}>
      <BasePageLayout {...props} />
    </div>
  );
};

