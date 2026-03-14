import React from 'react';
import clsx from 'clsx';

export interface TabItem {
  id: string;
  label: string;
  icon?: React.ReactNode;
}

export interface TabsProps {
  items: TabItem[];
  value: string;
  onChange: (id: string) => void;
  className?: string;
}

export const Tabs: React.FC<TabsProps> = ({ items, value, onChange, className = '' }) => {
  return (
    <div className={clsx('inline-flex flex-wrap gap-1 p-1 bg-slate-900/80 rounded-lg border border-slate-800 mb-6', className)}>
      {items.map(({ id, label, icon }) => (
        <button
          key={id}
          type="button"
          onClick={() => onChange(id)}
          className={clsx(
            'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors',
            value === id ? 'bg-pink-600 text-white shadow' : 'text-slate-400 hover:bg-slate-800 hover:text-white',
          )}
        >
          {icon}
          {label}
        </button>
      ))}
    </div>
  );
};

