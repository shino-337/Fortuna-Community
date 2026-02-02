import React from 'react';

interface CardProps {
  children: React.ReactNode;
  className?: string;
  title?: string;
  description?: string;
  actions?: React.ReactNode;
}

export const Card: React.FC<CardProps> = ({ children, className = '', title, description, actions }) => {
  return (
    <div className={`bg-slate-900 rounded-lg border border-slate-800 shadow-sm ${className}`}>
      {(title || description || actions) && (
        <div className="px-6 py-4 border-b border-slate-800 flex justify-between items-start">
          <div>
            {title && <h3 className="text-lg font-semibold text-slate-50">{title}</h3>}
            {description && <p className="text-sm text-slate-400 mt-1">{description}</p>}
          </div>
          {actions && <div className="ml-4">{actions}</div>}
        </div>
      )}
      <div className="p-6">{children}</div>
    </div>
  );
};