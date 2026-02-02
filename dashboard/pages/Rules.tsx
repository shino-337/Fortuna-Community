import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { SecurityRule } from '../types';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { RefreshCw, Upload, Search, Filter, PlayCircle, Edit3, AlertTriangle, Lightbulb, CheckCircle } from 'lucide-react';
import clsx from 'clsx';

export const Rules: React.FC = () => {
  const [rules, setRules] = useState<SecurityRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'active' | 'disabled' | 'templates'>('active');
  const [expandedRule, setExpandedRule] = useState<string | null>(null);
  const [testMode, setTestMode] = useState(false);

  useEffect(() => {
    loadRules();
  }, []);

  const loadRules = async () => {
    setLoading(true);
    const data = await api.getRules();
    setRules(data);
    setLoading(false);
  };

  const getSeverityBadge = (severity: string) => {
    const styles = {
      critical: 'bg-red-500/10 text-red-400 border-red-500/20',
      high: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
      medium: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
      low: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
    };
    return <span className={`px-2 py-0.5 text-xs font-medium rounded border uppercase ${styles[severity as keyof typeof styles] || styles.low}`}>{severity}</span>;
  };

  const filteredRules = rules.filter(rule => {
      if (activeTab === 'active') return rule.enabled;
      if (activeTab === 'disabled') return !rule.enabled;
      return true; // templates
  });

  const toggleExpand = (id: string) => {
      setExpandedRule(expandedRule === id ? null : id);
      setTestMode(false);
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-white">Rules Management</h1>
          <p className="text-slate-400">Policy-as-Code detection rule configuration.</p>
        </div>
        <div className="flex space-x-2">
            <Button variant="secondary" isLoading={loading} onClick={loadRules}>
                <RefreshCw className="w-4 h-4 mr-2" /> Reload
            </Button>
            <Button variant="secondary">
                <Upload className="w-4 h-4 mr-2" /> Upload YAML
            </Button>
            <Button>+ New Rule</Button>
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
           <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg text-center">
               <div className="text-slate-400 text-xs uppercase font-medium">Active</div>
               <div className="text-white font-bold text-xl">{rules.filter(r => r.enabled).length}</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg text-center">
               <div className="text-slate-400 text-xs uppercase font-medium">Disabled</div>
               <div className="text-white font-bold text-xl">{rules.filter(r => !r.enabled).length}</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg text-center">
               <div className="text-slate-400 text-xs uppercase font-medium">Eval/Sec</div>
               <div className="text-emerald-400 font-bold text-xl">1,250</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg text-center">
               <div className="text-slate-400 text-xs uppercase font-medium">Avg Latency</div>
               <div className="text-blue-400 font-bold text-xl">12μs</div>
           </div>
      </div>

      <div className="flex flex-col sm:flex-row space-y-4 sm:space-y-0 justify-between items-center">
          <div className="flex bg-slate-900 p-1 rounded-lg border border-slate-800">
              {(['active', 'disabled', 'templates'] as const).map(tab => (
                  <button
                      key={tab}
                      onClick={() => setActiveTab(tab)}
                      className={`px-4 py-2 rounded-md text-sm font-medium capitalize transition-colors ${
                          activeTab === tab ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'
                      }`}
                  >
                      {tab}
                  </button>
              ))}
          </div>
          
          <div className="flex space-x-2 w-full sm:w-auto">
             <div className="relative w-full sm:w-64">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4" />
                <input 
                    type="text" 
                    placeholder="Search rules..." 
                    className="pl-9 pr-4 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none w-full"
                />
             </div>
             <button className="p-2 border border-slate-700 rounded-lg bg-slate-900 text-slate-400 hover:text-white">
                 <Filter className="w-4 h-4" />
             </button>
          </div>
      </div>

      <Card className="overflow-hidden p-0">
        <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
                <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                    <tr>
                        <th className="px-6 py-4 font-medium">ID</th>
                        <th className="px-6 py-4 font-medium">Name</th>
                        <th className="px-6 py-4 font-medium">Type</th>
                        <th className="px-6 py-4 font-medium">Severity</th>
                        <th className="px-6 py-4 font-medium">Matches</th>
                        <th className="px-6 py-4 font-medium">Status</th>
                        <th className="px-6 py-4 font-medium text-right">Actions</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                    {filteredRules.map(rule => (
                        <React.Fragment key={rule.id}>
                        <tr className={clsx("hover:bg-slate-800/50 transition-colors cursor-pointer", expandedRule === rule.id ? "bg-slate-800/50" : "")} onClick={() => toggleExpand(rule.id)}>
                            <td className="px-6 py-4 font-mono text-xs text-slate-500">{rule.id}</td>
                            <td className="px-6 py-4">
                                <div className="font-medium text-white">{rule.name}</div>
                                <div className="text-xs text-slate-500 mt-0.5">{rule.category}</div>
                            </td>
                            <td className="px-6 py-4 text-slate-400">{rule.type}</td>
                            <td className="px-6 py-4">{getSeverityBadge(rule.severity)}</td>
                            <td className="px-6 py-4 text-slate-300 font-mono">{rule.matches}</td>
                            <td className="px-6 py-4">
                                <div className="flex items-center">
                                    {rule.enabled ? (
                                        <span className="flex items-center text-emerald-400 text-xs font-medium">
                                            <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
                                            On
                                        </span>
                                    ) : (
                                        <span className="flex items-center text-slate-500 text-xs font-medium">
                                            <span className="w-2 h-2 rounded-full bg-slate-600 mr-2"></span>
                                            Off
                                        </span>
                                    )}
                                </div>
                            </td>
                            <td className="px-6 py-4 text-right">
                                <div className="flex justify-end space-x-2">
                                    <button className="text-pink-500 hover:text-pink-400 text-xs font-medium border border-pink-500/20 bg-pink-500/10 px-2 py-1 rounded">View</button>
                                </div>
                            </td>
                        </tr>
                        {expandedRule === rule.id && (
                            <tr>
                                <td colSpan={7} className="bg-slate-950 p-6 border-b border-slate-800 shadow-inner">
                                    <div className="grid lg:grid-cols-2 gap-8">
                                        <div className="space-y-6">
                                            <div>
                                                <h3 className="text-lg font-bold text-white mb-1">{rule.id}: {rule.name}</h3>
                                                <p className="text-slate-400 text-sm">{rule.description}</p>
                                            </div>
                                            
                                            <div className="space-y-2">
                                                <h4 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Detection Logic (CEL)</h4>
                                                <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-300 overflow-x-auto">
                                                    <pre>{rule.logic || "// No logic defined for this rule"}</pre>
                                                </div>
                                            </div>

                                            <div className="grid grid-cols-2 gap-4">
                                                <div className="bg-slate-900 p-3 rounded border border-slate-800">
                                                    <div className="text-xs text-slate-500 uppercase">Eval Time</div>
                                                    <div className="text-white font-mono">{rule.evalTime || '10μs'}</div>
                                                </div>
                                                <div className="bg-slate-900 p-3 rounded border border-slate-800">
                                                    <div className="text-xs text-slate-500 uppercase">Last Updated</div>
                                                    <div className="text-white font-mono">{rule.lastUpdated || '2023-11-01'}</div>
                                                </div>
                                            </div>
                                        </div>

                                        <div className="space-y-6">
                                            {/* Impact Preview */}
                                            <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
                                                <div className="px-4 py-3 border-b border-slate-800 bg-slate-800/30 flex items-center justify-between">
                                                    <div className="flex items-center text-sm font-semibold text-white">
                                                        <Lightbulb className="w-4 h-4 text-yellow-500 mr-2" />
                                                        Impact Preview
                                                    </div>
                                                    <span className="text-xs text-slate-500">If disabled</span>
                                                </div>
                                                <div className="p-4 space-y-4">
                                                     <div className="flex items-center justify-between text-sm">
                                                         <span className="text-slate-400">Current Risks Detected</span>
                                                         <span className="text-white font-mono">{rule.matches}</span>
                                                     </div>
                                                     <div className="flex items-center justify-between text-sm">
                                                         <span className="text-slate-400">Affected Resources</span>
                                                         <span className="text-white font-mono">{rule.matches}</span>
                                                     </div>
                                                     <div className="p-3 bg-red-900/10 border border-red-900/30 rounded text-xs text-red-300">
                                                         <strong className="block mb-1 flex items-center"><AlertTriangle className="w-3 h-3 mr-1"/> Consequence:</strong>
                                                         Disabling this rule will hide {rule.matches} critical risks. Cluster risk score will improve artificially (-13 pts) but security posture will degrade.
                                                     </div>
                                                </div>
                                            </div>

                                            {/* Test Interface (Toggle) */}
                                            {testMode ? (
                                                <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 animate-in fade-in slide-in-from-top-2">
                                                    <div className="flex justify-between items-center mb-3">
                                                        <h4 className="text-sm font-semibold text-white">Test Rule</h4>
                                                        <button onClick={() => setTestMode(false)} className="text-xs text-slate-500 hover:text-white">Close</button>
                                                    </div>
                                                    <textarea 
                                                        className="w-full h-32 bg-slate-950 border border-slate-700 rounded p-2 text-xs font-mono text-slate-300 mb-3"
                                                        defaultValue={`apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod`}
                                                    />
                                                    <div className="flex justify-between items-center">
                                                        <div className="text-xs text-emerald-400 font-medium flex items-center">
                                                            <CheckCircle className="w-3 h-3 mr-1" /> Match Expected
                                                        </div>
                                                        <Button size="sm">Run Test</Button>
                                                    </div>
                                                </div>
                                            ) : (
                                                 <div className="flex space-x-3">
                                                    <Button variant="secondary" className="flex-1" onClick={() => setTestMode(true)}>
                                                        <PlayCircle className="w-4 h-4 mr-2" /> Test Rule
                                                    </Button>
                                                    <Button variant="secondary" className="flex-1">
                                                        <Edit3 className="w-4 h-4 mr-2" /> Edit
                                                    </Button>
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                </td>
                            </tr>
                        )}
                        </React.Fragment>
                    ))}
                </tbody>
            </table>
        </div>
      </Card>
    </div>
  );
};
