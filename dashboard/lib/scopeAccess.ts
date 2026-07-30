export type { OwnershipContext, ScopeDocument } from './ownershipContext';
export {
  buildOwnershipContext,
  parseScopeDocument,
} from './ownershipContext';
export type { OperationalScope } from './operationalScope';
export {
  emptyOperationalScope,
  operationalScopeFromDocument,
  resolveOperationalScope,
  scopeSummaryLabel,
} from './operationalScope';
