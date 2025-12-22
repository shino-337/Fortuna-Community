import React from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';

export const AttackPaths: React.FC = () => {
  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900">Attack Path Analysis</h1>
        <Button>Export Graph</Button>
      </div>

      <Card>
        <div className="text-center py-12">
          <p className="text-gray-500 mb-4">Graph visualization coming soon...</p>
          <p className="text-sm text-gray-400">
            This will display interactive D3.js graph showing privilege escalation paths
          </p>
        </div>
      </Card>
    </div>
  );
};

