import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { Report } from '../types';
import { Card } from '../components/ui/Card';
import { FileText, Download, Clock, PlayCircle } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Reports: React.FC = () => {
  const [reports, setReports] = useState<Report[]>([]);

  useEffect(() => {
    api.getReports().then(setReports);
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-white">Compliance Reports</h1>
          <p className="text-slate-400">Generate and download security and audit reports.</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        {[
          { title: 'CIS Benchmark', desc: 'Standard Kubernetes security benchmark', icon: <FileText className="text-blue-500" /> },
          { title: 'PCI-DSS Compliance', desc: 'Credit card data security standards', icon: <FileText className="text-pink-500" /> },
          { title: 'System Audit', desc: 'Complete system activity log', icon: <FileText className="text-emerald-500" /> },
        ].map((template, i) => (
          <Card key={i} className="hover:border-slate-700 transition-colors">
            <div className="flex flex-col h-full">
              <div className="p-3 bg-slate-800 w-fit rounded-lg mb-4">
                {template.icon}
              </div>
              <h3 className="font-semibold text-white text-lg">{template.title}</h3>
              <p className="text-slate-400 text-sm mt-2 flex-1">{template.desc}</p>
              <Button className="mt-6 w-full" variant="secondary">
                <PlayCircle className="w-4 h-4 mr-2" />
                Generate New
              </Button>
            </div>
          </Card>
        ))}
      </div>

      <h2 className="text-lg font-semibold text-white mb-4">Report History</h2>
      <Card className="overflow-hidden">
        <table className="w-full text-sm text-left">
          <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
            <tr>
              <th className="px-6 py-4 font-medium">Report Name</th>
              <th className="px-6 py-4 font-medium">Type</th>
              <th className="px-6 py-4 font-medium">Generated Date</th>
              <th className="px-6 py-4 font-medium">Status</th>
              <th className="px-6 py-4 font-medium text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800">
            {reports.map((report) => (
              <tr key={report.id} className="hover:bg-slate-800/50 transition-colors">
                <td className="px-6 py-4 font-medium text-white flex items-center">
                  <FileText className="w-4 h-4 mr-3 text-slate-500" />
                  {report.title}
                </td>
                <td className="px-6 py-4 text-slate-400 capitalize">{report.type}</td>
                <td className="px-6 py-4 text-slate-400 flex items-center">
                  <Clock className="w-3 h-3 mr-2" />
                  {report.generatedAt}
                </td>
                <td className="px-6 py-4">
                  <span className={`
                    inline-flex items-center px-2 py-0.5 rounded text-xs font-medium capitalize
                    ${report.status === 'ready' ? 'text-emerald-400 bg-emerald-500/10' : ''}
                    ${report.status === 'processing' ? 'text-blue-400 bg-blue-500/10 animate-pulse' : ''}
                  `}>
                    {report.status}
                  </span>
                </td>
                <td className="px-6 py-4 text-right">
                  {report.status === 'ready' && (
                    <button className="text-pink-500 hover:text-pink-400 font-medium text-xs inline-flex items-center">
                      <Download className="w-3 h-3 mr-1" /> Download
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
};