import React from 'react';
import { RotateCcw } from 'lucide-react';
import { Button } from './ui/Button';

/** Clears every filter, search and sort on a list back to its defaults; disabled when nothing is set. */
export const ResetFiltersButton: React.FC<{ onReset: () => void; active: boolean; title?: string }> = ({
  onReset,
  active,
  title = 'Clear search, filters and sort',
}) => (
  <Button variant="secondary" size="sm" type="button" onClick={onReset} disabled={!active} title={title}>
    <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
    Reset filters
  </Button>
);
