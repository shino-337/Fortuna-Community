import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { Cluster, User, RiskRuleItem, RiskRuleFull, FortunaUserSession, SecurityActivityItem } from '../types';
import { api } from '../lib/api';
import { Shield, Plus, Pencil, Trash2, HelpCircle, CheckCircle, XCircle, FileDown, FileUp } from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { useConfirm } from '../design-system/components/ConfirmDialog';
import { Dialog } from '../design-system/components/Dialog';
import { useToast } from '../design-system/components/Toast';
import { formatDateTime } from '../lib/display';
import { getSeverityBadgeClass } from '../lib/severity';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH, UI_TR, UI_TD } from '../lib/tableChrome';
import {
  FORTUNA_ADMIN_VS_USER_ADMIN,
  FORTUNA_ROLE_HELP_ROWS,
  fortunaRoleSelectLabel,
  fortunaRoleShortLabel,
  fortunaRoleTooltip,
} from '../lib/fortunaRoles';
import { useAuthStore } from '../store/authStore';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { ACTION_IDS, canRunAction } from '../lib/actionAccess';
import { isPlatformAdmin } from '../lib/roles';
import { PAGE_TITLES } from '../lib/pageTitles';

const SEVERITIES = ['critical', 'high', 'medium', 'low', 'info'] as const;
const CATEGORIES = ['rbac', 'pod-security', 'network-policy', 'secrets', 'runtime-behavior', 'compliance'] as const;
const AGGREGATIONS = ['AND', 'OR', 'THRESHOLD'] as const;

function parseUserScopeClusters(user: User): { restricted: boolean; clusters: string[]; invalid: boolean } {
  if (user.operationalScope) {
    return {
      restricted: Boolean(user.operationalScope.restricted),
      clusters: Array.isArray(user.operationalScope.clusters) ? user.operationalScope.clusters : [],
      invalid: false,
    };
  }
  const raw = String(user.scopeJson || '').trim();
  if (raw === '' || raw === '{}') return { restricted: false, clusters: [], invalid: false };
  try {
    const parsed = JSON.parse(raw) as { clusters?: unknown; cluster_ids?: unknown };
    const list = Array.isArray(parsed.clusters) ? parsed.clusters : Array.isArray(parsed.cluster_ids) ? parsed.cluster_ids : [];
    return {
      restricted: list.length > 0,
      clusters: list.map((id) => String(id)).filter(Boolean),
      invalid: false,
    };
  } catch {
    return { restricted: true, clusters: [], invalid: true };
  }
}

function buildClusterScopeJson(mode: 'all' | 'selected', clusterIds: string[]): string {
  if (mode === 'all') return '{}';
  return JSON.stringify({ clusters: Array.from(new Set(clusterIds.filter(Boolean))).sort() });
}

function clusterDisplayName(cluster: Cluster): string {
  return cluster.name && cluster.name !== cluster.id ? `${cluster.name} (${cluster.id})` : cluster.id;
}

/** Validation and persistence are enforced on the server only. Client only sends payload and displays server errors. */

/** Predefined rule templates for quick start. */
const RISK_RULE_TEMPLATES: { name: string; rule: RiskRuleFull }[] = [
  {
    name: 'Cluster admin binding',
    rule: {
      id: 'cluster-admin-binding',
      name: 'ClusterAdmin binding',
      severity: 'critical',
      category: 'rbac',
      description: 'Detects RoleBinding/ClusterRoleBinding that grants cluster-admin.',
      enabled: true,
      conditions: [{ type: 'expression', expression: "roleRef.name == 'cluster-admin'" }],
      aggregation: 'AND',
      base_score: 9,
      tags: ['rbac', 'cluster-admin'],
    },
  },
  {
    name: 'Wildcard permissions',
    rule: {
      id: 'wildcard-permissions',
      name: 'Wildcard permissions',
      severity: 'high',
      category: 'rbac',
      description: 'Detects roles with * in resources or verbs.',
      enabled: true,
      conditions: [{ type: 'expression', expression: "hasWildcard(rules)" }],
      aggregation: 'AND',
      base_score: 7.5,
      tags: ['rbac', 'least-privilege'],
    },
  },
  {
    name: 'Orphan ServiceAccount',
    rule: {
      id: 'orphan-serviceaccount',
      name: 'Orphan ServiceAccount',
      severity: 'medium',
      category: 'rbac',
      description: 'ServiceAccount with no linked pods (unused).',
      enabled: true,
      conditions: [{ type: 'expression', expression: 'linkedPods == "[]"' }],
      aggregation: 'AND',
      base_score: 5,
      tags: ['rbac', 'cleanup'],
    },
  },
  {
    name: 'Empty form (custom)',
    rule: {
      id: '',
      name: '',
      severity: 'medium',
      description: '',
      category: 'rbac',
      enabled: true,
      conditions: [{ type: 'expression', expression: 'true' }],
      aggregation: 'AND',
      base_score: 7,
      tags: [],
    },
  },
];

