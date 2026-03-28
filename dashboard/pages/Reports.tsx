import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Report } from '../types';
import { Card } from '../components/ui/Card';
import { FileText, Download, ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty } from '../components/PageEmpty';

export const Reports: React.FC = () => {
  const navigate = useNavigate();
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getReports().then((data) => {
      setReports(data);
      setLoading(false);
    });
  }, []);

  const exportCsv = () => {
    const header = ['resource', 'action', 'count'];
    const rows = reports.map((r) => [r.resource || '', r.action || '', String(r.count ?? 0)]);
    const content = [header, ...rows].map((row) => row.map((v) => `"${String(v).replace(/"/g, '""')}"`).join(',')).join('\n');
    const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `audit-reports-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <PageLayout
      title="Reports"
      description="Database-backed audit report aggregates."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <Button variant="secondary" onClick={exportCsv} disabled={reports.length === 0}>
            <Download className="w-4 h-4 mr-2" />
            Export CSV
          </Button>
        </div>
      }
    >
      <Card className="p-0 overflow-hidden">
        {loading ? (
          <div className="p-8 text-muted">Loading reports...</div>
        ) : reports.length === 0 ? (
          <PageEmpty title="No reports available" description="No audit aggregate data returned by /api/v1/reports." className="py-8" />
        ) : (
          <div className="ui-table-scroll">
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                <tr>
                  <th className="px-6 py-4 font-medium">Resource</th>
                  <th className="px-6 py-4 font-medium">Action</th>
                  <th className="px-6 py-4 font-medium text-right">Count</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {reports.map((report) => (
                  <tr key={report.id} className="hover:bg-muted/30 transition-colors">
                    <td className="px-6 py-4 font-medium text-text flex items-center">
                      <FileText className="w-4 h-4 mr-3 text-muted" />
                      {report.resource}
                    </td>
                    <td className="px-6 py-4 text-muted">{report.action}</td>
                    <td className="px-6 py-4 text-right text-text font-mono">{report.count ?? 0}</td>
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