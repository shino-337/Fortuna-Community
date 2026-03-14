import React, { useState, useEffect, useCallback } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { User, AuditLog, RiskRuleItem, RiskRuleFull } from '../types';
import { api } from '../lib/api';
import { Shield, Plus, Pencil, Trash2, HelpCircle, CheckCircle, XCircle, FileDown, FileUp } from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime } from '../lib/display';
import { getSeverityBadgeClass } from '../lib/severity';

const SEVERITIES = ['critical', 'high', 'medium', 'low', 'info'] as const;
const CATEGORIES = ['rbac', 'pod-security', 'network-policy', 'secrets', 'runtime-behavior', 'compliance'] as const;
const AGGREGATIONS = ['AND', 'OR', 'THRESHOLD'] as const;

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
  const [activeTab, setActiveTab] = useState('Users');
  const [users, setUsers] = useState<User[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [riskRules, setRiskRules] = useState<RiskRuleItem[]>([]);
  const [riskRulesSource, setRiskRulesSource] = useState<string>('');
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [loadingAudit, setLoadingAudit] = useState(false);
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

  useEffect(() => {
    if (activeTab === 'Users') {
      setLoadingUsers(true);
      api.getUsers().then((data) => {
        setUsers(data);
        setLoadingUsers(false);
      });
    }
    if (activeTab === 'Audit Logs') {
      setLoadingAudit(true);
      api.getAuditLogs({ page: 1, pageSize: 50 }).then((data) => {
        setAuditLogs(data.logs);
        setLoadingAudit(false);
      });
    }
    if (activeTab === 'Risk Rules') {
      loadRiskRules();
    }
  }, [activeTab, loadRiskRules]);

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
    if (!confirm('Delete this rule?')) return;
    try {
      await api.deleteRiskRule(id);
      loadRiskRules();
    } catch (e) {
      alert(String(e instanceof Error ? e.message : e));
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
      alert(String(e instanceof Error ? e.message : e));
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
      alert(String(e instanceof Error ? e.message : e));
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

  return (
    <PageLayout title="Settings" description="Administration: users, audit logs, and risk rules (DB).">
      <div className="border-b border-slate-800">
        <nav className="flex space-x-6 overflow-x-auto">
          {['Users', 'Audit Logs', 'Risk Rules'].map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`pb-4 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${activeTab === tab ? 'border-pink-500 text-pink-500' : 'border-transparent text-slate-400 hover:text-slate-200 hover:border-slate-700'}`}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>
      <div className="grid gap-6">
        {activeTab === 'Users' && (
          <Card className="p-0 overflow-hidden">
            {loadingUsers ? (
              <div className="p-8 text-muted">Loading users...</div>
            ) : users.length === 0 ? (
              <PageEmpty title="No users" description="No user records returned by /api/v1/users." className="py-8" />
            ) : (
              <div className="overflow-x-auto max-h-[60vh] overflow-y-auto">
                <table className="w-full text-sm text-left">
                  <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                    <tr>
                      <th className="px-6 py-4">User</th>
                      <th className="px-6 py-4">Role</th>
                      <th className="px-6 py-4">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {users.map((user) => (
                      <tr key={user.id} className="hover:bg-muted/30">
                        <td className="px-6 py-4">
                          <div className="text-text font-medium">{user.username || user.name || 'unknown'}</div>
                          <div className="text-muted text-xs">{user.email || 'N/A'}</div>
                        </td>
                        <td className="px-6 py-4 capitalize text-muted">
                          <div className="flex items-center">
                            <Shield className={`w-3 h-3 mr-2 ${user.role === 'admin' ? 'text-pink-500' : 'text-muted'}`} />
                            {user.role || 'viewer'}
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <span className={`px-2 py-0.5 rounded-full text-xs border capitalize ${user.active === false ? 'bg-muted/20 text-muted border-border' : 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20'}`}>
                            {user.status || (user.active === false ? 'disabled' : 'active')}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        )}

        {activeTab === 'Audit Logs' && (
          <Card className="p-0 overflow-hidden">
            {loadingAudit ? (
              <div className="p-8 text-muted">Loading audit logs...</div>
            ) : auditLogs.length === 0 ? (
              <PageEmpty title="No audit logs" description="No records returned by /api/v1/audit." className="py-8" />
            ) : (
              <div className="overflow-x-auto max-h-[60vh] overflow-y-auto">
                <table className="w-full text-sm text-left">
                  <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                    <tr>
                      <th className="px-6 py-4">Time</th>
                      <th className="px-6 py-4">Actor</th>
                      <th className="px-6 py-4">Action</th>
                      <th className="px-6 py-4">Resource</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {auditLogs.map((log) => (
                      <tr key={log.id} className="hover:bg-muted/30">
                        <td className="px-6 py-4 font-mono text-xs text-muted">{formatDateTime(log.timestamp)}</td>
                        <td className="px-6 py-4 text-text font-medium">{log.actor || log.user || 'system'}</td>
                        <td className="px-6 py-4 text-muted">{log.action}</td>
                        <td className="px-6 py-4 text-muted font-mono text-xs">{log.resource}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        )}

        {activeTab === 'Risk Rules' && (
          <Card className="p-0 overflow-hidden">
            <div className="p-4 border-b border-border flex items-center justify-between flex-wrap gap-2">
              <span className="text-sm text-muted">
                Source: <strong className="text-text">{riskRulesSource || '—'}</strong>
                {riskRulesSource && (
                  <span className="text-muted ml-1">({riskRules.length} rule{riskRules.length !== 1 ? 's' : ''})</span>
                )}
                {riskRulesSource === 'files' && ' — read-only from YAML; add rules in DB to edit here.'}
                {riskRulesSource === 'db' && ' — rules from database; new rules appear here after Save.'}
              </span>
              {riskRulesSource === 'db' && (
                <div className="flex items-center gap-2">
                  <Button size="sm" variant="secondary" onClick={handleExportAll} isLoading={exporting} disabled={riskRules.length === 0}>
                    <FileDown className="w-4 h-4 mr-2" /> Export YAML
                  </Button>
                  <Button size="sm" variant="secondary" onClick={openImportModal}>
                    <FileUp className="w-4 h-4 mr-2" /> Import YAML
                  </Button>
                  <Button size="sm" onClick={openAddRule}>
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
              <div className="overflow-x-auto max-h-[60vh] overflow-y-auto">
                <table className="w-full text-sm text-left">
                  <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                    <tr>
                      <th className="px-6 py-4">ID</th>
                      <th className="px-6 py-4">Name</th>
                      <th className="px-6 py-4">Severity</th>
                      <th className="px-6 py-4">Category</th>
                      <th className="px-6 py-4">Status</th>
                      {riskRulesSource === 'db' && <th className="px-6 py-4 text-right">Actions</th>}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {riskRules.map((r) => (
                      <tr key={r.id} className="hover:bg-muted/30">
                      <td className="px-6 py-4 font-mono text-slate-300">{r.id}</td>
                      <td className="px-6 py-4 text-white font-medium">{r.name}</td>
                      <td className="px-6 py-4">
                        <span className={getSeverityBadgeClass(r.severity)}>{r.severity}</span>
                      </td>
                      <td className="px-6 py-4 text-slate-400">{r.category || '—'}</td>
                      <td className="px-6 py-4">
                        <span className={r.enabled ? 'text-emerald-500' : 'text-slate-500'}>{r.enabled ? 'Enabled' : 'Disabled'}</span>
                      </td>
                      {riskRulesSource === 'db' && (
                        <td className="px-6 py-4 text-right">
                          <button onClick={() => handleExportOne(r.id)} className="text-slate-400 hover:text-sky-400 p-1 mr-2" title="Export as YAML" disabled={exporting}>
                            <FileDown className="w-4 h-4" />
                          </button>
                          <button onClick={() => openEditRule(r.id)} className="text-slate-400 hover:text-pink-500 p-1 mr-2" title="Edit">
                            <Pencil className="w-4 h-4" />
                          </button>
                          <button onClick={() => deleteRiskRule(r.id)} className="text-slate-400 hover:text-red-500 p-1" title="Delete">
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
      </div>

      {riskRuleModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setRiskRuleModal(null)}>
          <div className="bg-slate-900 border border-slate-700 rounded-lg max-w-lg w-full max-h-[90vh] overflow-hidden flex flex-col shadow-xl" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 overflow-y-auto">
              <h3 className="text-lg font-semibold text-white mb-2">{riskRuleModal === 'add' ? 'Add risk rule' : 'Edit risk rule'}</h3>
              <p className="text-xs text-slate-500 mb-4 flex items-center gap-1">
                <HelpCircle className="w-3.5 h-3.5 shrink-0" />
                Verify and Save are validated on the server only. Use <strong>Verify</strong> to check rules before saving.
              </p>

              {riskRuleModal === 'add' && (
                <div className="mb-4">
                  <label className="block text-xs text-slate-500 mb-1">Use template</label>
                  <select
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
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
                <div className="mb-4 p-3 rounded bg-red-500/10 border border-red-500/30 text-red-400 text-sm flex items-start gap-2">
                  <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <ul className="list-disc list-inside space-y-0.5">
                    {formErrors.length > 0 ? formErrors.map((msg, i) => <li key={i}>{msg}</li>) : (verifyResult?.errors ?? []).map((msg, i) => <li key={i}>{msg}</li>)}
                  </ul>
                </div>
              )}
              {verifyResult?.valid === true && (
                <div className="mb-4 p-3 rounded bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-sm flex items-center gap-2">
                  <CheckCircle className="w-4 h-4 shrink-0" />
                  Rule is valid and ready to save.
                </div>
              )}

              <div className="space-y-3">
                <div>
                  <label className="block text-xs text-slate-500 mb-1">Rule ID</label>
                  <input
                    value={formRule.id}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, id: e.target.value.replace(/\s/g, '-').toLowerCase() }))}
                    placeholder="e.g. my-custom-rule"
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                    readOnly={riskRuleModal === 'edit'}
                  />
                  <p className="text-xs text-slate-500 mt-0.5">Unique identifier; cannot be changed after create.</p>
                </div>
                <div>
                  <label className="block text-xs text-slate-500 mb-1">Name</label>
                  <input
                    value={formRule.name}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, name: e.target.value }))}
                    placeholder="Short display name"
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-slate-500 mb-1">Severity</label>
                    <select
                      value={formRule.severity}
                      onChange={(e) => setFormRule((prev) => ({ ...prev, severity: e.target.value }))}
                      className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                    >
                      {SEVERITIES.map((s) => (
                        <option key={s} value={s}>{s}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-slate-500 mb-1">Category</label>
                    <select
                      value={formRule.category}
                      onChange={(e) => setFormRule((prev) => ({ ...prev, category: e.target.value }))}
                      className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                    >
                      {CATEGORIES.map((c) => (
                        <option key={c} value={c}>{c}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-slate-500 mb-1">Description</label>
                  <textarea
                    value={formRule.description || ''}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, description: e.target.value }))}
                    rows={2}
                    placeholder="What this rule detects and why it matters"
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs text-slate-500 mb-1">Conditions (expression)</label>
                  <textarea
                    value={formRule.conditions?.[0]?.expression ?? ''}
                    onChange={(e) => setFormRule((prev) => ({
                      ...prev,
                      conditions: [{ type: 'expression', expression: e.target.value }],
                    }))}
                    rows={2}
                    placeholder="e.g. true or roleRef.name == 'cluster-admin'"
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white font-mono"
                  />
                  <p className="text-xs text-slate-500 mt-0.5">First condition: type expression. Used to match resources (RBAC, etc.).</p>
                </div>
                <div>
                  <label className="block text-xs text-slate-500 mb-1">Aggregation</label>
                  <select
                    value={formRule.aggregation ?? 'AND'}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, aggregation: e.target.value }))}
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                  >
                    {AGGREGATIONS.map((a) => (
                      <option key={a} value={a}>{a}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="flex items-center gap-2 text-sm text-slate-400">
                    <input type="checkbox" checked={formRule.enabled} onChange={(e) => setFormRule((prev) => ({ ...prev, enabled: e.target.checked }))} />
                    Enabled
                  </label>
                </div>
                <div>
                  <label className="block text-xs text-slate-500 mb-1">Base score (0–10)</label>
                  <input
                    type="number"
                    min={0}
                    max={10}
                    step={0.1}
                    value={formRule.base_score ?? 7}
                    onChange={(e) => setFormRule((prev) => ({ ...prev, base_score: parseFloat(e.target.value) || 0 }))}
                    className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white"
                  />
                  <p className="text-xs text-slate-500 mt-0.5">CVSS-like score for risk ranking.</p>
                </div>
              </div>
            </div>
            <div className="p-6 pt-0 flex flex-wrap items-center justify-end gap-2 border-t border-slate-800">
              <Button variant="secondary" onClick={() => setRiskRuleModal(null)}>Cancel</Button>
              <Button variant="secondary" onClick={handleVerifyRule} isLoading={verifying}>
                Verify
              </Button>
              <Button onClick={saveRiskRule} isLoading={saving}>Save</Button>
            </div>
          </div>
        </div>
      )}

      {importModalOpen && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setImportModalOpen(false)}>
          <div className="bg-slate-900 border border-slate-700 rounded-lg max-w-2xl w-full max-h-[90vh] overflow-hidden flex flex-col shadow-xl" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 overflow-y-auto">
              <h3 className="text-lg font-semibold text-white mb-2">Import rule from YAML</h3>
              <p className="text-xs text-slate-500 mb-4">
                Paste YAML below. Use <strong>Verify</strong> to validate on the server before importing.
              </p>
              {importErrors.length > 0 && (
                <div className="mb-4 p-3 rounded bg-red-500/10 border border-red-500/30 text-red-400 text-sm flex items-start gap-2">
                  <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <ul className="list-disc list-inside space-y-0.5">
                    {importErrors.map((msg, i) => (
                      <li key={i}>{msg}</li>
                    ))}
                  </ul>
                </div>
              )}
              {importVerifyResult?.valid === true && (
                <div className="mb-4 p-3 rounded bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-sm flex items-center gap-2">
                  <CheckCircle className="w-4 h-4 shrink-0" />
                  YAML is valid. You can import now.
                </div>
              )}
              <textarea
                value={importYaml}
                onChange={(e) => setImportYaml(e.target.value)}
                placeholder={`id: my-rule\nname: My Rule\nseverity: medium\ncategory: rbac\ndescription: "..."\nenabled: true\nconditions:\n  - type: expression\n    expression: 'true'\naggregation: AND\nbase_score: 7`}
                rows={14}
                className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-white font-mono"
              />
            </div>
            <div className="p-6 pt-0 flex flex-wrap items-center justify-end gap-2 border-t border-slate-800">
              <Button variant="secondary" onClick={() => setImportModalOpen(false)}>Cancel</Button>
              <Button variant="secondary" onClick={handleVerifyImportYaml} isLoading={verifyingImport}>
                Verify YAML
              </Button>
              <Button onClick={handleImportYaml} isLoading={importing}>Import</Button>
            </div>
          </div>
        </div>
      )}
    </PageLayout>
  );
};
