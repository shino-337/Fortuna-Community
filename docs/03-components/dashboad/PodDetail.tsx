
import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import { K8sResource, PodSbom, Insight, Vulnerability } from '../types';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { 
  Box, Shield, Activity, Package, Key, Info, ChevronLeft, 
  ExternalLink, Terminal, Cpu, Share2, AlertTriangle, ShieldCheck,
  Search, Filter, Download, ExternalLink as ExternalLinkIcon, X,
  ShieldAlert, FileText, CheckCircle2
} from 'lucide-react';

export const PodDetail: React.FC = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [pod, setPod] = useState<K8sResource | null>(null);
  const [sbom, setSbom] = useState<PodSbom | null>(null);
  const [risks, setRisks] = useState<Insight[]>([]);
  const [activeTab, setActiveTab] = useState(searchParams.get('tab') || 'Overview');
  const [loading, setLoading] = useState(true);
  const [selectedVulnerability, setSelectedVulnerability] = useState<Vulnerability | null>(null);

  useEffect(() => {
    if (id) {
      const fetchData = async () => {
        setLoading(true);
        const [podData, sbomData, risksData] = await Promise.all([
          api.getPodById(id),
          api.getSbomByPodId(id),
          api.getRisksByResourceId(id)
        ]);
        setPod(podData || null);
        setSbom(sbomData || null);
        setRisks(risksData);
        setLoading(false);
      };
      fetchData();
    }
  }, [id]);

  if (loading) return <div className="p-8 text-center text-slate-500">Loading Pod investigation context...</div>;
  if (!pod) return <div className="p-8 text-center text-slate-500">Pod not found.</div>;

  const tabs = [
    { id: 'Overview', icon: <Info size={16} /> },
    { id: 'Security', icon: <Shield size={16} /> },
    { id: 'SBOM', icon: <Package size={16} /> },
    { id: 'RBAC', icon: <Key size={16} /> },
    { id: 'Runtime', icon: <Activity size={16} /> },
  ];

  const severityColor = pod.riskLevel === 'critical' ? 'text-red-500 bg-red-500/10 border-red-500/20' : 
                         pod.riskLevel === 'high' ? 'text-orange-500 bg-orange-500/10 border-orange-500/20' : 'text-emerald-500 bg-emerald-500/10 border-emerald-500/20';

  return (
    <div className="space-y-6 animate-in fade-in duration-500">
      <button onClick={() => navigate(-1)} className="flex items-center text-sm text-slate-400 hover:text-white transition-colors">
        <ChevronLeft size={16} className="mr-1" /> Back to Resources
      </button>

      {/* Object Header */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
        <div className="flex items-center space-x-4">
          <div className="p-4 bg-slate-900 border border-slate-800 rounded-2xl">
            <Box size={32} className="text-pink-500" />
          </div>
          <div>
            <div className="flex items-center space-x-3">
              <h1 className="text-3xl font-bold text-white tracking-tight">{pod.name}</h1>
              <span className={`px-3 py-0.5 rounded-full text-xs font-bold uppercase border ${severityColor}`}>
                {pod.riskLevel} Risk
              </span>
            </div>
            <div className="flex items-center mt-1 text-slate-500 text-sm font-mono">
              <span>{pod.namespace}</span>
              <span className="mx-2 opacity-30">|</span>
              <span>{pod.node}</span>
            </div>
          </div>
        </div>
        <div className="flex space-x-3">
          <Button variant="secondary" size="sm"><Terminal size={14} className="mr-2" /> Exec</Button>
          <Button variant="secondary" size="sm"><Share2 size={14} className="mr-2" /> Export</Button>
          <Button size="sm">Download YAML</Button>
        </div>
      </div>

      {/* Context Summary Row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Status</p>
          <div className="flex items-center text-emerald-400 font-bold">
            <ShieldCheck size={16} className="mr-2" /> {pod.status}
          </div>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Risk Score</p>
          <div className="text-xl font-bold text-white">{pod.riskScore}/100</div>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Service Account</p>
          <div className="text-sm font-bold text-slate-300 truncate">{pod.saName}</div>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Containers</p>
          <div className="text-xl font-bold text-white">1/1 Running</div>
        </Card>
      </div>

      {/* Tabs Layout */}
      <div className="border-b border-slate-800 flex space-x-8">
        {tabs.map(tab => (
          <button 
            key={tab.id} 
            onClick={() => setActiveTab(tab.id)}
            className={`pb-4 text-sm font-bold flex items-center transition-all ${activeTab === tab.id ? 'text-pink-500 border-b-2 border-pink-500' : 'text-slate-500 hover:text-slate-300'}`}
          >
            <span className="mr-2">{tab.icon}</span> {tab.id}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="min-h-[400px]">
        {activeTab === 'Overview' && (
          <div className="grid lg:grid-cols-2 gap-8 animate-in slide-in-from-bottom-2">
            <Card title="Container Metadata">
              <div className="space-y-4">
                <div className="p-3 bg-slate-950 rounded-lg border border-slate-800">
                  <p className="text-[10px] font-bold text-slate-500 uppercase mb-1">Image URI</p>
                  <p className="text-sm font-mono text-slate-300 break-all">{pod.image}</p>
                </div>
                <div className="grid grid-cols-2 gap-4">
                   <div className="p-3 bg-slate-950 rounded-lg border border-slate-800">
                    <p className="text-[10px] font-bold text-slate-500 uppercase mb-1">IP Address</p>
                    <p className="text-sm font-mono text-slate-300">{pod.ip}</p>
                  </div>
                  <div className="p-3 bg-slate-950 rounded-lg border border-slate-800">
                    <p className="text-[10px] font-bold text-slate-500 uppercase mb-1">Created</p>
                    <p className="text-sm font-mono text-slate-300">{pod.age}</p>
                  </div>
                </div>
              </div>
            </Card>
            <Card title="Labels">
              <div className="flex flex-wrap gap-2">
                {Object.entries(pod.labels || {}).map(([k, v]) => (
                  <span key={k} className="px-3 py-1 bg-slate-800 border border-slate-700 rounded-lg text-xs text-slate-400">
                    <span className="text-slate-300">{k}:</span> {v}
                  </span>
                ))}
              </div>
            </Card>
          </div>
        )}

        {activeTab === 'Security' && (
          <div className="space-y-6 animate-in slide-in-from-bottom-2">
            {risks.length > 0 ? (
              <div className="space-y-4">
                {risks.map(risk => (
                  <Card key={risk.id} className={`${risk.severity === 'critical' ? 'bg-red-500/5 border-red-500/20' : 'bg-orange-500/5 border-orange-500/20'}`}>
                    <div className="flex items-start">
                      <AlertTriangle className={`${risk.severity === 'critical' ? 'text-red-500' : 'text-orange-500'} w-6 h-6 mr-4 mt-1`} />
                      <div className="flex-1">
                        <div className="flex justify-between items-start">
                          <h3 className={`text-lg font-bold ${risk.severity === 'critical' ? 'text-red-500' : 'text-orange-500'}`}>{risk.title}</h3>
                          <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${risk.severity === 'critical' ? 'text-red-400 border-red-500/20 bg-red-500/10' : 'text-orange-400 border-orange-500/20 bg-orange-500/10'}`}>
                            {risk.severity}
                          </span>
                        </div>
                        <p className="text-slate-400 text-sm mt-1">{risk.description}</p>
                        <div className="mt-4 flex space-x-3">
                          <Button variant={risk.severity === 'critical' ? 'danger' : 'secondary'} size="sm" onClick={() => navigate(`/risks/${risk.id}`)}>Investigate</Button>
                          <Button variant="secondary" size="sm">Remediate</Button>
                        </div>
                      </div>
                    </div>
                  </Card>
                ))}
              </div>
            ) : (
              <Card className="bg-emerald-500/5 border-emerald-500/20">
                <div className="flex items-center">
                  <ShieldCheck className="text-emerald-500 w-6 h-6 mr-4" />
                  <div>
                    <h3 className="text-lg font-bold text-emerald-500">No Active Risks Detected</h3>
                    <p className="text-slate-400 text-sm">This pod currently meets all security policy requirements.</p>
                  </div>
                </div>
              </Card>
            )}
            
            <Card title="Security Context Details">
              <div className="divide-y divide-slate-800 text-sm">
                <div className="py-3 flex justify-between">
                  <span className="text-slate-500">Privileged</span>
                  <span className={`${pod.riskLevel === 'critical' ? 'text-red-400' : 'text-slate-400'} font-bold`}>
                    {pod.riskLevel === 'critical' ? 'true' : 'false'}
                  </span>
                </div>
                <div className="py-3 flex justify-between">
                  <span className="text-slate-500">Allow Privilege Escalation</span>
                  <span className="text-red-400 font-bold">true</span>
                </div>
                <div className="py-3 flex justify-between">
                  <span className="text-slate-500">Read Only Root Filesystem</span>
                  <span className="text-emerald-400 font-bold">false</span>
                </div>
                <div className="py-3 flex justify-between">
                  <span className="text-slate-500">Run as Non-Root</span>
                  <span className="text-slate-400 font-bold">false</span>
                </div>
              </div>
            </Card>
          </div>
        )}

        {activeTab === 'SBOM' && (
          <div className="space-y-6 animate-in slide-in-from-bottom-2">
            {/* SBOM Summary Stats */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <Card className="p-4 bg-slate-900/40 border-slate-800">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Total Components</p>
                    <div className="text-2xl font-bold text-white">{sbom?.components.length || 0}</div>
                  </div>
                  <div className="p-2 bg-pink-500/10 rounded-lg">
                    <Package size={20} className="text-pink-500" />
                  </div>
                </div>
              </Card>
              <Card className="p-4 bg-slate-900/40 border-slate-800">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Total Vulnerabilities</p>
                    <div className="text-2xl font-bold text-red-500">
                      {sbom?.components.reduce((acc, c) => acc + c.vulnerabilities.length, 0) || 0}
                    </div>
                  </div>
                  <div className="p-2 bg-red-500/10 rounded-lg">
                    <ShieldAlert size={20} className="text-red-500" />
                  </div>
                </div>
              </Card>
              <Card className="p-4 bg-slate-900/40 border-slate-800">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-1">Last Scan</p>
                    <div className="text-sm font-bold text-slate-300 mt-2">{sbom?.lastScan || 'Never'}</div>
                  </div>
                  <div className="p-2 bg-blue-500/10 rounded-lg">
                    <Activity size={20} className="text-blue-500" />
                  </div>
                </div>
              </Card>
            </div>

            <div className="flex gap-6 h-full">
              <div className={`space-y-6 transition-all duration-300 ${selectedVulnerability ? 'flex-1 min-w-0' : 'w-full'}`}>
                <div className="flex justify-between items-center">
                  <div className="relative w-64">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={14} />
                    <input 
                      type="text" 
                      placeholder="Filter packages..." 
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg py-2 pl-9 pr-4 text-xs text-white focus:outline-none focus:ring-1 focus:ring-pink-500"
                    />
                  </div>
                  <div className="flex space-x-2">
                    <Button variant="secondary" size="sm"><Filter size={14} className="mr-2" /> Filter</Button>
                    <Button variant="secondary" size="sm"><Download size={14} className="mr-2" /> CycloneDX</Button>
                  </div>
                </div>

                <Card className="p-0 overflow-hidden">
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm text-left">
                      <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                        <tr>
                          <th className="px-6 py-4 font-medium">Package Name</th>
                          <th className="px-6 py-4 font-medium">Version</th>
                          <th className="px-6 py-4 font-medium">Type</th>
                          <th className="px-6 py-4 font-medium">Vulnerabilities</th>
                          <th className="px-6 py-4 font-medium">Status</th>
                          <th className="px-6 py-4 font-medium">Fix Version</th>
                          <th className="px-6 py-4 font-medium text-right">Actions</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800">
                        {sbom?.components.map(comp => (
                          <tr key={comp.id} className={`hover:bg-slate-800/50 transition-colors ${comp.vulnerabilities.some(v => v.id === selectedVulnerability?.id) ? 'bg-pink-500/5' : ''}`}>
                            <td className="px-6 py-4">
                              <div className="flex items-center">
                                <Package size={14} className="mr-2 text-slate-500" />
                                <span className="font-medium text-white">{comp.name}</span>
                                {comp.language && <span className="ml-2 text-[10px] text-slate-500 uppercase">{comp.language}</span>}
                              </div>
                            </td>
                            <td className="px-6 py-4 text-slate-400 font-mono text-xs">{comp.version}</td>
                            <td className="px-6 py-4 text-slate-500">{comp.type}</td>
                            <td className="px-6 py-4">
                              {comp.vulnerabilities.length > 0 ? (
                                <div className="flex flex-wrap gap-1">
                                  {comp.vulnerabilities.map(v => (
                                    <button 
                                      key={v.id} 
                                      onClick={() => setSelectedVulnerability(selectedVulnerability?.id === v.id ? null : v)}
                                      className={`px-2 py-0.5 rounded text-[10px] font-bold border transition-colors ${selectedVulnerability?.id === v.id ? 'bg-pink-500 text-white border-pink-500' : 'bg-red-500/10 text-red-400 border-red-500/20 hover:bg-red-500/20'}`}
                                    >
                                      {v.id}
                                    </button>
                                  ))}
                                </div>
                              ) : (
                                <span className="text-emerald-500 text-xs flex items-center">
                                  <ShieldCheck size={12} className="mr-1" /> Clean
                                </span>
                              )}
                            </td>
                            <td className="px-6 py-4">
                              {comp.vulnerabilities.length > 0 ? (
                                <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${comp.vulnerabilities.some(v => v.status === 'unfixed') ? 'text-red-400 bg-red-500/10 border-red-500/20' : 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20'}`}>
                                  {comp.vulnerabilities.some(v => v.status === 'unfixed') ? 'Unfixed' : 'Fixed'}
                                </span>
                              ) : (
                                <span className="text-slate-500">-</span>
                              )}
                            </td>
                            <td className="px-6 py-4 text-slate-400 font-mono text-xs">
                              {comp.vulnerabilities.find(v => v.fixedVersion)?.fixedVersion || '-'}
                            </td>
                            <td className="px-6 py-4 text-right">
                              <button className="text-slate-500 hover:text-pink-500 transition-colors">
                                <ExternalLinkIcon size={14} />
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </Card>
              </div>

              {/* Side Panel for CVE Details - Unified with Sbom.tsx theme */}
              {selectedVulnerability && (
                <div className="w-[450px] shrink-0 animate-in slide-in-from-right-4 duration-300">
                  <Card className="h-full flex flex-col p-0 overflow-hidden border-pink-500/30 bg-slate-900/90 shadow-2xl">
                    <div className="flex justify-between items-center p-6 border-b border-slate-800 bg-slate-950/50">
                      <div className="flex items-center space-x-3">
                        <div className={`p-2.5 rounded-xl ${selectedVulnerability.severity === 'critical' ? 'bg-red-500/20 text-red-400' : 'bg-orange-500/20 text-orange-400'}`}>
                          <Shield size={20} />
                        </div>
                        <div>
                          <h3 className="text-xl font-bold text-white tracking-tight">{selectedVulnerability.id}</h3>
                          <div className="flex items-center mt-1">
                            <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${selectedVulnerability.severity === 'critical' ? 'bg-red-500/10 text-red-400 border-red-500/20' : 'bg-orange-500/10 text-orange-400 border-orange-500/20'}`}>
                              {selectedVulnerability.severity}
                            </span>
                          </div>
                        </div>
                      </div>
                      <button onClick={() => setSelectedVulnerability(null)} className="p-2 hover:bg-slate-800 rounded-full text-slate-500 hover:text-white transition-all">
                        <X size={20} />
                      </button>
                    </div>
                    
                    <div className="p-6 space-y-8 overflow-y-auto flex-1">
                      {/* CVSS Score Card */}
                      <div className="flex items-center justify-between p-4 bg-slate-950 rounded-2xl border border-slate-800 shadow-inner">
                        <div className="flex items-center">
                          <div className="mr-4">
                            <div className="text-[10px] text-slate-500 font-black uppercase tracking-widest mb-1">CVSS Score</div>
                            <div className={`text-3xl font-black ${selectedVulnerability.cvssScore >= 7 ? 'text-red-500' : selectedVulnerability.cvssScore >= 4 ? 'text-orange-500' : 'text-yellow-500'}`}>
                              {selectedVulnerability.cvssScore.toFixed(1)}
                            </div>
                          </div>
                          <div className="h-10 w-px bg-slate-800 mx-4"></div>
                          <div>
                            <div className="text-[10px] text-slate-500 font-black uppercase tracking-widest mb-1">Status</div>
                            <div className={`text-sm font-bold uppercase ${selectedVulnerability.status === 'unfixed' ? 'text-red-400' : 'text-emerald-400'}`}>
                              {selectedVulnerability.status || 'Unknown'}
                            </div>
                          </div>
                        </div>
                        <div className="p-3 bg-slate-900 rounded-xl">
                          <Activity size={24} className="text-slate-600" />
                        </div>
                      </div>
                      
                      {/* Description Section */}
                      <div>
                        <div className="flex items-center mb-3 text-[10px] font-black text-slate-500 uppercase tracking-widest">
                          <FileText size={14} className="mr-2 text-pink-500" />
                          Vulnerability Description
                        </div>
                        <div className="bg-slate-950/60 p-5 rounded-2xl border border-slate-800/50 italic leading-relaxed text-sm text-slate-300">
                          "{selectedVulnerability.description}"
                        </div>
                      </div>

                      {/* Remediation Section */}
                      {selectedVulnerability.fixedVersion ? (
                        <div className="p-5 bg-emerald-500/5 border border-emerald-500/20 rounded-2xl shadow-sm">
                          <div className="flex items-center text-emerald-400 mb-3">
                            <div className="p-1.5 bg-emerald-500/20 rounded-lg mr-3">
                              <CheckCircle2 size={16} className="text-emerald-500" />
                            </div>
                            <p className="text-[10px] font-black uppercase tracking-widest">Remediation Available</p>
                          </div>
                          <p className="text-sm text-slate-300 leading-relaxed">
                            A fix is available in version <span className="font-black font-mono text-emerald-300 px-2 py-0.5 bg-emerald-500/10 rounded border border-emerald-500/20">v{selectedVulnerability.fixedVersion}</span>. 
                            Update this component to resolve the security risk.
                          </p>
                        </div>
                      ) : (
                        <div className="p-5 bg-yellow-500/5 border border-yellow-500/20 rounded-2xl shadow-sm">
                          <div className="flex items-center text-yellow-500 mb-3">
                            <div className="p-1.5 bg-yellow-500/20 rounded-lg mr-3">
                              <Info size={16} className="text-yellow-500" />
                            </div>
                            <p className="text-[10px] font-black uppercase tracking-widest">No Fix Version Yet</p>
                          </div>
                          <p className="text-sm text-slate-300 leading-relaxed">
                            No official fix version has been recorded for this CVE yet. Monitor official security advisories.
                          </p>
                        </div>
                      )}

                      <div className="pt-6 space-y-3">
                        <Button className="w-full flex justify-center py-3" onClick={() => window.open(`https://nvd.nist.gov/vuln/detail/${selectedVulnerability.id}`, '_blank')}>
                          <ExternalLink size={16} className="mr-2" /> View NVD Details
                        </Button>
                        <Button variant="secondary" className="w-full py-3" onClick={() => setSelectedVulnerability(null)}>
                          Close Intelligence Panel
                        </Button>
                      </div>
                    </div>
                  </Card>
                </div>
              )}
            </div>
          </div>
        )}

        {['Runtime', 'RBAC'].includes(activeTab) && (
          <div className="flex flex-col items-center justify-center h-[300px] text-slate-600 border border-dashed border-slate-800 rounded-2xl bg-slate-900/20">
            <Cpu size={48} className="mb-4 opacity-20" />
            <p className="text-sm font-medium">Visualizing {activeTab} Data...</p>
            <p className="text-xs mt-1">Collecting telemetry from cluster agent.</p>
          </div>
        )}
      </div>
    </div>
  );
};


