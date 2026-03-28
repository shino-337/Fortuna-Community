import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Certificate, RotationEvent } from '../types';
import { Card } from '../components/ui/Card';
import { Lock, AlertCircle, CheckCircle, XCircle, RefreshCw, ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime } from '../lib/display';

export const Certificates: React.FC = () => {
  const navigate = useNavigate();
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [history, setHistory] = useState<RotationEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [rotating, setRotating] = useState(false);

  const fetchData = async () => {
    setLoading(true);
    const [certData, historyData] = await Promise.all([api.getCertificates(), api.getRotationHistory()]);
    setCerts(certData);
    setHistory(historyData);
    setLoading(false);
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
        return <CheckCircle className="w-5 h-5 text-emerald-500" />;
      case 'warning':
        return <AlertCircle className="w-5 h-5 text-amber-500" />;
      case 'expired':
        return <XCircle className="w-5 h-5 text-red-500" />;
      default:
        return <AlertCircle className="w-5 h-5 text-slate-500" />;
    }
  };

  const getStatusClass = (status?: string) => {
    switch (status) {
      case 'valid':
        return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
      case 'warning':
        return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
      case 'expired':
        return 'bg-red-500/10 text-red-400 border-red-500/20';
      default:
        return 'bg-slate-800 text-slate-400';
    }
  };

  return (
    <PageLayout
      title="Certificates"
      description="Core TLS certificate status and rotation history."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <Button variant="secondary" onClick={fetchData} isLoading={loading}>
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
          <Button variant="secondary" onClick={rotateCertificate} isLoading={rotating}>
            Rotate Certificate
          </Button>
        </div>
      }
    >
      {certs.length === 0 ? (
        <PageEmpty title="No certificate data available" description="Enable TLS/CertManager in Core to expose certificate information." />
      ) : (
        <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
          {certs.map((cert) => (
            <Card key={cert.id} className="relative overflow-hidden group hover:border-slate-600 transition-colors">
              <div className="absolute top-0 right-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity pointer-events-none">
                <Lock size={120} />
              </div>
              <div className="flex justify-between items-start mb-4">
                <div className="p-2.5 bg-slate-800 rounded-lg">{getStatusIcon(cert.status)}</div>
                <span className={`px-2.5 py-1 rounded-md text-xs font-medium border capitalize ${getStatusClass(cert.status)}`}>
                  {cert.status ?? 'unknown'}
                </span>
              </div>
              <h3 className="text-lg font-semibold text-white mb-1 truncate">{cert.name}</h3>
              <p className="text-sm text-slate-400 mb-4">Issuer: {cert.issuer || 'N/A'}</p>
              <div className="space-y-3 border-t border-slate-800 pt-4 mt-4 text-sm">
                <div className="flex justify-between">
                  <span className="text-slate-500">Expires</span>
                  <span className="font-mono text-slate-300">{formatDateTime(cert.expiryDate)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Days remaining</span>
                  <span className="text-slate-300">{cert.daysRemaining ?? 'N/A'}</span>
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
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                <tr>
                  <th className="px-4 py-3 font-medium">Time</th>
                  <th className="px-4 py-3 font-medium text-right">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {history.map((event) => (
                  <tr key={event.id} className="hover:bg-muted/30">
                    <td className="px-4 py-3 text-text font-mono">{formatDateTime(event.timestamp)}</td>
                    <td className="px-4 py-3 text-right">
                      <span className={`inline-flex items-center text-xs font-medium px-2 py-0.5 rounded border ${event.success ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-red-400 bg-red-500/10 border-red-500/20'}`}>
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