export const Settings: React.FC = () => {
  const { user } = useAuthStore();
  const confirm = useConfirm();
  const toast = useToast();
  const permUser = usePermUser();
  const canUsers = can(permUser, P.usersRead);
  const canUsersUpdate = canRunAction(permUser, ACTION_IDS.userUpdate);
  const canUsersRoleAssign = canRunAction(permUser, ACTION_IDS.userRoleAssign);
  const canUsersCreate = canRunAction(permUser, ACTION_IDS.userCreate);
  const canUsersDelete = canRunAction(permUser, ACTION_IDS.userDelete);
  const canUsersRowActions = canUsersUpdate || canUsersRoleAssign || canUsersDelete;
  const canRegister = can(permUser, P.authRegister);
  const canRulesRead = can(permUser, P.rulesRead);
  const canRulesWrite = can(permUser, P.rulesWrite);
  const canRulesDelete = can(permUser, P.rulesDelete);
  const canRulesImport = can(permUser, P.rulesImport);
  const canRulesExport = can(permUser, P.rulesExport);
  const canGovAudit = can(permUser, P.systemAuditRead);
  const canSessionsRead = can(permUser, P.sessionsRead);
  const canSessionsRevoke = canRunAction(permUser, ACTION_IDS.sessionRevoke);
  const canSessionsRevokeAll = canRunAction(permUser, ACTION_IDS.sessionRevokeAll);
  const canUsersReadForSessions = can(permUser, P.usersRead);

  const isFortunaAdmin = isPlatformAdmin(user);
  const fortunaRoleEditOptions = useMemo(
    () =>
      isFortunaAdmin
        ? (['admin', 'cluster_admin', 'user_admin', 'operator', 'viewer'] as const)
        : (['user_admin', 'operator', 'viewer'] as const),
    [isFortunaAdmin],
  );

  const settingsTabs = useMemo(() => {
    const t: string[] = [];
    if (canUsers) t.push('Users');
    if (canSessionsRead) t.push('Sessions');
    if (canRulesRead) t.push('Risk Rules');
    return t;
  }, [canUsers, canSessionsRead, canRulesRead]);

  const [activeTab, setActiveTab] = useState(() => settingsTabs[0] ?? '');
  const [users, setUsers] = useState<User[]>([]);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const clusterById = useMemo(() => new Map(clusters.map((cluster) => [cluster.id, cluster])), [clusters]);
  const [riskRules, setRiskRules] = useState<RiskRuleItem[]>([]);
  const [riskRulesSource, setRiskRulesSource] = useState<string>('');
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [loadingClusters, setLoadingClusters] = useState(false);
  const [usersError, setUsersError] = useState<string | null>(null);
  const [savingUserId, setSavingUserId] = useState<string | null>(null);
  const [loadingRiskRules, setLoadingRiskRules] = useState(false);
  const [riskRuleModal, setRiskRuleModal] = useState<'add' | 'edit' | null>(null);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [formRule, setFormRule] = useState<RiskRuleFull>({
    id: '',
    name: '',
    severity: 'medium',
    description: '',
    category: 'rbac',
    enabled: true,
    conditions: [{ type: 'expression', expression: 'true' }],
    aggregation: 'AND',
    base_score: 7,
    tags: [],
  });
  const [formErrors, setFormErrors] = useState<string[]>([]);
  const [verifyResult, setVerifyResult] = useState<{ valid: boolean; errors: string[] } | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [saving, setSaving] = useState(false);
  const [importModalOpen, setImportModalOpen] = useState(false);
  const [importYaml, setImportYaml] = useState('');
  const [importErrors, setImportErrors] = useState<string[]>([]);
  const [importVerifyResult, setImportVerifyResult] = useState<{ valid: boolean; errors: string[] } | null>(null);
  const [verifyingImport, setVerifyingImport] = useState(false);
  const [importing, setImporting] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [addUserOpen, setAddUserOpen] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newUserRole, setNewUserRole] = useState('viewer');
  const [newUserScopeMode, setNewUserScopeMode] = useState<'all' | 'selected'>('all');
  const [newUserScopeClusters, setNewUserScopeClusters] = useState<string[]>([]);
  const [addUserError, setAddUserError] = useState('');
  const [addUserBusy, setAddUserBusy] = useState(false);
  const [scopeEditorUser, setScopeEditorUser] = useState<User | null>(null);
  const [scopeMode, setScopeMode] = useState<'all' | 'selected'>('all');
  const [selectedScopeClusters, setSelectedScopeClusters] = useState<string[]>([]);
  const [scopeSaving, setScopeSaving] = useState(false);
  const [scopeError, setScopeError] = useState('');

  const [govActivityItems, setGovActivityItems] = useState<SecurityActivityItem[]>([]);
  const [govActivityTotal, setGovActivityTotal] = useState(0);
  const [govActivityOffset, setGovActivityOffset] = useState(0);
  const govActivityLimit = 30;
  const [govLoading, setGovLoading] = useState(false);
  const [govSeverity, setGovSeverity] = useState('');
  const [govResult, setGovResult] = useState('');
  const [govAction, setGovAction] = useState('');
  const [sessionRows, setSessionRows] = useState<FortunaUserSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [sessionUserIdFilter, setSessionUserIdFilter] = useState('');
  const [revokingSessionId, setRevokingSessionId] = useState<string | null>(null);

  const loadRiskRules = useCallback(async () => {
    setLoadingRiskRules(true);
    try {
      const data = await api.getRiskRules();
      setRiskRules(data.rules);
      setRiskRulesSource(data.source || '');
    } catch {
      setRiskRules([]);
      setRiskRulesSource('');
    } finally {
      setLoadingRiskRules(false);
    }
  }, []);

  const loadUsers = useCallback(async () => {
    if (!canUsers) return;
    setLoadingUsers(true);
    setUsersError(null);
    try {
      const data = await api.getUsers();
      setUsers(data);
    } catch (e) {
      setUsers([]);
      setUsersError(e instanceof Error ? e.message : "Failed to load users");
    } finally {
      setLoadingUsers(false);
    }
  }, [canUsers]);

  const loadClustersForScope = useCallback(async () => {
    if (!isFortunaAdmin || !canUsers) return;
    setLoadingClusters(true);
    try {
      setClusters(await api.getClustersStats());
    } catch {
      setClusters([]);
    } finally {
      setLoadingClusters(false);
    }
  }, [canUsers, isFortunaAdmin]);

  const refreshUsersSilently = useCallback(async () => {
    try {
      const data = await api.getUsers();
      setUsers(data);
      setUsersError(null);
    } catch (e) {
      setUsersError(e instanceof Error ? e.message : "Failed to refresh users");
    }
  }, []);


  const loadGovernance = useCallback(async () => {
    if (canGovAudit) {
      setGovLoading(true);
      try {
        const r = await api.listSecurityActivity({
          limit: govActivityLimit,
          offset: govActivityOffset,
          severity: govSeverity.trim() || undefined,
          result: govResult.trim() || undefined,
          action: govAction.trim() || undefined,
        });
        setGovActivityItems(r.items);
        setGovActivityTotal(r.total);
      } catch (e) {
        setGovActivityItems([]);
        setGovActivityTotal(0);
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setGovLoading(false);
      }
    }
    if (canSessionsRead) {
      setSessionsLoading(true);
      try {
        const uid =
          isFortunaAdmin && canUsersReadForSessions && sessionUserIdFilter.trim() !== ''
            ? sessionUserIdFilter.trim()
            : undefined;
        setSessionRows(await api.listUserSessions(uid));
      } catch {
        setSessionRows([]);
      } finally {
        setSessionsLoading(false);
      }
    }
  }, [
    canGovAudit,
    canSessionsRead,
    govActivityOffset,
    govSeverity,
    govResult,
    govAction,
    isFortunaAdmin,
    canUsersReadForSessions,
    sessionUserIdFilter,
    govActivityLimit,
    toast,
  ]);

  const revokeFortunaSession = useCallback(
    async (sessionId: string) => {
      const confirmed = await confirm({
        title: 'Revoke session',
        description: 'That client must sign in again after this session is revoked.',
        confirmLabel: 'Revoke session',
        variant: 'danger',
      });
      if (!confirmed) return;
      setRevokingSessionId(sessionId);
      try {
        await api.revokeUserSession(sessionId);
        await loadGovernance();
      } catch (e) {
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setRevokingSessionId(null);
      }
    },
    [confirm, loadGovernance, toast],
  );

  const revokeAllMySessions = useCallback(async () => {
    if (!canSessionsRevoke) return;
    const confirmed = await confirm({
      title: 'Revoke all active sessions',
      description: 'You will need to sign in again on this browser after all active sessions are revoked.',
      confirmLabel: 'Revoke all sessions',
      variant: 'danger',
    });
    if (!confirmed) return;
    try {
      await api.revokeAllUserSessions();
      useAuthStore.getState().logout();
    } catch (e) {
      toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    }
  }, [canSessionsRevoke, confirm, toast]);

  const updateFortunaUser = useCallback(
    async (rowId: string, body: { role?: string; active?: boolean; scopeJson?: string }) => {
      setSavingUserId(rowId);
      try {
        await api.updateUser(rowId, body);
        await refreshUsersSilently();
      } catch (e) {
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setSavingUserId(null);
      }
    },
    [refreshUsersSilently, toast],
  );

  const openScopeEditor = useCallback((row: User) => {
    const parsed = parseUserScopeClusters(row);
    setScopeEditorUser(row);
    setScopeMode(parsed.restricted ? 'selected' : 'all');
    setSelectedScopeClusters(parsed.clusters);
    setScopeError(parsed.invalid ? 'Current scope JSON is invalid. Saving will replace it with the selected cluster scope.' : '');
  }, []);

  const saveScopeEditor = useCallback(async () => {
    if (!scopeEditorUser) return;
    setScopeError('');
    if (scopeMode === 'selected' && selectedScopeClusters.length === 0) {
      setScopeError('Select at least one cluster, or choose All clusters.');
      return;
    }
    setScopeSaving(true);
    try {
      await api.updateUser(scopeEditorUser.id, {
        scopeJson: buildClusterScopeJson(scopeMode, selectedScopeClusters),
      });
      await refreshUsersSilently();
      setScopeEditorUser(null);
      toast({ title: 'Cluster access updated', description: `${scopeEditorUser.username} scope was saved.`, variant: 'success' });
    } catch (e) {
      setScopeError(String(e instanceof Error ? e.message : e));
    } finally {
      setScopeSaving(false);
    }
  }, [refreshUsersSilently, scopeEditorUser, scopeMode, selectedScopeClusters, toast]);

  const removeFortunaUser = useCallback(
    async (rowId: string) => {
      setSavingUserId(rowId);
      try {
        await api.deleteUser(rowId);
        await refreshUsersSilently();
      } catch (e) {
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setSavingUserId(null);
      }
    },
    [refreshUsersSilently, toast],
  );

  const registerFortunaUser = useCallback(async () => {
    setAddUserError('');
    if (newUsername.trim().length < 3) {
      setAddUserError('Username must be at least 3 characters.');
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(newEmail.trim())) {
      setAddUserError('Valid email required.');
      return;
    }
    if (newPassword.length < 12) {
      setAddUserError('Password must be at least 12 characters.');
      return;
    }
    const newRoleIsAdmin = newUserRole.toLowerCase() === 'admin';
    if (isFortunaAdmin && !newRoleIsAdmin && newUserScopeMode === 'selected' && newUserScopeClusters.length === 0) {
      setAddUserError('Select at least one cluster, or choose All clusters.');
      return;
    }
    setAddUserBusy(true);
    try {
      await api.registerUser({
        username: newUsername.trim(),
        email: newEmail.trim(),
        password: newPassword,
        role: newUserRole,
        scopeJson: isFortunaAdmin && !newRoleIsAdmin ? buildClusterScopeJson(newUserScopeMode, newUserScopeClusters) : undefined,
      });
      setAddUserOpen(false);
      setNewUsername('');
      setNewEmail('');
      setNewPassword('');
      setNewUserRole('viewer');
      setNewUserScopeMode('all');
      setNewUserScopeClusters([]);
      await refreshUsersSilently();
    } catch (e) {
      setAddUserError(String(e instanceof Error ? e.message : e));
    } finally {
      setAddUserBusy(false);
    }
  }, [isFortunaAdmin, newUsername, newEmail, newPassword, newUserRole, newUserScopeMode, newUserScopeClusters, refreshUsersSilently]);

  useEffect(() => {
    if (activeTab === "Users" && canUsers) {
      void loadUsers();
      void loadClustersForScope();
    }
    if (activeTab === "Risk Rules" && canRulesRead) {
      loadRiskRules();
    }
  }, [activeTab, loadUsers, loadClustersForScope, loadRiskRules, canUsers, canRulesRead]);

  useEffect(() => {
    if (activeTab === 'Sessions' && canSessionsRead) {
      void loadGovernance();
    }
  }, [activeTab, canSessionsRead, loadGovernance]);

  useEffect(() => {
    if (settingsTabs.length === 0) return;
    if (!settingsTabs.includes(activeTab)) {
      setActiveTab(settingsTabs[0]);
    }
  }, [settingsTabs, activeTab]);

  const openAddRule = () => {
    setFormRule({
      id: '',
      name: '',
      severity: 'medium',
      description: '',
      category: 'rbac',
      enabled: true,
      conditions: [{ type: 'expression', expression: 'true' }],
      aggregation: 'AND',
      base_score: 7,
      tags: [],
    });
    setFormErrors([]);
    setVerifyResult(null);
    setEditingRuleId(null);
    setRiskRuleModal('add');
  };

  const openEditRule = async (id: string) => {
    setFormErrors([]);
    setVerifyResult(null);
    const full = await api.getRiskRule(id);
    if (full) {
      const idStr = typeof full.id === 'string' ? full.id : String(full.id ?? '');
      setFormRule({
        id: (full as { ruleId?: string }).ruleId ?? idStr,
        name: full.name,
        severity: full.severity || 'medium',
        description: full.description ?? '',
        category: full.category ?? 'rbac',
        enabled: full.enabled,
        conditions: full.conditions?.length ? full.conditions : [{ type: 'expression', expression: 'true' }],
        aggregation: full.aggregation ?? 'AND',
        base_score: full.base_score ?? 7,
        tags: full.tags ?? [],
      });
      setEditingRuleId((full as { ruleId?: string }).ruleId ?? idStr);
      setRiskRuleModal('edit');
    }
  };

  const applyTemplate = (template: RiskRuleFull) => {
    setFormRule({ ...template });
    setFormErrors([]);
    setVerifyResult(null);
  };

  const handleVerifyRule = async () => {
    setFormErrors([]);
    setVerifyResult(null);
    setVerifying(true);
    try {
      const result = await api.validateRiskRule(formRule);
      setVerifyResult(result);
      if (!result.valid) setFormErrors(result.errors);
    } catch {
      setVerifyResult({ valid: false, errors: ['Request failed. Try again.'] });
      setFormErrors(['Request failed. Try again.']);
    } finally {
      setVerifying(false);
    }
  };

  const saveRiskRule = async () => {
    setFormErrors([]);
    setSaving(true);
    try {
      const payload = { ...formRule, id: formRule.id || editingRuleId || '' };
      if (riskRuleModal === 'add') {
        await api.createRiskRule(payload);
      } else if (editingRuleId) {
        await api.updateRiskRule(editingRuleId, payload);
      }
      setRiskRuleModal(null);
      loadRiskRules();
    } catch (e: unknown) {
      const err = e as Error & { errors?: string[] };
      if (Array.isArray(err.errors) && err.errors.length > 0) {
        setFormErrors(err.errors);
      } else {
        setFormErrors([err.message || 'Save failed']);
      }
    } finally {
      setSaving(false);
    }
  };

  const deleteRiskRule = async (id: string) => {
    const confirmed = await confirm({
      title: 'Delete risk rule',
      description: 'Delete this rule from the database-backed rule catalog?',
      confirmLabel: 'Delete rule',
      variant: 'danger',
    });
    if (!confirmed) return;
    try {
      await api.deleteRiskRule(id);
      loadRiskRules();
    } catch (e) {
      toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    }
  };

  const downloadYaml = (yamlContent: string, filename: string) => {
    const blob = new Blob([yamlContent], { type: 'application/x-yaml' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const handleExportAll = async () => {
    if (riskRulesSource !== 'db') return;
    setExporting(true);
    try {
      const yaml = await api.getRiskRulesExportYaml();
      downloadYaml(yaml, 'risk-rules.yaml');
    } catch (e) {
      toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    } finally {
      setExporting(false);
    }
  };

  const handleExportOne = async (ruleId: string) => {
    setExporting(true);
    try {
      const yaml = await api.getRiskRulesExportYaml(ruleId);
      downloadYaml(yaml, `risk-rule-${ruleId}.yaml`);
    } catch (e) {
      toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    } finally {
      setExporting(false);
    }
  };

  const openImportModal = () => {
    setImportYaml('');
    setImportErrors([]);
    setImportVerifyResult(null);
    setImportModalOpen(true);
  };

  const handleVerifyImportYaml = async () => {
    setImportErrors([]);
    setImportVerifyResult(null);
    if (!importYaml.trim()) {
      setImportErrors(['Paste YAML content first.']);
      return;
    }
    setVerifyingImport(true);
    try {
      const result = await api.validateRiskRuleYaml(importYaml);
      setImportVerifyResult(result);
      if (!result.valid) setImportErrors(result.errors);
    } catch (e) {
      setImportVerifyResult({ valid: false, errors: [] });
      setImportErrors([String(e instanceof Error ? e.message : e)]);
    } finally {
      setVerifyingImport(false);
    }
  };

  const handleImportYaml = async () => {
    setImportErrors([]);
    if (!importYaml.trim()) {
      setImportErrors(['Paste YAML content first.']);
      return;
    }
    setImporting(true);
    try {
      await api.importRiskRuleYaml(importYaml);
      setImportModalOpen(false);
      loadRiskRules();
    } catch (e: unknown) {
      const err = e as Error & { errors?: string[] };
      setImportErrors(Array.isArray(err.errors) && err.errors.length > 0 ? err.errors : [err.message || 'Import failed']);
    } finally {
      setImporting(false);
    }
  };

  if (settingsTabs.length === 0) {
    return (
      <PageLayout title={PAGE_TITLES.settings} description="Fortuna administration">
        <PageEmpty
          title="No settings available"
          description="Your role does not include user, session, or risk rule administration permissions."
          className="py-10"
        />
      </PageLayout>
    );
  }

  return (
    <PageLayout
      title={PAGE_TITLES.settings}
      description="Fortuna administration: users, cluster access, sessions, and risk rules. Governance analytics live under Administration → Audit & Governance."
    >
      <div className="border-b border-border">
        <nav className="flex space-x-6 overflow-x-auto">
          {settingsTabs.map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`pb-4 text-body font-medium border-b-2 transition-colors whitespace-nowrap ${activeTab === tab ? 'border-brand text-brand' : 'border-transparent text-muted hover:text-text hover:border-border'}`}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>
      <div className="grid gap-6">
        {activeTab === 'Users' && (
          <>
            <Card className="p-4 sm:p-5 border border-border/90 bg-surface-2/25">
              <h3 className="mb-3 text-card-title text-text">Fortuna application roles</h3>
              <ul className="grid gap-3 sm:grid-cols-2 text-caption text-muted">
                {FORTUNA_ROLE_HELP_ROWS.map((row) => (
                  <li key={row.key} className="rounded-lg border border-border/70 bg-base/40 px-3 py-2.5">
                    <span className="font-semibold text-text">{row.title}</span>
                    <p className="mt-1.5 leading-snug">{row.body}</p>
                  </li>
                ))}
              </ul>
              <p className="text-caption text-muted mt-4 pt-3 border-t border-border leading-snug">
                <span className="font-medium text-text">Admin vs User admin:</span> {FORTUNA_ADMIN_VS_USER_ADMIN}
              </p>
            </Card>
            {canRegister && canUsersCreate && canUsers && (
              <div className="flex justify-end">
                <Button
                  type="button"
                  size="sm"
                  onClick={() => {
                    setAddUserError('');
                    setNewUserScopeMode('all');
                    setNewUserScopeClusters([]);
                    setAddUserOpen(true);
                  }}
                >
                  <Plus className="w-4 h-4 mr-2" /> Add user
                </Button>
              </div>
            )}
            <Card className="p-0 overflow-hidden">
            {loadingUsers ? (
              <div className="p-8 text-muted">Loading users...</div>
            ) : usersError ? (
              <PageError
                title="Could not load users"
                description={usersError}
                className="py-8"
                action={<Button type="button" variant="secondary" size="sm" onClick={() => void loadUsers()}>Retry</Button>}
              />
            ) : users.length === 0 ? (
              <PageEmpty title="No users" description="No user records returned by /api/v1/users." className="py-8" />
            ) : (
              <div className="ui-table-scroll">
                <table className={UI_TABLE}>
                  <thead className={UI_THEAD_STICKY}>
                    <tr>
                      <th className={UI_TH}>User</th>
                      <th className={UI_TH}>Fortuna role</th>
                      <th className={UI_TH}>Cluster access</th>
                      <th className={UI_TH}>Status</th>
                      {(canUsersRowActions) && <th className={`${UI_TH} text-right`}>Actions</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {users.map((u) => (
                      <tr key={u.id} className={UI_TR}>
                        <td className={UI_TD}>
                          <div className="text-text font-medium">{u.username || u.name || 'unknown'}</div>
                          <div className="text-muted text-caption">{u.email || 'N/A'}</div>
                        </td>
                        <td className={UI_TD}>
                          {(() => {
                            const rowRole = (u.role || 'viewer').toLowerCase();
                            const userAdminCannotEditThisRole =
                              !isFortunaAdmin && (rowRole === 'admin' || rowRole === 'cluster_admin');
                            if (canUsersRoleAssign && !userAdminCannotEditThisRole) {
                              const selVal = rowRole === 'user' ? 'operator' : rowRole;
                              return (
                                <select
                                  className="bg-base border border-border rounded px-2 py-1.5 text-body text-text min-w-[12rem] max-w-[20rem]"
                                  value={
                                    (fortunaRoleEditOptions as readonly string[]).includes(selVal)
                                      ? selVal
                                      : fortunaRoleEditOptions[0]
                                  }
                                  disabled={savingUserId === u.id}
                                  title="Fortuna application role (not Kubernetes RBAC)"
                                  onChange={(e) => {
                                    const next = e.target.value;
                                    void updateFortunaUser(u.id, { role: next });
                                  }}
                                >
                                  {fortunaRoleEditOptions.map((r) => (
                                    <option key={r} value={r}>
                                      {fortunaRoleSelectLabel(r)}
                                    </option>
                                  ))}
                                </select>
                              );
                            }
                            return (
                              <div
                                className="flex items-start gap-2 text-muted"
                                title={fortunaRoleTooltip(u.role)}
                              >
                                <Shield
                                  className={`w-3 h-3 mt-0.5 shrink-0 ${
                                    rowRole === 'admin'
                                      ? 'text-brand'
                                      : rowRole === 'cluster_admin'
                                        ? 'text-info'
                                      : rowRole === 'user_admin'
                                        ? 'text-warning'
                                        : 'text-muted'
                                  }`}
                                />
                                <span className="text-body text-text">{fortunaRoleShortLabel(u.role)}</span>
                              </div>
                            );
                          })()}
                        </td>
                        <td className={UI_TD}>
                          {(() => {
                            const parsed = parseUserScopeClusters(u);
                            const rowRole = (u.role || 'viewer').toLowerCase();
                            const clusterNames = parsed.clusters
                              .map((id) => clusterById.get(id)?.name || id)
                              .join(', ');
                            const adminUnrestricted = rowRole === 'admin';
                            const label = adminUnrestricted
                              ? 'All clusters'
                              : parsed.invalid
                                ? 'Invalid scope'
                                : parsed.restricted
                                  ? `${parsed.clusters.length} cluster${parsed.clusters.length === 1 ? '' : 's'}`
                                  : 'All clusters';
                            return (
                              <div className="flex flex-wrap items-center gap-2">
                                <span
                                  className={`inline-flex rounded-full border px-2 py-0.5 text-caption ${
                                    parsed.invalid
                                      ? 'border-critical/30 bg-critical/10 text-critical'
                                      : parsed.restricted
                                        ? 'border-info/30 bg-info/10 text-info'
                                        : 'border-border bg-surface-2/60 text-muted'
                                  }`}
                                  title={parsed.restricted ? clusterNames || 'Selected clusters' : 'Unrestricted cluster access'}
                                >
                                  {label}
                                </span>
                                {parsed.restricted && !parsed.invalid ? (
                                  <span className="max-w-[14rem] truncate text-caption text-muted" title={clusterNames}>
                                    {clusterNames || 'Selected cluster ids'}
                                  </span>
                                ) : null}
                                {isFortunaAdmin && !adminUnrestricted ? (
                                  <button
                                    type="button"
                                    className="rounded px-2 py-1 text-caption text-brand transition-colors hover:bg-brand/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 disabled:cursor-not-allowed disabled:opacity-50"
                                    disabled={savingUserId === u.id || loadingClusters}
                                    onClick={() => openScopeEditor(u)}
                                  >
                                    Edit scope
                                  </button>
                                ) : null}
                              </div>
                            );
                          })()}
                        </td>
                        <td className={UI_TD}>
                          {(() => {
                            const rowRole = (u.role || 'viewer').toLowerCase();
                            const userAdminCannotEditThisRow =
                              !isFortunaAdmin && (rowRole === 'admin' || rowRole === 'cluster_admin');
                            if (canUsersUpdate && !userAdminCannotEditThisRow) {
                              return (
                                <label className="inline-flex items-center gap-2 text-body text-text cursor-pointer select-none">
                                  <input
                                    type="checkbox"
                                    className="rounded border-border"
                                    checked={u.active !== false}
                                    disabled={savingUserId === u.id}
                                    onChange={(e) => {
                                      void updateFortunaUser(u.id, { active: e.target.checked });
                                    }}
                                  />
                                  <span>{u.active === false ? 'disabled' : 'active'}</span>
                                </label>
                              );
                            }
                            return (
                              <span
                                className={`px-2 py-0.5 rounded-full text-caption border capitalize ${
                                  u.active === false ? 'bg-muted/20 text-muted border-border' : 'bg-success/10 text-success border-success/20'
                                }`}
                              >
                                {u.status || (u.active === false ? 'disabled' : 'active')}
                              </span>
                            );
                          })()}
                        </td>
                        {(canUsersRowActions) && (
                          <td className={`${UI_TD} text-right`}>
                            {canUsersDelete && String(user?.id) !== u.id && (
                              <button
                                type="button"
                                className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-critical/60 disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-8 sm:min-w-8"
                                aria-label={`Delete user ${u.username || u.id}`}
                                disabled={savingUserId === u.id}
                                onClick={() => {
                                  void confirm({
                                    title: 'Delete Fortuna user',
                                    description: `Delete user ${u.username || u.id}? This removes their dashboard account access.`,
                                    confirmLabel: 'Delete user',
                                    variant: 'danger',
                                  }).then((confirmed) => {
                                    if (confirmed) void removeFortunaUser(u.id);
                                  });
                                }}
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            )}
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
          </>
        )}

        {activeTab === 'Risk Rules' && (
          <Card className="p-0 overflow-hidden">
            <div className="p-4 border-b border-border flex items-center justify-between flex-wrap gap-2">
              <span className="text-body text-muted">
                Source: <strong className="text-text">{riskRulesSource || '—'}</strong>
                {riskRulesSource && (
                  <span className="text-muted ml-1">({riskRules.length} rule{riskRules.length !== 1 ? 's' : ''})</span>
                )}
                {riskRulesSource === 'files' && ' — read-only from YAML; add rules in DB to edit here.'}
                {riskRulesSource === 'db' && ' — rules from database; new rules appear here after Save.'}
              </span>
              {riskRulesSource === 'db' && (
                <div className="flex items-center gap-2">
                  <Button size="sm" variant="secondary" onClick={handleExportAll} isLoading={exporting} disabled={riskRules.length === 0 || !canRulesExport}>
                    <FileDown className="w-4 h-4 mr-2" /> Export YAML
                  </Button>
                  <Button size="sm" variant="secondary" onClick={openImportModal} disabled={!canRulesImport}>
                    <FileUp className="w-4 h-4 mr-2" /> Import YAML
                  </Button>
                  <Button size="sm" onClick={openAddRule} disabled={!canRulesWrite}>
                    <Plus className="w-4 h-4 mr-2" /> Add rule
                  </Button>
                </div>
              )}
            </div>
            {loadingRiskRules ? (
              <div className="p-8 text-muted">Loading risk rules...</div>
            ) : riskRules.length === 0 ? (
              <PageEmpty
                title="No risk rules"
                description={riskRulesSource === 'db' ? 'Add a rule to evaluate resources. Rules are loaded by the engine at startup.' : 'Rules are loaded from files (FORTUNA_RULES_DIR). Migrate to DB to edit here.'}
                className="py-8"
              />
            ) : (
              <div className="ui-table-scroll">
                <table className={UI_TABLE}>
                  <thead className={UI_THEAD_STICKY}>
                    <tr>
                      <th className={UI_TH}>ID</th>
                      <th className={UI_TH}>Name</th>
                      <th className={UI_TH}>Severity</th>
                      <th className={UI_TH}>Category</th>
                      <th className={UI_TH}>Status</th>
                      {riskRulesSource === 'db' && <th className={`${UI_TH} text-right`}>Actions</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {riskRules.map((r) => (
                      <tr key={r.id} className={UI_TR}>
                      <td className={`${UI_TD} font-mono text-text`}>{r.id}</td>
                      <td className={`${UI_TD} text-text font-medium`}>{r.name}</td>
                      <td className={UI_TD}>
                        <span className={getSeverityBadgeClass(r.severity)}>{r.severity}</span>
                      </td>
                      <td className={`${UI_TD} text-muted`}>{r.category || '—'}</td>
                      <td className={UI_TD}>
                        <span className={r.enabled ? 'text-success' : 'text-muted'}>{r.enabled ? 'Enabled' : 'Disabled'}</span>
                      </td>
                      {riskRulesSource === 'db' && (
                        <td className={`${UI_TD} text-right`}>
                          <button
                            type="button"
                            onClick={() => handleExportOne(r.id)}
                            className="mr-2 rounded p-1 text-muted hover:text-info focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 disabled:cursor-not-allowed disabled:opacity-50"
                            title="Export as YAML"
                            aria-label={`Export ${r.name || r.id} as YAML`}
                            disabled={exporting || !canRulesExport}
                          >
                            <FileDown className="w-4 h-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => openEditRule(r.id)}
                            className="mr-2 rounded p-1 text-muted hover:text-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 disabled:cursor-not-allowed disabled:opacity-50"
                            title="Edit"
                            aria-label={`Edit ${r.name || r.id}`}
                            disabled={!canRulesWrite}
                          >
                            <Pencil className="w-4 h-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => deleteRiskRule(r.id)}
                            className="rounded p-1 text-muted hover:text-critical focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-critical/70 disabled:cursor-not-allowed disabled:opacity-50"
                            title="Delete"
                            aria-label={`Delete ${r.name || r.id}`}
                            disabled={!canRulesDelete}
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
              </div>
            )}
          </Card>
        )}

        {activeTab === 'Sessions' && (
          <>
            <Card className="p-4 sm:p-5 border border-border/90 bg-surface-2/25">
              <h3 className="mb-2 text-card-title text-text">Sessions</h3>
              <p className="text-caption text-muted leading-snug">
                Manage JWT-bound dashboard sessions. Authorization analytics, permission explorer, and audit review live under Administration → Audit & Governance.
              </p>
            </Card>

            {false && canGovAudit && (
              <Card className="p-0 overflow-hidden">
                <div className="p-4 border-b border-border space-y-3">
                  <div>
                    <h4 className="text-body font-semibold text-text">Security activity</h4>
                    <p className="text-caption text-muted mt-0.5">
                      Timeline from <span className="font-mono text-caption">GET /governance/security-activity</span> (immutable append-only store).
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-3 items-end">
                    <div>
                      <label className="text-caption text-muted block mb-1">Severity</label>
                      <select
                        className="bg-base border border-border rounded px-2 py-1.5 text-body min-w-[8rem]"
                        value={govSeverity}
                        onChange={(e) => {
                          setGovSeverity(e.target.value);
                          setGovActivityOffset(0);
                        }}
                      >
                        <option value="">Any</option>
                        {SEVERITIES.map((s) => (
                          <option key={s} value={s}>
                            {s}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="text-caption text-muted block mb-1">Result</label>
                      <select
                        className="bg-base border border-border rounded px-2 py-1.5 text-body min-w-[8rem]"
                        value={govResult}
                        onChange={(e) => {
                          setGovResult(e.target.value);
                          setGovActivityOffset(0);
                        }}
                      >
                        <option value="">Any</option>
                        <option value="success">success</option>
                        <option value="deny">deny</option>
                        <option value="error">error</option>
                      </select>
                    </div>
                    <div className="flex-1 min-w-[12rem]">
                      <label className="text-caption text-muted block mb-1">Action contains</label>
                      <input
                        className="w-full bg-base border border-border rounded px-2 py-1.5 text-body"
                        value={govAction}
                        onChange={(e) => {
                          setGovAction(e.target.value);
                          setGovActivityOffset(0);
                        }}
                        placeholder="e.g. graph_query"
                      />
                    </div>
                  </div>
                </div>
                {govLoading ? (
                  <div className="p-8 text-muted">Loading activity…</div>
                ) : govActivityItems.length === 0 ? (
                  <PageEmpty title="No events" description="No rows match the current filters, or the audit table is empty." className="py-8" />
                ) : (
                  <div className="ui-table-scroll">
                    <table className={UI_TABLE}>
                      <thead className={UI_THEAD_STICKY}>
                        <tr>
                          <th className={UI_TH}>Time</th>
                          <th className={UI_TH}>Severity</th>
                          <th className={UI_TH}>Result</th>
                          <th className={UI_TH}>Action</th>
                          <th className={UI_TH}>Actor</th>
                          <th className={UI_TH}>Resource</th>
                          <th className={UI_TH}>Session</th>
                        </tr>
                      </thead>
                      <tbody>
                        {govActivityItems.map((row) => (
                          <tr key={`${row.id ?? row.eventId ?? row.createdAt}-${row.action}`} className={UI_TR}>
                            <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                              {row.createdAt ? formatDateTime(row.createdAt) : '—'}
                            </td>
                            <td className={UI_TD}>
                              <span className={getSeverityBadgeClass((row.severity || 'info').toLowerCase())}>
                                {row.severity || '—'}
                              </span>
                            </td>
                            <td className={`${UI_TD} text-caption`}>{row.result || '—'}</td>
                            <td className={`${UI_TD} font-mono text-caption max-w-[14rem] truncate`} title={row.action}>
                              {row.action || '—'}
                            </td>
                            <td className={`${UI_TD} text-caption`}>
                              {row.actorUsername || '—'}
                              {row.actorUserId != null ? (
                                <span className="text-muted"> · uid {row.actorUserId}</span>
                              ) : null}
                            </td>
                            <td className={`${UI_TD} text-caption max-w-[12rem] truncate`} title={row.resourceType || row.resource}>
                              {row.resourceType || row.resource || '—'}
                              {row.resourceId ? <span className="text-muted"> / {row.resourceId}</span> : null}
                            </td>
                            <td className={`${UI_TD} font-mono text-caption max-w-[8rem] truncate`} title={row.sessionId}>
                              {row.sessionId ? `${row.sessionId.slice(0, 8)}…` : '—'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                <div className="p-3 border-t border-border flex flex-wrap justify-between items-center gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={govActivityOffset === 0 || govLoading}
                    onClick={() => setGovActivityOffset((o) => Math.max(0, o - govActivityLimit))}
                  >
                    Previous
                  </Button>
                  <span className="text-caption text-muted">
                    {govActivityTotal === 0
                      ? '0 events'
                      : `Offset ${govActivityOffset} · ${govActivityItems.length} row(s) · ${govActivityTotal} total`}
                  </span>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={govLoading || govActivityOffset + govActivityLimit >= govActivityTotal}
                    onClick={() => setGovActivityOffset((o) => o + govActivityLimit)}
                  >
                    Next
                  </Button>
                </div>
              </Card>
            )}

            {canSessionsRead && (
              <Card className="p-0 overflow-hidden">
                <div className="p-4 border-b border-border flex flex-wrap justify-between gap-3 items-start">
                  <div>
                    <h4 className="text-body font-semibold text-text">Sessions</h4>
                    <p className="text-caption text-muted mt-0.5">
                      JWT must reference an active server session when <span className="font-mono">user_sessions</span> is migrated. Revocation takes effect immediately.
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2 items-center">
                    {isFortunaAdmin && canUsersReadForSessions && (
                      <div className="flex items-center gap-2">
                        <label className="text-caption text-muted whitespace-nowrap">Filter by user id</label>
                        <input
                          className="bg-base border border-border rounded px-2 py-1.5 text-body w-28"
                          value={sessionUserIdFilter}
                          onChange={(e) => setSessionUserIdFilter(e.target.value.replace(/[^\d]/g, ''))}
                          placeholder="uid"
                        />
                        <Button size="sm" variant="secondary" type="button" onClick={() => void loadGovernance()}>
                          Refresh
                        </Button>
                      </div>
                    )}
                    {canSessionsRevoke && (
                      <Button size="sm" variant="secondary" type="button" onClick={() => void revokeAllMySessions()}>
                        Revoke all my sessions
                      </Button>
                    )}
                    {isFortunaAdmin && canSessionsRevokeAll && sessionUserIdFilter.trim() !== '' && (
                      <Button
                        size="sm"
                        variant="secondary"
                        type="button"
                        onClick={async () => {
                          const targetUserId = sessionUserIdFilter.trim();
                          const confirmed = await confirm({
                            title: 'Revoke all sessions for user',
                            description: `Revoke every session for Fortuna user id ${targetUserId}? They must sign in again.`,
                            confirmLabel: 'Revoke user sessions',
                            variant: 'danger',
                          });
                          if (!confirmed) return;
                          try {
                            await api.revokeAllUserSessions(targetUserId);
                            await loadGovernance();
                          } catch (e) {
                            toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
                          }
                        }}
                      >
                        Revoke all for user
                      </Button>
                    )}
                  </div>
                </div>
                {sessionsLoading ? (
                  <div className="p-8 text-muted">Loading sessions…</div>
                ) : sessionRows.length === 0 ? (
                  <PageEmpty
                    title="No sessions"
                    description="No session rows returned (table may not be migrated yet, or filters exclude all rows)."
                    className="py-8"
                  />
                ) : (
                  <div className="ui-table-scroll">
                    <table className={UI_TABLE}>
                      <thead className={UI_THEAD_STICKY}>
                        <tr>
                          <th className={UI_TH}>Session</th>
                          <th className={UI_TH}>User</th>
                          <th className={UI_TH}>Issued</th>
                          <th className={UI_TH}>Expires</th>
                          <th className={UI_TH}>Last activity</th>
                          <th className={UI_TH}>IP</th>
                          <th className={UI_TH}>State</th>
                          {canSessionsRevoke && <th className={`${UI_TH} text-right`}>Actions</th>}
                        </tr>
                      </thead>
                      <tbody>
                        {sessionRows.map((s) => {
                          const isOwn = String(user?.id ?? '') === s.userId;
                          const mayRevoke =
                            canSessionsRevoke &&
                            (isOwn || (isFortunaAdmin && canUsersReadForSessions));
                          return (
                            <tr key={s.id} className={UI_TR}>
                              <td className={`${UI_TD} font-mono text-caption`}>{s.id.slice(0, 8)}…</td>
                              <td className={UI_TD}>{s.userId}</td>
                              <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                                {s.issuedAt ? formatDateTime(s.issuedAt) : '—'}
                              </td>
                              <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                                {s.expiresAt ? formatDateTime(s.expiresAt) : '—'}
                              </td>
                              <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                                {s.lastActivityAt ? formatDateTime(s.lastActivityAt) : '—'}
                              </td>
                              <td className={`${UI_TD} text-caption`}>{s.sourceIp || '—'}</td>
                              <td className={UI_TD}>
                                {s.revokedAt ? (
                                  <span className="text-warning text-caption">revoked</span>
                                ) : (
                                  <span className="text-success text-caption">active</span>
                                )}
                              </td>
                              {canSessionsRevoke && (
                                <td className={`${UI_TD} text-right`}>
                                  {mayRevoke && !s.revokedAt ? (
                                    <button
                                      type="button"
                                      className="rounded px-2 py-1 text-caption text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-critical/60 disabled:cursor-not-allowed disabled:opacity-50"
                                      aria-label={`Revoke session ${s.id}`}
                                      disabled={revokingSessionId === s.id}
                                      onClick={() => void revokeFortunaSession(s.id)}
                                    >
                                      {revokingSessionId === s.id ? '…' : 'Revoke'}
                                    </button>
                                  ) : (
                                    <span className="text-caption text-muted">—</span>
                                  )}
                                </td>
                              )}
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </Card>
            )}
          </>
        )}

      </div>

      <Dialog
        open={addUserOpen}
        title="Add Fortuna user"
        description="Creates a dashboard user via registration. Password must meet server policy (at least 12 characters)."
        size="sm"
        closeDisabled={addUserBusy}
        onClose={() => setAddUserOpen(false)}
        footer={
          <>
            <Button type="button" variant="secondary" disabled={addUserBusy} onClick={() => setAddUserOpen(false)}>
              Cancel
            </Button>
            <Button type="button" isLoading={addUserBusy} onClick={() => void registerFortunaUser()}>
              Create user
            </Button>
          </>
        }
      >
            <div className="space-y-4">
              {addUserError ? (
                <div className="p-3 rounded bg-critical/10 border border-critical/30 text-critical text-body">{addUserError}</div>
              ) : null}
              <div>
                <label htmlFor="settings-new-username" className="block text-caption text-muted mb-1">Username</label>
                <input
                  id="settings-new-username"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  autoComplete="off"
                  disabled={addUserBusy}
                />
              </div>
              <div>
                <label htmlFor="settings-new-email" className="block text-caption text-muted mb-1">Email</label>
                <input
                  id="settings-new-email"
                  type="email"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  autoComplete="off"
                  disabled={addUserBusy}
                />
              </div>
              <div>
                <label htmlFor="settings-new-password" className="block text-caption text-muted mb-1">Password</label>
                <input
                  id="settings-new-password"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  autoComplete="new-password"
                  disabled={addUserBusy}
                />
              </div>
              <div>
                <label htmlFor="settings-new-role" className="block text-caption text-muted mb-1">Fortuna role</label>
                <select
                  id="settings-new-role"
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  value={(fortunaRoleEditOptions as readonly string[]).includes(newUserRole) ? newUserRole : fortunaRoleEditOptions[0]}
                  onChange={(e) => setNewUserRole(e.target.value)}
                  disabled={addUserBusy}
                >
                  {fortunaRoleEditOptions.map((r) => (
                    <option key={r} value={r}>
                      {fortunaRoleSelectLabel(r)}
                    </option>
                  ))}
                </select>
              </div>
              {isFortunaAdmin ? (
                <div className="space-y-3 rounded-lg border border-border bg-base/35 p-3">
                  <div>
                    <div className="text-caption font-semibold uppercase tracking-wide text-muted">Cluster access</div>
                    <p className="mt-1 text-caption text-muted">
                      Scope applies to security data after the user signs in. Admin role remains unrestricted by server policy.
                    </p>
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <label className="flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-surface/50 p-2.5 text-body text-text">
                      <input
                        type="radio"
                        name="settings-new-user-scope-mode"
                        className="mt-1"
                        checked={newUserScopeMode === 'all'}
                        disabled={addUserBusy}
                        onChange={() => setNewUserScopeMode('all')}
                      />
                      <span>
                        <span className="block font-semibold">All clusters</span>
                        <span className="block text-caption text-muted">Current and future clusters.</span>
                      </span>
                    </label>
                    <label className="flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-surface/50 p-2.5 text-body text-text">
                      <input
                        type="radio"
                        name="settings-new-user-scope-mode"
                        className="mt-1"
                        checked={newUserScopeMode === 'selected'}
                        disabled={addUserBusy}
                        onChange={() => setNewUserScopeMode('selected')}
                      />
                      <span>
                        <span className="block font-semibold">Selected clusters</span>
                        <span className="block text-caption text-muted">Restrict by cluster_id allow-list.</span>
                      </span>
                    </label>
                  </div>
                  {newUserRole.toLowerCase() === 'admin' ? (
                    <div className="rounded border border-border bg-surface/50 p-2 text-caption text-muted">
                      Platform admin is always unrestricted. Cluster scope is applied to viewer, operator, and user admin accounts.
                    </div>
                  ) : loadingClusters ? (
                    <div className="text-caption text-muted">Loading clusters…</div>
                  ) : clusters.length === 0 ? (
                    <div className="rounded border border-border bg-surface/50 p-2 text-caption text-muted">
                      No active clusters returned by /inventory/clusters/stats.
                    </div>
                  ) : (
                    <div className={newUserScopeMode === 'selected' ? 'grid gap-2' : 'grid gap-2 opacity-55'}>
                      {clusters.map((cluster) => (
                        <label key={cluster.id} className="flex cursor-pointer items-start gap-2 rounded border border-border bg-surface/50 p-2">
                          <input
                            type="checkbox"
                            className="mt-1"
                            checked={newUserScopeClusters.includes(cluster.id)}
                            disabled={addUserBusy || newUserScopeMode !== 'selected'}
                            onChange={(event) => {
                              const checked = event.target.checked;
                              setNewUserScopeClusters((current) =>
                                checked
                                  ? Array.from(new Set([...current, cluster.id]))
                                  : current.filter((id) => id !== cluster.id),
                              );
                            }}
                          />
                          <span className="min-w-0">
                            <span className="block truncate text-body font-semibold text-text" title={clusterDisplayName(cluster)}>
                              {cluster.name || cluster.id}
                            </span>
                            <span className="block break-all font-mono text-[11px] text-muted">{cluster.id}</span>
                          </span>
                        </label>
                      ))}
                    </div>
                  )}
                </div>
              ) : null}
            </div>
      </Dialog>

      <Dialog
        open={!!scopeEditorUser}
        title="Edit cluster access"
        description="Cluster access limits which monitored clusters this user can see in inventory, runtime, findings, attack paths, and reports. Platform admins remain unrestricted."
        size="lg"
        closeDisabled={scopeSaving}
        onClose={() => setScopeEditorUser(null)}
        footer={
          <>
            <Button type="button" variant="secondary" disabled={scopeSaving} onClick={() => setScopeEditorUser(null)}>
              Cancel
            </Button>
            <Button type="button" isLoading={scopeSaving} onClick={() => void saveScopeEditor()}>
              Save access
            </Button>
          </>
        }
      >
        <div className="space-y-5">
          {scopeEditorUser ? (
            <div className="rounded-lg border border-border bg-base/40 px-3 py-2.5">
              <div className="text-body font-semibold text-text">{scopeEditorUser.username}</div>
              <div className="mt-1 text-caption text-muted">
                {scopeEditorUser.email || 'No email'} · {fortunaRoleShortLabel(scopeEditorUser.role)}
              </div>
              {scopeEditorUser.permissions?.length ? (
                <div className="mt-3 border-t border-border pt-3">
                  <div className="mb-2 text-caption font-semibold uppercase tracking-wide text-muted">Effective permissions</div>
                  <div className="flex flex-wrap gap-1.5">
                    {scopeEditorUser.permissions.slice(0, 10).map((permission) => (
                      <span
                        key={permission}
                        className="rounded border border-border bg-surface-2/70 px-2 py-0.5 font-mono text-[11px] text-muted"
                      >
                        {permission}
                      </span>
                    ))}
                    {scopeEditorUser.permissions.length > 10 ? (
                      <span className="rounded border border-border bg-surface-2/70 px-2 py-0.5 text-[11px] text-muted">
                        +{scopeEditorUser.permissions.length - 10} more
                      </span>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}

          {scopeError ? (
            <div className="rounded-lg border border-critical/30 bg-critical/10 p-3 text-body text-critical">{scopeError}</div>
          ) : null}

          <fieldset className="space-y-3">
            <legend className="text-caption font-semibold uppercase tracking-wide text-muted">Access mode</legend>
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-base/35 p-3 text-body text-text">
              <input
                type="radio"
                name="settings-cluster-scope-mode"
                className="mt-1"
                checked={scopeMode === 'all'}
                disabled={scopeSaving}
                onChange={() => setScopeMode('all')}
              />
              <span>
                <span className="block font-semibold">All clusters</span>
                <span className="mt-1 block text-caption text-muted">
                  User can access every current and future monitored cluster allowed by their Fortuna role.
                </span>
              </span>
            </label>
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-base/35 p-3 text-body text-text">
              <input
                type="radio"
                name="settings-cluster-scope-mode"
                className="mt-1"
                checked={scopeMode === 'selected'}
                disabled={scopeSaving}
                onChange={() => setScopeMode('selected')}
              />
              <span>
                <span className="block font-semibold">Selected clusters</span>
                <span className="mt-1 block text-caption text-muted">
                  User only sees security data whose cluster_id is in this allow-list.
                </span>
              </span>
            </label>
          </fieldset>

          <div className={scopeMode === 'selected' ? 'space-y-3' : 'space-y-3 opacity-55'}>
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-body font-semibold text-text">Cluster allow-list</h3>
              {scopeMode === 'selected' ? (
                <span className="text-caption text-muted">
                  {selectedScopeClusters.length} selected
                </span>
              ) : null}
            </div>
            {loadingClusters ? (
              <div className="rounded-lg border border-border bg-base/35 p-4 text-body text-muted">Loading clusters…</div>
            ) : clusters.length === 0 ? (
              <PageEmpty
                title="No clusters available"
                description="No active synced clusters were returned by /inventory/clusters/stats."
                className="rounded-lg border border-border py-6"
              />
            ) : (
              <div className="grid gap-2 sm:grid-cols-2">
                {clusters.map((cluster) => {
                  const checked = selectedScopeClusters.includes(cluster.id);
                  return (
                    <label
                      key={cluster.id}
                      className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-base/35 p-3"
                    >
                      <input
                        type="checkbox"
                        className="mt-1"
                        checked={checked}
                        disabled={scopeSaving || scopeMode !== 'selected'}
                        onChange={(event) => {
                          const nextChecked = event.target.checked;
                          setSelectedScopeClusters((current) =>
                            nextChecked
                              ? Array.from(new Set([...current, cluster.id]))
                              : current.filter((id) => id !== cluster.id),
                          );
                        }}
                      />
                      <span className="min-w-0">
                        <span className="block truncate text-body font-semibold text-text" title={clusterDisplayName(cluster)}>
                          {cluster.name || cluster.id}
                        </span>
                        <span className="mt-1 block break-all font-mono text-[11px] text-muted">{cluster.id}</span>
                        <span className="mt-1 block text-caption text-muted">
                          {cluster.podCount ?? cluster.pods ?? 0} pods · {cluster.connectionStatus || cluster.status || 'unknown'}
                        </span>
                      </span>
                    </label>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </Dialog>

      <Dialog
        open={!!riskRuleModal}
        title={riskRuleModal === 'add' ? 'Add risk rule' : 'Edit risk rule'}
        description={
          <span className="flex items-center gap-1">
                <HelpCircle className="w-3.5 h-3.5 shrink-0" />
                Verify and Save are validated on the server only. Use <strong>Verify</strong> to check rules before saving.
          </span>
        }
        size="md"
        closeDisabled={saving || verifying}
        onClose={() => setRiskRuleModal(null)}
        footer={
          <>
            <Button variant="secondary" disabled={saving || verifying} onClick={() => setRiskRuleModal(null)}>Cancel</Button>
            <Button variant="secondary" onClick={handleVerifyRule} isLoading={verifying} disabled={saving}>
              Verify
            </Button>
            <Button onClick={saveRiskRule} isLoading={saving} disabled={verifying}>Save</Button>
          </>
        }
      >

              {riskRuleModal === 'add' && (
                <div className="mb-4">
                  <label className="block text-caption text-muted mb-1">Use template</label>
                  <select
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                    value={Math.max(0, RISK_RULE_TEMPLATES.findIndex((t) => t.rule.id === formRule.id && t.rule.name === formRule.name))}
                    onChange={(e) => {
                      const idx = Number(e.target.value);
                      if (idx >= 0 && idx < RISK_RULE_TEMPLATES.length) applyTemplate(RISK_RULE_TEMPLATES[idx].rule);
                    }}
                  >
                    {RISK_RULE_TEMPLATES.map((t, i) => (
                      <option key={t.name} value={i}>{t.name}</option>
                    ))}
                  </select>
                </div>
              )}

              {(formErrors.length > 0 || (verifyResult && !verifyResult.valid)) && (
                <div className="mb-4 p-3 rounded bg-critical/10 border border-critical/30 text-critical text-body flex items-start gap-2">
                  <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <ul className="list-disc list-inside space-y-0.5">
                    {formErrors.length > 0 ? formErrors.map((msg, i) => <li key={i}>{msg}</li>) : (verifyResult?.errors ?? []).map((msg, i) => <li key={i}>{msg}</li>)}
                  </ul>
                </div>
              )}
              {verifyResult?.valid === true && (
                <div className="mb-4 p-3 rounded bg-success/10 border border-success/30 text-success text-body flex items-center gap-2">
                  <CheckCircle className="w-4 h-4 shrink-0" />
                  Rule is valid and ready to save.
                </div>
              )}

              <div className="space-y-3">
                <div>
                  <label className="block text-caption text-muted mb-1">Rule ID</label>
                  <input
                    value={formRule.id}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, id: e.target.value.replace(/\s/g, '-').toLowerCase() }))}
                    placeholder="e.g. my-custom-rule"
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                    readOnly={riskRuleModal === 'edit'}
                  />
                  <p className="text-caption text-muted mt-0.5">Unique identifier; cannot be changed after create.</p>
                </div>
                <div>
                  <label className="block text-caption text-muted mb-1">Name</label>
                  <input
                    value={formRule.name}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, name: e.target.value }))}
                    placeholder="Short display name"
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-caption text-muted mb-1">Severity</label>
                    <select
                      value={formRule.severity}
                      onChange={(e) => setFormRule((prev) => ({ ...prev, severity: e.target.value }))}
                      className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                    >
                      {SEVERITIES.map((s) => (
                        <option key={s} value={s}>{s}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-caption text-muted mb-1">Category</label>
                    <select
                      value={formRule.category}
                      onChange={(e) => setFormRule((prev) => ({ ...prev, category: e.target.value }))}
                      className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                    >
                      {CATEGORIES.map((c) => (
                        <option key={c} value={c}>{c}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <div>
                  <label className="block text-caption text-muted mb-1">Description</label>
                  <textarea
                    value={formRule.description || ''}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, description: e.target.value }))}
                    rows={2}
                    placeholder="What this rule detects and why it matters"
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  />
                </div>
                <div>
                  <label className="block text-caption text-muted mb-1">Conditions (expression)</label>
                  <textarea
                    value={formRule.conditions?.[0]?.expression ?? ''}
                    onChange={(e) => setFormRule((prev) => ({
                      ...prev,
                      conditions: [{ type: 'expression', expression: e.target.value }],
                    }))}
                    rows={2}
                    placeholder="e.g. true or roleRef.name == 'cluster-admin'"
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text font-mono"
                  />
                  <p className="text-caption text-muted mt-0.5">First condition: type expression. Used to match resources (RBAC, etc.).</p>
                </div>
                <div>
                  <label className="block text-caption text-muted mb-1">Aggregation</label>
                  <select
                    value={formRule.aggregation ?? 'AND'}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, aggregation: e.target.value }))}
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  >
                    {AGGREGATIONS.map((a) => (
                      <option key={a} value={a}>{a}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="flex items-center gap-2 text-body text-muted">
                    <input type="checkbox" checked={formRule.enabled} onChange={(e) => setFormRule((prev) => ({ ...prev, enabled: e.target.checked }))} />
                    Enabled
                  </label>
                </div>
                <div>
                  <label className="block text-caption text-muted mb-1">Base score (0–10)</label>
                  <input
                    type="number"
                    min={0}
                    max={10}
                    step={0.1}
                    value={formRule.base_score ?? 7}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, base_score: parseFloat(e.target.value) || 0 }))}
                    className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  />
                  <p className="text-caption text-muted mt-0.5">CVSS-like score for risk ranking.</p>
                </div>
              </div>
      </Dialog>

      <Dialog
        open={importModalOpen}
        title="Import rule from YAML"
        description={<>Paste YAML below. Use <strong>Verify</strong> to validate on the server before importing.</>}
        size="lg"
        closeDisabled={importing || verifyingImport}
        onClose={() => setImportModalOpen(false)}
        footer={
          <>
            <Button variant="secondary" disabled={importing || verifyingImport} onClick={() => setImportModalOpen(false)}>Cancel</Button>
            <Button variant="secondary" onClick={handleVerifyImportYaml} isLoading={verifyingImport} disabled={importing}>
              Verify YAML
            </Button>
            <Button onClick={handleImportYaml} isLoading={importing} disabled={verifyingImport}>Import</Button>
          </>
        }
      >
              {importErrors.length > 0 && (
                <div className="mb-4 p-3 rounded bg-critical/10 border border-critical/30 text-critical text-body flex items-start gap-2">
                  <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <ul className="list-disc list-inside space-y-0.5">
                    {importErrors.map((msg, i) => (
                      <li key={i}>{msg}</li>
                    ))}
                  </ul>
                </div>
              )}
              {importVerifyResult?.valid === true && (
                <div className="mb-4 p-3 rounded bg-success/10 border border-success/30 text-success text-body flex items-center gap-2">
                  <CheckCircle className="w-4 h-4 shrink-0" />
                  YAML is valid. You can import now.
                </div>
              )}
              <textarea
                value={importYaml}
                onChange={(e) => setImportYaml(e.target.value)}
                placeholder={`id: my-rule\nname: My Rule\nseverity: medium\ncategory: rbac\ndescription: "..."\nenabled: true\nconditions:\n  - type: expression\n    expression: 'true'\naggregation: AND\nbase_score: 7`}
                rows={14}
                className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text font-mono"
              />
      </Dialog>
    </PageLayout>
  );
};
