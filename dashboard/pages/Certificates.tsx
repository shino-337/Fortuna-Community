import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Certificate, RotationEvent } from '../types';
import { usePermUser } from '../hooks/usePermUser';
import { ACTION_IDS, canRunAction } from '../lib/actionAccess';
import { Card } from '../design-system/components/Card';
import { Lock, AlertCircle, CheckCircle, XCircle, RefreshCw, ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { formatDateTime } from '../lib/display';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';
import { PAGE_TITLES } from '../lib/pageTitles';
import { DataFreshness } from '../components/DataFreshness';

export const Certificates: React.FC = () => {
  const navigate = useNavigate();
  const permUser = usePermUser();
  const canRotateClusterCert = canRunAction(permUser, ACTION_IDS.certificateRotate);
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [history, setHistory] = useState<RotationEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [rotating, setRotating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const fetchData = async () => {
    setLoading(true);
    try {
      const [certData, historyData] = await Promise.all([api.getCertificates(), api.getRotationHistory()]);
      setCerts(certData);
      setHistory(historyData);
      setError(null);
      setUpdatedAt(new Date());
    } catch {
      setError('Certificate data could not be refreshed.');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const rotateCertificate = async () => {
    setRotating(true);
    try {
      await api.rotateCertificate();
      await fetchData();
    } finally {
      setRotating(false);
    }
  };

  const getStatusIcon = (status?: string) => {
    switch (status) {
      case 'valid':
        return <CheckCircle className="w-5 h-5 text-success" />;
      case 'warning':
        return <AlertCircle className="w-5 h-5 text-warning" />;
      case 'expired':
        return <XCircle className="w-5 h-5 text-critical" />;
      default:
        return <AlertCircle className="w-5 h-5 text-muted" />;
    }
  };

  const getStatusClass = (status?: string) => {
    switch (status) {
      case 'valid':
        return 'bg-success/10 text-success border-success/20';
      case 'warning':
        return 'bg-warning/10 text-warning border-warning/20';
      case 'expired':
        return 'bg-critical/10 text-critical border-critical/20';
      default:
        return 'bg-surface-2 text-muted';
    }
  };

  return (
    <PageLayout
      title={PAGE_TITLES.certificates}
      description="Core TLS certificate status and rotation history."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <DataFreshness updatedAt={updatedAt} loading={loading} error={error} />
          <Button variant="secondary" onClick={fetchData} isLoading={loading}>
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
          {canRotateClusterCert ? (
            <Button variant="secondary" onClick={rotateCertificate} isLoading={rotating}>
              Rotate Certificate
            </Button>
          ) : null}
        </div>
      }
    >
      {error && certs.length === 0 ? (
        <PageError
          title="Could not load certificates"
          description="Certificate status is unavailable. Retry when Core certificate APIs are reachable."
          action={<Button variant="secondary" onClick={fetchData} isLoading={loading}>Retry certificates</Button>}
        />
      ) : certs.length === 0 ? (
        <PageEmpty title="No certificate data available" description="Enable TLS/CertManager in Core to expose certificate information." />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {certs.map((cert) => (
            <Card key={cert.id} className="relative overflow-hidden group hover:border-border transition-colors">
              <div className="absolute top-0 right-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity pointer-events-none">
                <Lock size={120} />
              </div>
              <div className="flex justify-between items-start mb-4">
                <div className="p-2.5 bg-surface-2 rounded-lg">{getStatusIcon(cert.status)}</div>
                <span className={`px-2.5 py-1 rounded-md text-caption font-medium border capitalize ${getStatusClass(cert.status)}`}>
                  {cert.status ?? 'unknown'}
                </span>
              </div>
              <h3 className="mb-1 break-words text-section-title text-text">{cert.name}</h3>
              <p className="text-body text-muted mb-4">Issuer: {cert.issuer || 'N/A'}</p>
              <div className="space-y-3 border-t border-border pt-4 mt-4 text-body">
                <div className="flex justify-between">
                  <span className="text-muted">Expires</span>
                  <span className="font-mono text-text">{formatDateTime(cert.expiryDate)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted">Days remaining</span>
                  <span className="text-text">{cert.daysRemaining ?? 'N/A'}</span>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Card className="mt-6 p-0 overflow-hidden" title="Rotation History">
        {history.length === 0 ? (
          <PageEmpty title="No rotation history records" description="Rotation history table is not populated yet." className="py-8" />
        ) : (
        <div className="ui-table-scroll">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH}>Time</th>
                  <th className={`${UI_TH} text-right`}>Status</th>
                </tr>
              </thead>
              <tbody>
                {history.map((event) => (
                  <tr key={event.id} className={UI_TR}>
                    <td className={`${UI_TD} font-mono text-text`}>{formatDateTime(event.timestamp)}</td>
                    <td className={`${UI_TD} text-right`}>
                      <span className={`inline-flex items-center text-caption font-medium px-2 py-0.5 rounded border ${event.success ? 'text-success bg-success/10 border-success/20' : 'text-critical bg-critical/10 border-critical/20'}`}>
                        {event.success ? 'success' : 'failed'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </PageLayout>
  );
};
