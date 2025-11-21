import React from 'react';
import { FilterState, NodeType } from '../types';
import { LEGEND_ITEMS } from '../constants';

interface SidebarLeftProps {
  collapsed: boolean;
  setCollapsed: (v: boolean) => void;
  filters: FilterState;
  setFilters: React.Dispatch<React.SetStateAction<FilterState>>;
  availableClusters: string[];
  availableNamespaces: string[];
}

const SidebarLeft: React.FC<SidebarLeftProps> = ({ 
    collapsed, 
    setCollapsed, 
    filters, 
    setFilters,
    availableClusters,
    availableNamespaces
}) => {
  
  const toggleNodeType = (type: NodeType) => {
    setFilters(prev => ({
      ...prev,
      nodeTypes: { ...prev.nodeTypes, [type]: !prev.nodeTypes[type] }
    }));
  };

  const toggleCluster = (cluster: string) => {
    setFilters(prev => ({
        ...prev,
        clusters: { ...prev.clusters, [cluster]: !prev.clusters[cluster] }
    }));
  };

  const toggleNamespace = (ns: string) => {
      setFilters(prev => ({
          ...prev,
          namespaces: { ...prev.namespaces, [ns]: !prev.namespaces[ns] }
      }));
  };

  return (
    <div className={`bg-slate-800 border-r border-slate-700 flex flex-col transition-all duration-300 ease-in-out ${collapsed ? 'w-12' : 'w-72'}`}>
      {/* Header / Toggle */}
      <div className="h-14 flex items-center justify-between px-4 border-b border-slate-700 shrink-0">
        {!collapsed && <h2 className="text-slate-100 font-bold text-lg">Filters</h2>}
        <button 
          onClick={() => setCollapsed(!collapsed)}
          className="p-1 rounded hover:bg-slate-700 text-slate-400"
        >
          {collapsed ? (
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
              <path strokeLinecap="round" strokeLinejoin="round" d="M11.25 4.5l7.5 7.5-7.5 7.5m-6-15l7.5 7.5-7.5 7.5" />
            </svg>
          ) : (
             <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
              <path strokeLinecap="round" strokeLinejoin="round" d="M18.75 19.5l-7.5-7.5 7.5-7.5m-6 15L5.25 12l7.5-7.5" />
            </svg>
          )}
        </button>
      </div>

      {/* Content */}
      {!collapsed && (
        <div className="p-4 flex-1 overflow-y-auto custom-scrollbar">
          
          {/* Clusters */}
          <div className="mb-6">
            <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Clusters</h3>
            <div className="space-y-2">
                {availableClusters.map(cluster => (
                    <label key={cluster} className="flex items-center justify-between cursor-pointer hover:bg-slate-700/50 p-1 rounded">
                        <span className="text-sm text-slate-300 truncate mr-2" title={cluster}>{cluster}</span>
                        <input 
                            type="checkbox"
                            checked={!!filters.clusters[cluster]}
                            onChange={() => toggleCluster(cluster)}
                            className="rounded bg-slate-700 border-slate-600 text-blue-600 focus:ring-blue-500 focus:ring-offset-slate-800"
                        />
                    </label>
                ))}
            </div>
          </div>

          {/* Namespaces */}
          <div className="mb-6">
            <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Namespaces</h3>
            <div className="space-y-2 max-h-48 overflow-y-auto pr-2 custom-scrollbar">
                {availableNamespaces.map(ns => (
                    <label key={ns} className="flex items-center justify-between cursor-pointer hover:bg-slate-700/50 p-1 rounded">
                        <span className="text-sm text-slate-300 truncate mr-2" title={ns}>{ns}</span>
                        <input 
                            type="checkbox"
                            checked={!!filters.namespaces[ns]}
                            onChange={() => toggleNamespace(ns)}
                            className="rounded bg-slate-700 border-slate-600 text-blue-600 focus:ring-blue-500 focus:ring-offset-slate-800"
                        />
                    </label>
                ))}
            </div>
          </div>

          {/* Node Types */}
          <div className="mb-6">
            <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Node Types</h3>
            <div className="space-y-2">
              {LEGEND_ITEMS.map(item => (
                <div key={item.type} className="flex items-center justify-between cursor-pointer hover:bg-slate-700/50 p-1 rounded" onClick={() => toggleNodeType(item.type)}>
                  <div className="flex items-center gap-2">
                    <span 
                      className="w-3 h-3 rounded-full shadow-sm" 
                      style={{ backgroundColor: item.color }} 
                    />
                    <span className="text-sm text-slate-300">{item.label}</span>
                  </div>
                  <input 
                    type="checkbox"
                    checked={filters.nodeTypes[item.type]}
                    readOnly
                    className="rounded bg-slate-700 border-slate-600 text-blue-600 focus:ring-blue-500 focus:ring-offset-slate-800"
                  />
                </div>
              ))}
            </div>
          </div>

          {/* Stats */}
          <div className="mt-auto pt-6 border-t border-slate-700">
             <div className="bg-slate-700/50 rounded p-3 text-xs text-slate-400">
               <p>Tip: Drag nodes to rearrange.</p>
               <p className="mt-1">Scroll to zoom in/out.</p>
             </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default SidebarLeft;