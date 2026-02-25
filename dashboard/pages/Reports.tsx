import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { Report } from '../types';
import { Card } from '../components/ui/Card';
import { FileText, Download } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { PageEmpty } from '../components/PageEmpty';

export const Reports: React.FC = () => {
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
        <Button variant="secondary" onClick={exportCsv} disabled={reports.length === 0}>
          <Download className="w-4 h-4 mr-2" />
          Export CSV
        </Button>
      }
    >
      <Card className="overflow-hidden">
        {loading ? (
          <div className="p-8 text-slate-500">Loading reports...</div>
        ) : reports.length === 0 ? (
          <PageEmpty title="No reports available" description="No audit aggregate data returned by /api/v1/reports." className="py-8" />
        ) : (
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
              <tr>
                <th className="px-6 py-4 font-medium">Resource</th>
                <th className="px-6 py-4 font-medium">Action</th>
                <th className="px-6 py-4 font-medium text-right">Count</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {reports.map((report) => (
                <tr key={report.id} className="hover:bg-slate-800/50 transition-colors">
                  <td className="px-6 py-4 font-medium text-white flex items-center">
                    <FileText className="w-4 h-4 mr-3 text-slate-500" />
                    {report.resource}
                  </td>
                  <td className="px-6 py-4 text-slate-300">{report.action}</td>
                  <td className="px-6 py-4 text-right text-slate-200 font-mono">{report.count ?? 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </PageLayout>
  );
};