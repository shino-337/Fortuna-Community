import React, { useCallback, useEffect, useState } from 'react';
import { CheckCircle, FileDown, FileUp, HelpCircle, Pencil, Plus, Trash2, XCircle } from 'lucide-react';
import { Card } from '../../design-system/components/Card';
import { Button } from '../ui/Button';
import { Dialog } from '../../design-system/components/Dialog';
import { useConfirm } from '../../design-system/components/ConfirmDialog';
import { useToast } from '../../design-system/components/Toast';
import { PageEmpty } from '../../design-system/components/PageStatus';
import type { RiskRuleFull, RiskRuleItem } from '../../types';
import { api } from '../../lib/api';
import { Badge } from '../../design-system/components/Badge';
import { UI_TABLE, UI_TD, UI_TH, UI_THEAD_STICKY, UI_TR } from '../../lib/tableChrome';
import { can, P } from '../../lib/permissions';
import { usePermUser } from '../../hooks/usePermUser';
import { downloadText } from '../../lib/download';

const SEVERITIES = ['critical', 'high', 'medium', 'low', 'info'] as const;
const CATEGORIES = ['rbac', 'pod-security', 'network-policy', 'secrets', 'runtime-behavior', 'compliance'] as const;
const AGGREGATIONS = ['AND', 'OR', 'THRESHOLD'] as const;

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

/** Risk scoring rules (`/risk/rules`): list, add, edit, delete, YAML import and export. */
export const RiskRulesPanel: React.FC = () => {
  const confirm = useConfirm();
  const toast = useToast();
  const permUser = usePermUser();
  const canRulesWrite = can(permUser, P.rulesWrite);
  const canRulesDelete = can(permUser, P.rulesDelete);
  const canRulesImport = can(permUser, P.rulesImport);
  const canRulesExport = can(permUser, P.rulesExport);
  const [riskRules, setRiskRules] = useState<RiskRuleItem[]>([]);
  const [riskRulesSource, setRiskRulesSource] = useState<string>('');
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
    void loadRiskRules();
  }, [loadRiskRules]);


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
    downloadText(yamlContent, filename, 'application/x-yaml');
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

  return (
    <>
      <Card className="p-0 overflow-hidden">
        <div className="p-4 border-b border-border flex items-center justify-between flex-wrap gap-2">
          <span className="text-body text-muted">
            <strong className="text-text tabular-nums">{riskRules.length}</strong> rule{riskRules.length !== 1 ? 's' : ''}
            {riskRulesSource === 'db' && ' · stored in the database, editable here'}
            {riskRulesSource === 'files' && ' · read-only, loaded from YAML files'}
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
                    <Badge severity={r.severity} uppercase={false} showShape={false} className="capitalize">{r.severity}</Badge>
                  </td>
                  <td className={`${UI_TD} text-muted`}>{r.category || '—'}</td>
                  <td className={UI_TD}>
                    <span
                      className={`px-2 py-0.5 rounded text-caption font-medium border ${
                        r.enabled ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-muted bg-surface-2 border-border'
                      }`}
                    >
                      {r.enabled ? 'Enabled' : 'Disabled'}
                    </span>
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
    </>
  );
};
