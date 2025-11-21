import { GraphData, NodeType, RbacNode, RbacLink } from '../types';
import { NODE_RADIUS } from '../constants';

export const generateRbacData = (): GraphData => {
  const nodes: RbacNode[] = [];
  const links: RbacLink[] = [];

  const clusters = ['production', 'staging'];

  clusters.forEach(clusterName => {
      const prefix = (id: string) => `${clusterName}:${id}`;

      // 1. Create Subjects (Users, SAs)
      // Users are technically global, but often authenticated per cluster context. We'll verify them per cluster for viz.
      const users = ['alice@example.com', 'bob@devops.com'];
      if (clusterName === 'production') users.push('auditor@company.com');

      users.forEach((u) => {
        nodes.push({
          id: prefix(u),
          name: u,
          type: NodeType.USER,
          clusterName,
          radius: NODE_RADIUS[NodeType.USER],
        });
      });

      const serviceAccounts = [
        { name: 'default', ns: 'default' },
        { name: 'deployment-controller', ns: 'kube-system' },
      ];
      
      if (clusterName === 'staging') {
          serviceAccounts.push({ name: 'test-runner', ns: 'integration-tests' });
      }

      serviceAccounts.forEach((sa) => {
        const id = `system:serviceaccount:${sa.ns}:${sa.name}`;
        nodes.push({
          id: prefix(id),
          name: sa.name,
          namespace: sa.ns,
          clusterName,
          type: NodeType.SERVICE_ACCOUNT,
          radius: NODE_RADIUS[NodeType.SERVICE_ACCOUNT],
        });
      });

      // 2. Create Roles & ClusterRoles
      const clusterRoles = [
        { 
            name: 'cluster-admin', 
            rules: [{ apiGroups: ['*'], resources: ['*'], verbs: ['*'] }] 
        },
        { 
            name: 'view', 
            rules: [{ apiGroups: ['*'], resources: ['pods', 'services', 'deployments'], verbs: ['get', 'list', 'watch'] }] 
        },
      ];

      clusterRoles.forEach((cr) => {
        nodes.push({
          id: prefix(`cr:${cr.name}`),
          name: cr.name,
          type: NodeType.CLUSTER_ROLE,
          clusterName,
          rules: cr.rules,
          radius: NODE_RADIUS[NodeType.CLUSTER_ROLE],
        });
      });

      const roles = [
        { name: 'config-editor', ns: 'default', rules: [{ apiGroups: [''], resources: ['configmaps'], verbs: ['update', 'patch'] }] },
      ];
      
      if (clusterName === 'staging') {
           roles.push({ name: 'test-admin', ns: 'integration-tests', rules: [{ apiGroups: ['*'], resources: ['*'], verbs: ['*'] }] });
      }

      roles.forEach((r) => {
        nodes.push({
          id: prefix(`role:${r.ns}:${r.name}`),
          name: r.name,
          namespace: r.ns,
          clusterName,
          type: NodeType.ROLE,
          rules: r.rules,
          radius: NODE_RADIUS[NodeType.ROLE],
        });
      });

      // 3. Bindings
      // Alice is cluster-admin
      const binding1 = { id: prefix('crb:alice-admin'), name: 'alice-admin-binding', type: NodeType.CLUSTER_ROLE_BINDING };
      nodes.push({ ...binding1, clusterName, radius: NODE_RADIUS[NodeType.CLUSTER_ROLE_BINDING] });
      links.push({ source: prefix('alice@example.com'), target: binding1.id, type: 'subject_of' });
      links.push({ source: binding1.id, target: prefix('cr:cluster-admin'), type: 'binds_to' });

      // Bob is view-only
      const binding2 = { id: prefix('crb:bob-view'), name: 'bob-view-binding', type: NodeType.CLUSTER_ROLE_BINDING };
      nodes.push({ ...binding2, clusterName, radius: NODE_RADIUS[NodeType.CLUSTER_ROLE_BINDING] });
      links.push({ source: prefix('bob@devops.com'), target: binding2.id, type: 'subject_of' });
      links.push({ source: binding2.id, target: prefix('cr:view'), type: 'binds_to' });

      // SA deployment-controller gets view (just for example)
      const binding3 = { id: prefix('crb:sa-view'), name: 'system-view', type: NodeType.CLUSTER_ROLE_BINDING };
      nodes.push({ ...binding3, clusterName, radius: NODE_RADIUS[NodeType.CLUSTER_ROLE_BINDING] });
      links.push({ source: prefix('system:serviceaccount:kube-system:deployment-controller'), target: binding3.id, type: 'subject_of' });
      links.push({ source: binding3.id, target: prefix('cr:view'), type: 'binds_to' });

      // Staging specific bindings
      if (clusterName === 'staging') {
          const bindingTest = { id: prefix('rb:test-runner-admin'), name: 'test-runner-binding', namespace: 'integration-tests', type: NodeType.ROLE_BINDING };
          nodes.push({ ...bindingTest, clusterName, radius: NODE_RADIUS[NodeType.ROLE_BINDING] });
          links.push({ source: prefix('system:serviceaccount:integration-tests:test-runner'), target: bindingTest.id, type: 'subject_of' });
          links.push({ source: bindingTest.id, target: prefix('role:integration-tests:test-admin'), type: 'binds_to' });
      }
  });

  return { nodes, links };
};