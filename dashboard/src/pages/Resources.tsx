import React, { useState } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';

type ResourceType = 'pods' | 'service-accounts' | 'roles' | 'role-bindings';

export const Resources: React.FC = () => {
  const [activeTab, setActiveTab] = useState<ResourceType>('pods');

  const tabs = [
    { id: 'pods' as ResourceType, name: 'Pods', icon: '📦' },
    { id: 'service-accounts' as ResourceType, name: 'Service Accounts', icon: '👤' },
    { id: 'roles' as ResourceType, name: 'Roles', icon: '🔐' },
    { id: 'role-bindings' as ResourceType, name: 'Role Bindings', icon: '🔗' },
  ];

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900">Resources</h1>
        <Button variant="secondary">Export CSV</Button>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`py-4 px-1 border-b-2 font-medium text-sm ${
                activeTab === tab.id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              <span className="mr-2">{tab.icon}</span>
              {tab.name}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      <Card>
        <div className="text-center py-12 text-gray-500">
          {activeTab.charAt(0).toUpperCase() + activeTab.slice(1)} view coming soon...
        </div>
      </Card>
    </div>
  );
};

