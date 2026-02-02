import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { Certificate, RotationEvent } from '../types';
import { Card } from '../components/ui/Card';
import { Lock, AlertCircle, CheckCircle, XCircle, ArrowRight } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Certificates: React.FC = () => {
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [history, setHistory] = useState<RotationEvent[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      const [certData, historyData] = await Promise.all([
        api.getCertificates(),
        api.getRotationHistory()
      ]);
      setCerts(certData);
      setHistory(historyData);
      setLoading(false);
    };
    fetchData();
  }, []);

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'valid': return <CheckCircle className="w-5 h-5 text-emerald-500" />;
      case 'warning': return <AlertCircle className="w-5 h-5 text-amber-500" />;
      case 'expired': return <XCircle className="w-5 h-5 text-red-500" />;
      default: return <AlertCircle className="w-5 h-5 text-slate-500" />;
    }
  };

  const getStatusClass = (status: string) => {
    switch (status) {
      case 'valid': return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
      case 'warning': return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
      case 'expired': return 'bg-red-500/10 text-red-400 border-red-500/20';
      default: return 'bg-slate-800 text-slate-400';
    }
  };

  return (
    <div className="space-y-8">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-white">Certificate Management</h1>
          <p className="text-slate-400">Monitor TLS/SSL certificate lifecycle and expiration.</p>
        </div>
        <Button variant="secondary">Rotate Certificate</Button>
      </div>

      <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
        {certs.map((cert) => (
          <Card key={cert.id} className="relative overflow-hidden group hover:border-slate-600 transition-colors">
            <div className="absolute top-0 right-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity pointer-events-none">
              <Lock size={120} />
            </div>
            
            <div className="flex justify-between items-start mb-4">
              <div className="p-2.5 bg-slate-800 rounded-lg">
                {getStatusIcon(cert.status)}
              </div>
              <span className={`px-2.5 py-1 rounded-md text-xs font-medium border capitalize ${getStatusClass(cert.status)}`}>
                {cert.status}
              </span>
            </div>

            <h3 className="text-lg font-semibold text-white mb-1 truncate" title={cert.name}>{cert.name}</h3>
            <p className="text-sm text-slate-400 mb-4">Issuer: {cert.issuer}</p>

            <div className="space-y-3 border-t border-slate-800 pt-4 mt-4">
              <div className="flex justify-between text-sm">
                <span className="text-slate-500">Expires</span>
                <span className={`font-mono ${cert.status === 'expired' ? 'text-red-400' : 'text-slate-300'}`}>
                    {cert.expiryDate}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-slate-500">Usage</span>
                <span className="text-slate-300 text-right">{cert.usage.join(', ')}</span>
              </div>
            </div>
            
            <div className="mt-4 pt-4 border-t border-slate-800">
                <button className="text-pink-500 text-sm font-medium hover:text-pink-400 flex items-center">
                    View Details <ArrowRight className="w-3 h-3 ml-1" />
                </button>
            </div>
          </Card>
        ))}
      </div>

      <div className="grid lg:grid-cols-3 gap-6">
          <Card className="lg:col-span-2" title="Rotation History">
              <div className="overflow-x-auto">
                  <table className="w-full text-sm text-left">
                      <thead className="text-xs text-slate-400 uppercase border-b border-slate-800">
                          <tr>
                              <th className="px-4 py-3 font-medium">Date</th>
                              <th className="px-4 py-3 font-medium">Action</th>
                              <th className="px-4 py-3 font-medium">Duration</th>
                              <th className="px-4 py-3 font-medium text-right">Status</th>
                          </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800">
                          {history.map(event => (
                              <tr key={event.id} className="hover:bg-slate-800/50">
                                  <td className="px-4 py-3 text-white font-mono">{event.date}</td>
                                  <td className="px-4 py-3 text-slate-300">{event.action}</td>
                                  <td className="px-4 py-3 text-slate-400">{event.duration}</td>
                                  <td className="px-4 py-3 text-right">
                                      <span className="inline-flex items-center text-emerald-400 text-xs font-medium bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20 uppercase">
                                          {event.status}
                                      </span>
                                  </td>
                              </tr>
                          ))}
                      </tbody>
                  </table>
                  <div className="mt-4 text-center">
                      <button className="text-sm text-slate-500 hover:text-white">View All History</button>
                  </div>
              </div>
          </Card>
          
          <Card title="Rotation Settings">
              <div className="space-y-6">
                  <div>
                      <label className="text-sm font-medium text-slate-300 block mb-3">Auto-rotation</label>
                      <div className="flex bg-slate-900 rounded-lg p-1 border border-slate-800">
                          <button className="flex-1 py-1.5 text-sm font-medium bg-slate-800 text-white rounded shadow-sm">Enabled</button>
                          <button className="flex-1 py-1.5 text-sm font-medium text-slate-500 hover:text-slate-300">Disabled</button>
                      </div>
                  </div>
                  <div>
                      <div className="flex justify-between text-sm mb-1">
                          <span className="text-slate-400">Rotation Interval</span>
                          <span className="text-white">60 days</span>
                      </div>
                      <div className="w-full bg-slate-800 rounded-full h-2">
                          <div className="bg-pink-600 h-2 rounded-full" style={{ width: '60%' }}></div>
                      </div>
                  </div>
                  <div className="space-y-2">
                      <div className="text-sm font-medium text-slate-300">Alerts</div>
                      <div className="flex items-center text-sm text-slate-400">
                          <CheckCircle className="w-4 h-4 text-emerald-500 mr-2" /> 30 days before expiry
                      </div>
                      <div className="flex items-center text-sm text-slate-400">
                          <CheckCircle className="w-4 h-4 text-emerald-500 mr-2" /> 7 days before expiry
                      </div>
                  </div>
                  <Button className="w-full">Save Settings</Button>
              </div>
          </Card>
      </div>
    </div>
  );
};