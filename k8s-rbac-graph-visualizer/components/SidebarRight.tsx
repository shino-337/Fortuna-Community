import React, { useState, useEffect } from 'react';
import { RbacNode } from '../types';
import { analyzeRbacNode } from '../services/geminiService';

interface SidebarRightProps {
  node: RbacNode | null;
}

const SidebarRight: React.FC<SidebarRightProps> = ({ node }) => {
  const [analysis, setAnalysis] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setAnalysis(null); // Reset analysis when node changes
  }, [node]);

  const handleAnalyze = async () => {
    if (!node) return;
    setLoading(true);
    const result = await analyzeRbacNode(node);
    setAnalysis(result);
    setLoading(false);
  };

  if (!node || !node.id) {
    return (
      <div className="w-80 bg-slate-800 border-l border-slate-700 p-6 text-slate-400 flex flex-col items-center justify-center text-center h-full">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-16 h-16 mb-4 opacity-30 text-blue-500">
          <path strokeLinecap="round" strokeLinejoin="round" d="M15.042 21.672L13.684 16.6m0 0l-2.51 2.225.569-9.47 5.227 7.917-3.286-.672zM12 2.25V4.5m5.834.166l-1.591 1.591M20.25 10.5H18M7.757 14.743l-1.59 1.59M6 10.5H3.75m4.007-4.243l-1.59-1.59" />
        </svg>
        <h3 className="text-lg font-medium text-slate-300 mb-2">No Node Selected</h3>
        <p className="text-sm">Click on a node in the graph to view its details, permissions, and security analysis.</p>
      </div>
    );
  }

  return (
    <div className="w-96 bg-slate-800 border-l border-slate-700 flex flex-col h-full overflow-hidden shadow-2xl z-20">
      {/* Header */}
      <div className="p-6 border-b border-slate-700 bg-slate-900/50">
        <div className="flex items-center justify-between mb-2">
           <span className="inline-block px-2 py-0.5 text-xs font-bold uppercase tracking-wide rounded bg-slate-700 text-slate-300 border border-slate-600">
            {node.type}
          </span>
        </div>
        <h2 className="text-xl font-bold text-white break-all leading-tight">{node.name}</h2>
        {node.namespace ? (
          <p className="text-sm text-slate-400 mt-2 flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-emerald-500"></span>
            Namespace: <span className="text-slate-200 font-mono">{node.namespace}</span>
          </p>
        ) : (
          <p className="text-sm text-slate-500 mt-2 italic">Cluster Scope</p>
        )}
      </div>

      {/* Scrollable Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6 custom-scrollbar">
        
        {/* Rules Section for Roles */}
        {(node.type === 'Role' || node.type === 'ClusterRole') && node.rules && (
          <div className="space-y-3">
            <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                  <path fillRule="evenodd" d="M10 1a4.5 4.5 0 00-4.5 4.5V9H5a2 2 0 00-2 2v6a2 2 0 002 2h10a2 2 0 002-2v-6a2 2 0 00-2-2h-.5V5.5A4.5 4.5 0 0010 1zm3 8V5.5a3 3 0 10-6 0V9h6z" clipRule="evenodd" />
                </svg>
                Permissions
            </h3>
            {node.rules.map((rule, idx) => (
              <div key={idx} className="bg-slate-700/30 p-3 rounded-md border border-slate-700/50 text-sm hover:border-slate-600 transition-colors">
                <div className="mb-2">
                    <span className="text-slate-500 text-xs font-semibold">RESOURCES</span>
                    <div className="text-sky-400 font-mono text-xs break-words mt-0.5">
                        {rule.resources.join(', ')}
                    </div>
                </div>
                <div>
                    <span className="text-slate-500 text-xs font-semibold">VERBS</span>
                    <div className="flex flex-wrap gap-1 mt-1">
                        {rule.verbs.map(v => (
                            <span key={v} className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${v === '*' ? 'bg-red-500/20 text-red-300 border border-red-500/30' : 'bg-slate-600 text-slate-300'}`}>
                                {v}
                            </span>
                        ))}
                    </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* AI Analysis */}
        <div className="border-t border-slate-700 pt-6">
            <div className="flex items-center justify-between mb-4">
                 <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider flex items-center gap-2">
                    <span className="text-purple-400 text-base">✨</span> Security Audit
                 </h3>
                 {!analysis && !loading && (
                    <button 
                        onClick={handleAnalyze}
                        className="text-xs bg-purple-600 hover:bg-purple-500 text-white px-3 py-1.5 rounded shadow-sm transition-all hover:scale-105 active:scale-95"
                    >
                        Generate Report
                    </button>
                 )}
            </div>
            
            {loading && (
                <div className="space-y-3 animate-pulse bg-slate-700/20 p-4 rounded-lg">
                    <div className="h-2 bg-slate-700 rounded w-3/4"></div>
                    <div className="h-2 bg-slate-700 rounded w-full"></div>
                    <div className="h-2 bg-slate-700 rounded w-5/6"></div>
                    <div className="h-2 bg-slate-700 rounded w-1/2"></div>
                </div>
            )}

            {analysis && (
                 <div className="bg-gradient-to-br from-purple-900/20 to-slate-900/50 border border-purple-500/30 p-4 rounded-lg shadow-sm">
                    <div className="text-sm text-slate-300 leading-relaxed whitespace-pre-line font-light">
                        {analysis}
                    </div>
                    <div className="mt-3 pt-3 border-t border-purple-500/20 flex justify-end">
                        <button onClick={() => setAnalysis(null)} className="text-xs text-purple-400 hover:text-purple-300 transition-colors">
                            Clear Result
                        </button>
                    </div>
                 </div>
            )}
        </div>

        {/* Metadata Dump */}
        <div className="pt-4">
           <details className="group">
             <summary className="list-none cursor-pointer text-xs font-mono text-slate-600 hover:text-slate-400 flex items-center gap-1">
               <span className="group-open:rotate-90 transition-transform">▶</span> RAW DATA
             </summary>
             <pre className="mt-2 bg-slate-950 p-3 rounded text-[10px] text-slate-500 font-mono overflow-x-auto border border-slate-900">
                {JSON.stringify(node, (key, val) => {
                    if (['x', 'y', 'vx', 'vy', 'index', 'fx', 'fy'].includes(key)) return undefined;
                    return val;
                }, 2)}
             </pre>
           </details>
        </div>

      </div>
    </div>
  );
};

export default SidebarRight;