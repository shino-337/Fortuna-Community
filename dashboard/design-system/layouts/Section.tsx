import React from 'react';
import {
  PageSection as BaseSection,
  PageSectionProps as BaseSectionProps,
} from '../../components/PageLayout';

export type SectionProps = BaseSectionProps;

export const Section: React.FC<SectionProps> = (props) => <BaseSection {...props} />;

