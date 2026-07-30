import React from 'react';
import {
  PageLayout as BasePageLayout,
  PageLayoutProps as BasePageLayoutProps,
} from '../../components/PageLayout';
import { PageContainer } from './PageContainer';

export interface PageLayoutProps extends BasePageLayoutProps {}

export const PageLayout: React.FC<PageLayoutProps> = ({ compact, fillHeight, ...props }) => {
  return (
    <PageContainer fillHeight={fillHeight}>
      <BasePageLayout compact={compact} fillHeight={fillHeight} {...props} />
    </PageContainer>
  );
};

