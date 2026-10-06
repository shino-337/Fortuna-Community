import React, { useEffect, useRef } from 'react';
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { PageContract } from '../components/PageContract';
import { PageEmpty } from '../design-system/components/PageStatus';
import { Button } from '../components/ui/Button';
import { useInvestigationCases } from '../hooks/useInvestigationCases';
import { CaseList } from './cases/CaseList';
import { CaseDetail } from './cases/CaseDetail';

/** Cases: the list at /investigation, one case at /investigation?case=<id>. */
export const Investigation: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const caseId = searchParams.get('case');
  const { cases, loading, error, canRead, canWrite, canDelete, setActiveCase, refresh, createCase, updateCase, deleteCase } =
    useInvestigationCases();
  const loadedOnce = useRef(false);
  if (!loading) loadedOnce.current = true;

  const activeCase = caseId ? cases.find((c) => c.id === caseId) ?? null : null;

  useEffect(() => {
    if (activeCase) setActiveCase(activeCase.id);
  }, [activeCase?.id, setActiveCase]);

  if (!canRead) return <Navigate to="/" replace />;

  return (
    <PageContract feature="investigation" loading={loading && !loadedOnce.current}>
      {caseId ? (
        activeCase ? (
          <CaseDetail
            key={activeCase.id}
            activeCase={activeCase}
            canWrite={canWrite}
            canDelete={canDelete}
            onUpdate={(patch) => updateCase(activeCase.id, patch)}
            onArchive={async () => {
              await deleteCase(activeCase.id);
              navigate('/investigation');
            }}
          />
        ) : (
          <PageEmpty
            title="Case not found"
            description="It was archived, or it belongs to someone else. Cases are visible to their owner and creator."
            action={
              <Button size="sm" variant="secondary" onClick={() => navigate('/investigation')}>
                All cases
              </Button>
            }
          />
        )
      ) : (
        <CaseList
          cases={cases}
          error={error}
          canWrite={canWrite}
          onRetry={() => void refresh()}
          onCreate={(title) => createCase({ title })}
        />
      )}
    </PageContract>
  );
};
