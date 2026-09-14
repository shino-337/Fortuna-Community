export interface BulkFindingResult {
  success_count: number;
  failed_count: number;
  action: string;
  errors?: Array<{ id: string; error: string }>;
}

/** Keep failed selections available for retry and never infer success from HTTP 200. */
export function summarizeBulkFindingResult(result: BulkFindingResult, requestedIds: string[]) {
  const requested = [...new Set(requestedIds)];
  const failedIds = [...new Set((result.errors ?? []).map(item => String(item.id)))].filter(id => requested.includes(id));
  const complete = result.failed_count === 0 && result.success_count === requested.length;
  const identifiedFailures = failedIds.length === result.failed_count
    && result.success_count + result.failed_count === requested.length;
  const remainingIds = complete ? [] : identifiedFailures ? failedIds : requested;
  const details = (result.errors ?? []).slice(0, 3).map(item => `${item.id}: ${item.error}`).join('; ');
  const message = complete
    ? `${result.success_count} finding(s) updated.`
    : `${result.success_count} finding(s) updated; ${result.failed_count} failed.${details ? ` ${details}` : ' Review the result before retrying.'}`;
  return { complete, remainingIds, message };
}
