// IDE fallback shims:
// In some environments, workspace lint runs without installed node_modules.
// These declarations keep TS language service usable for editing.

declare module 'react';
declare module 'react/jsx-runtime' {
  export const Fragment: any;
  export function jsx(type: any, props: any, key?: any): any;
  export function jsxs(type: any, props: any, key?: any): any;
}
declare module 'react-router-dom';
declare module 'lucide-react';
declare module 'recharts';
declare module 'd3';
