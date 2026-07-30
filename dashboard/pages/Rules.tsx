import React, {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { PolicyInstanceRow, PolicyTemplateRow, SecurityRule } from "../types";
import { Card } from "../design-system/components/Card";
import { Button } from "../components/ui/Button";
import { PageLayout } from "../design-system/layouts/PageLayout";
import { PageEmpty, PageLoading } from "../design-system/components/PageStatus";
import { SemanticEmptyState } from "../design-system/components/SemanticEmptyState";
import { FilterBar } from "../design-system/components/FilterBar";
import { Pagination } from "../components/Pagination";
import { Tabs } from "../design-system/components/Tabs";
import { useConfirm } from "../design-system/components/ConfirmDialog";
import { Dialog } from "../design-system/components/Dialog";
import { useToast } from "../design-system/components/Toast";
import { DataFreshness } from "../components/DataFreshness";
import { usePolling, REFRESH_INTERVALS } from "../hooks/usePolling";
import { useRefreshIntervalStore } from "../store/refreshIntervalStore";
import { useRefreshTriggerStore } from "../store/refreshTriggerStore";
import { usePermUser } from "../hooks/usePermUser";
import { ACTION_IDS, canRunAction } from "../lib/actionAccess";
import { useFeatureVisibility } from "../hooks/useVisibility";
import { PAGE_TITLES } from "../lib/pageTitles";
import {
  RefreshCw,
  FlaskConical,
  Info,
  ListChecks,
  FileCode2,
  Boxes,
  Plus,
  Trash2,
  Pencil,
} from "lucide-react";
import { getSeverityBadgeClass } from "../lib/severity";
import {
  UI_TABLE,
  UI_TD,
  UI_TH,
  UI_TR,
  UI_THEAD_STICKY,
} from "../lib/tableChrome";

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const SELECT_CLASS =
  "bg-base border border-border rounded-lg px-3 py-2 text-body text-text focus:outline-none focus:border-brand";
const POLICY_WORKBENCH_SECTION = "border-t border-border/70";
const POLICY_WORKBENCH_PAD = "px-4 py-3 sm:px-5";

type RulesTab = "detection" | "templates" | "instances";

export const Rules: React.FC = () => {
  const navigate = useNavigate();
  const confirm = useConfirm();
  const toast = useToast();
  const permUser = usePermUser();
  const canPoliciesDraft = canRunAction(permUser, ACTION_IDS.policyDraft);
  const canPoliciesPublish = canRunAction(permUser, ACTION_IDS.policyPublish);
  const canPoliciesDelete = canRunAction(permUser, ACTION_IDS.policyDelete);
  const rulesVisibility = useFeatureVisibility("rules_catalog");
  const [activeTab, setActiveTab] = useState<RulesTab>("detection");
  const [rules, setRules] = useState<SecurityRule[]>([]);
  const [loading, setLoading] = useState(false);
  const [initialBoot, setInitialBoot] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rulesUpdatedAt, setRulesUpdatedAt] = useState<Date | null>(null);
  const [statusFilter, setStatusFilter] = useState<
    "all" | "enabled" | "disabled"
  >("enabled");
  const [severityFilter, setSeverityFilter] = useState<
    "all" | "critical" | "high" | "medium" | "low"
  >("all");
  const [searchTerm, setSearchTerm] = useState("");
  const [sortBy, setSortBy] = useState<"name_asc" | "severity_desc" | "status">(
    "severity_desc",
  );
  const [overlappingOnly, setOverlappingOnly] = useState(false);
  const [reloadingRules, setReloadingRules] = useState(false);
  const [testingRuleId, setTestingRuleId] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const [templates, setTemplates] = useState<PolicyTemplateRow[]>([]);
  const [instances, setInstances] = useState<PolicyInstanceRow[]>([]);
  const [auxLoading, setAuxLoading] = useState(false);
  const [auxError, setAuxError] = useState<string | null>(null);
  const [auxUpdatedAt, setAuxUpdatedAt] = useState<Date | null>(null);
  const [tplSearch, setTplSearch] = useState("");
  const [instSearch, setInstSearch] = useState("");

  const [showTplForm, setShowTplForm] = useState(false);
  const [editTpl, setEditTpl] = useState<PolicyTemplateRow | null>(null);
  const [showInstForm, setShowInstForm] = useState(false);
  const [editInst, setEditInst] = useState<PolicyInstanceRow | null>(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const loadRules = useCallback(async () => {
    if (!rulesVisibility.visible) return;
    setError(null);
    setLoading(true);
    try {
      const data = await api.getRules();
      setRules(data);
      setRulesUpdatedAt(new Date());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load rules");
      setRules([]);
    } finally {
      setLoading(false);
      setInitialBoot(false);
    }
  }, [rulesVisibility.visible]);

  const loadPolicyAux = useCallback(async () => {
    setAuxError(null);
    setAuxLoading(true);
    try {
      const [t, i] = await Promise.all([
        api.getPolicyTemplates(),
        api.getPolicyInstances(),
      ]);
      setTemplates(t);
      setInstances(i);
      setAuxUpdatedAt(new Date());
    } catch (e) {
      setAuxError(
        e instanceof Error
          ? e.message
          : "Failed to load policy templates/instances",
      );
      setTemplates([]);
      setInstances([]);
    } finally {
      setAuxLoading(false);
    }
  }, []);

  const intervalMs = useRefreshIntervalStore((s) =>
    s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST),
  );
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(loadRules, intervalMs, {
    enabled: rulesVisibility.visible,
    refreshTrigger,
  });
  useEffect(() => {
    if (rulesVisibility.visible) loadRules();
  }, [loadRules, rulesVisibility.visible]);

  useEffect(() => {
    if (
      rulesVisibility.visible &&
      (activeTab === "templates" || activeTab === "instances")
    ) {
      loadPolicyAux();
    }
  }, [activeTab, loadPolicyAux, rulesVisibility.visible]);

  useEffect(() => {
    setPage(1);
  }, [
    statusFilter,
    severityFilter,
    overlappingOnly,
    searchTerm,
    sortBy,
    activeTab,
  ]);

  const filteredRules = useMemo(() => {
    let out = [...rules];
    if (statusFilter !== "all")
      out = out.filter((r) =>
        statusFilter === "enabled" ? r.enabled : !r.enabled,
      );
    if (severityFilter !== "all")
      out = out.filter(
        (r) => (r.severity || "").toLowerCase() === severityFilter,
      );
    if (overlappingOnly) out = out.filter((r) => r.isCanonical === false);
    if (searchTerm.trim()) {
      const q = searchTerm.trim().toLowerCase();
      out = out.filter((r) =>
        [r.id, r.name, r.category, r.type, r.description].some((v) =>
          (v || "").toLowerCase().includes(q),
        ),
      );
    }
    const sevRank: Record<string, number> = {
      critical: 4,
      high: 3,
      medium: 2,
      low: 1,
    };
    out.sort((a, b) => {
      switch (sortBy) {
        case "name_asc":
          return (a.name || "").localeCompare(b.name || "");
        case "status":
          return Number(b.enabled) - Number(a.enabled);
        case "severity_desc":
        default:
          return (
            (sevRank[(b.severity || "").toLowerCase()] ?? 0) -
            (sevRank[(a.severity || "").toLowerCase()] ?? 0)
          );
      }
    });
    return out;
  }, [
    rules,
    statusFilter,
    severityFilter,
    overlappingOnly,
    searchTerm,
    sortBy,
  ]);

  const paginatedRules = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredRules.slice(start, start + pageSize);
  }, [filteredRules, page, pageSize]);

  const filteredTemplates = useMemo(() => {
    if (!tplSearch.trim()) return templates;
    const q = tplSearch.trim().toLowerCase();
    return templates.filter(
      (t) =>
        (t.templateId || "").toLowerCase().includes(q) ||
        (t.name || "").toLowerCase().includes(q) ||
        (t.category || "").toLowerCase().includes(q) ||
        (t.description || "").toLowerCase().includes(q),
    );
  }, [templates, tplSearch]);

  const filteredInstances = useMemo(() => {
    if (!instSearch.trim()) return instances;
    const q = instSearch.trim().toLowerCase();
    return instances.filter(
      (i) =>
        (i.instanceName || "").toLowerCase().includes(q) ||
        (i.templateId || "").toLowerCase().includes(q) ||
        (i.description || "").toLowerCase().includes(q),
    );
  }, [instances, instSearch]);

  const handleReloadRules = async () => {
    setReloadingRules(true);
    try {
      await api.reloadRules();
      await loadRules();
    } finally {
      setReloadingRules(false);
    }
  };

  const handleQuickTest = async (ruleId: string) => {
    setTestingRuleId(ruleId);
    try {
      await api.testRule(ruleId, {
        apiVersion: "v1",
        kind: "Pod",
        metadata: { name: "quick-test-pod", namespace: "default" },
      });
    } finally {
      setTestingRuleId(null);
    }
  };

  const handleDeleteTemplate = async (t: PolicyTemplateRow) => {
    const confirmed = await confirm({
      title: "Delete policy template",
      description: `Delete template "${t.templateId}" version ${t.version}. This cannot be undone.`,
      confirmLabel: "Delete template",
      cancelLabel: "Keep template",
      variant: "danger",
    });
    if (!confirmed) return;
    try {
      await api.deletePolicyTemplate(t.templateId, t.version);
      await loadPolicyAux();
      toast({
        title: "Template deleted",
        description: `${t.templateId} v${t.version} was removed.`,
        variant: "success",
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : "Delete failed";
      setAuxError(message);
      toast({
        title: "Template delete failed",
        description: message,
        variant: "error",
      });
    }
  };

  const handleSaveTemplate = async (data: Omit<PolicyTemplateRow, "id">) => {
    setSaving(true);
    setFormError(null);
    try {
      if (editTpl) {
        await api.updatePolicyTemplate(
          editTpl.templateId,
          editTpl.version,
          data,
        );
      } else {
        await api.createPolicyTemplate(data);
      }
      setShowTplForm(false);
      setEditTpl(null);
      await loadPolicyAux();
    } catch (e) {
      setFormError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteInstance = async (inst: PolicyInstanceRow) => {
    const confirmed = await confirm({
      title: "Delete policy instance",
      description: `Delete instance "${inst.instanceName}". This removes its policy evaluation configuration.`,
      confirmLabel: "Delete instance",
      cancelLabel: "Keep instance",
      variant: "danger",
    });
    if (!confirmed) return;
    try {
      await api.deletePolicyInstance(inst.instanceName);
      await loadPolicyAux();
      toast({
        title: "Instance deleted",
        description: `${inst.instanceName} was removed.`,
        variant: "success",
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : "Delete failed";
      setAuxError(message);
      toast({
        title: "Instance delete failed",
        description: message,
        variant: "error",
      });
    }
  };

  const handleSaveInstance = async (data: Omit<PolicyInstanceRow, "id">) => {
    setSaving(true);
    setFormError(null);
    try {
      if (editInst) {
        await api.updatePolicyInstance(editInst.instanceName, data);
      } else {
        await api.createPolicyInstance(data);
      }
      setShowInstForm(false);
      setEditInst(null);
      await loadPolicyAux();
    } catch (e) {
      setFormError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  if (initialBoot && loading) {
    return (
      <PageLoading
        message="Loading policy rules..."
        className="min-h-[40dvh]"
      />
    );
  }

  if (!rulesVisibility.visible) {
    return (
      <PageLayout
        title={PAGE_TITLES.policyRules}
        description="Detection rules, CEL templates, and policy instances."
      >
        <SemanticEmptyState
          state={rulesVisibility.semanticState}
          reason={rulesVisibility.reason}
          className="min-h-[40dvh]"
        />
      </PageLayout>
    );
  }

  const tooltipLabelClass =
    "inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help";
  const enabledFilteredCount = filteredRules.filter((r) => r.enabled).length;
  const disabledFilteredCount = filteredRules.length - enabledFilteredCount;
  const activeRuleActivityCount = filteredRules.filter(
    (r) => (r.matches ?? 0) > 0,
  ).length;
  const activeTabMeta = {
    detection: {
      title: "Detection catalog",
      description: "Runtime and posture rules used to classify findings.",
      count: filteredRules.length,
      countLabel: "rules",
    },
    templates: {
      title: "Policy templates",
      description: "Reusable CEL definitions for policy evaluation.",
      count: filteredTemplates.length,
      countLabel: "templates",
    },
    instances: {
      title: "Policy instances",
      description: "Scoped policy evaluations created from templates.",
      count: filteredInstances.length,
      countLabel: "instances",
    },
  }[activeTab];

  const detectionToolbar = (
    <FilterBar
      embedded
      search={{
        value: searchTerm,
        onChange: setSearchTerm,
        placeholder: "Search id, name, category…",
        inputClassName: "min-w-[220px] flex-1 max-w-md",
      }}
      trailing={
        <>
          <select
            value={statusFilter}
            onChange={(e) =>
              setStatusFilter(e.target.value as typeof statusFilter)
            }
            className={SELECT_CLASS}
          >
            <option value="all">All status</option>
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
          <select
            value={severityFilter}
            onChange={(e) =>
              setSeverityFilter(e.target.value as typeof severityFilter)
            }
            className={SELECT_CLASS}
          >
            <option value="all">All severity</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as typeof sortBy)}
            className={SELECT_CLASS}
          >
            <option value="severity_desc">Sort: Severity high to low</option>
            <option value="name_asc">Sort: Name A-Z</option>
            <option value="status">Sort: Enabled first</option>
          </select>
        </>
      }
    >
      <label className="inline-flex items-center gap-2 text-body text-text">
        <input
          type="checkbox"
          checked={overlappingOnly}
          onChange={(e) => setOverlappingOnly(e.target.checked)}
          className="rounded border-border bg-base"
        />
        Overlapping only
      </label>
    </FilterBar>
  );

  const templatesToolbar = (
    <FilterBar
      embedded
      search={{
        value: tplSearch,
        onChange: setTplSearch,
        placeholder: "Search template id, name, category…",
        inputClassName: "min-w-[220px] flex-1 max-w-md",
      }}
      trailing={
        <Button
          size="sm"
          disabled={!canPoliciesDraft}
          onClick={() => {
            setEditTpl(null);
            setFormError(null);
            setShowTplForm(true);
          }}
        >
          <Plus className="mr-1 h-4 w-4" /> New template
        </Button>
      }
    />
  );

  const instancesToolbar = (
    <FilterBar
      embedded
      search={{
        value: instSearch,
        onChange: setInstSearch,
        placeholder: "Search instance name, template…",
        inputClassName: "min-w-[220px] flex-1 max-w-md",
      }}
      trailing={
        <Button
          size="sm"
          disabled={!canPoliciesDraft}
          onClick={() => {
            setEditInst(null);
            setFormError(null);
            setShowInstForm(true);
          }}
        >
          <Plus className="mr-1 h-4 w-4" /> New instance
        </Button>
      }
    />
  );
  const activeToolbar =
    activeTab === "detection"
      ? detectionToolbar
      : activeTab === "templates"
        ? templatesToolbar
        : instancesToolbar;
  const activeActions = (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      <DataFreshness
        updatedAt={activeTab === "detection" ? rulesUpdatedAt : auxUpdatedAt}
        loading={(loading && activeTab === "detection") || (auxLoading && activeTab !== "detection")}
        error={activeTab === "detection" ? error : auxError}
        staleAfterMs={intervalMs * 2}
      />
      <Button
        size="sm"
        variant="secondary"
        isLoading={
          (loading && activeTab === "detection") ||
          (auxLoading && activeTab !== "detection")
        }
        onClick={() =>
          activeTab === "detection" ? loadRules() : loadPolicyAux()
        }
      >
        <RefreshCw className="mr-1.5 h-4 w-4" /> Refresh
      </Button>
      {activeTab === "detection" ? (
        <Button
          size="sm"
          variant="secondary"
          disabled={!canPoliciesPublish}
          isLoading={reloadingRules}
          onClick={handleReloadRules}
        >
          Reload engine
        </Button>
      ) : null}
    </div>
  );
  const activeSummary =
    activeTab === "detection"
      ? `${filteredRules.length.toLocaleString("en-US")} shown · ${activeRuleActivityCount.toLocaleString("en-US")} with finding activity · ${enabledFilteredCount.toLocaleString("en-US")} enabled · ${disabledFilteredCount.toLocaleString("en-US")} disabled`
      : activeTab === "templates"
        ? `${filteredTemplates.length.toLocaleString("en-US")} shown · ${templates.length.toLocaleString("en-US")} total`
        : `${filteredInstances.length.toLocaleString("en-US")} shown · ${instances.length.toLocaleString("en-US")} total`;

  return (
    <PageLayout
      title={PAGE_TITLES.policyRules}
      description="Manage detection rules, reusable CEL templates, and scoped policy instances."
    >
      {activeTab === "detection" && error && (
        <div className="mb-4 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-amber-200 text-body">
          {error}
        </div>
      )}
      {(activeTab === "templates" || activeTab === "instances") && auxError && (
        <div className="mb-4 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-amber-200 text-body">
          {auxError}
        </div>
      )}

      <Card className="overflow-hidden p-0">
        <div className="flex min-w-0 flex-col gap-3 px-4 pt-4 sm:px-5 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0 flex-1">
            <Tabs
              className="mb-0"
              variant="underline"
              ariaLabel="Policy rule sections"
              items={[
                {
                  id: "detection",
                  label: "Detection",
                  icon: <ListChecks className="w-4 h-4" />,
                },
                {
                  id: "templates",
                  label: "Templates",
                  icon: <FileCode2 className="w-4 h-4" />,
                },
                {
                  id: "instances",
                  label: "Instances",
                  icon: <Boxes className="w-4 h-4" />,
                },
              ]}
              value={activeTab}
              onChange={(id) => setActiveTab(id as RulesTab)}
            />
          </div>
          {activeActions}
        </div>

        <div className={`${POLICY_WORKBENCH_SECTION} ${POLICY_WORKBENCH_PAD}`}>
          <div className="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between">
            <div className="min-w-0">
              <h2 className="text-card-title text-text">
                {activeTabMeta.title}
              </h2>
              <p className="mt-1 text-caption text-muted">
                {activeTabMeta.description}
              </p>
            </div>
            <p className="shrink-0 text-caption text-muted">
              <span className="font-semibold text-text">
                {activeTabMeta.count.toLocaleString("en-US")}
              </span>{" "}
              {activeTabMeta.countLabel}
            </p>
          </div>
          <p className="mt-3 text-meta text-muted">{activeSummary}</p>
        </div>

        <div className={`${POLICY_WORKBENCH_SECTION} ${POLICY_WORKBENCH_PAD}`}>
          {activeToolbar}
        </div>

        {activeTab === "detection" && (
          <>
            <div className={POLICY_WORKBENCH_SECTION}>
              <div className="ui-table-scroll">
                <table className={UI_TABLE}>
                  <thead className={UI_THEAD_STICKY}>
                    <tr>
                      <th className={UI_TH}>Rule</th>
                      <th className={UI_TH}>Category / Type</th>
                      <th className={UI_TH}>Severity</th>
                      <th className={UI_TH}>Status</th>
                      <th className={UI_TH}>
                        <span
                          className={tooltipLabelClass}
                          title="Rule source: db (managed), files (YAML), built-in (fallback)"
                        >
                          Source <Info className="w-3 h-3" />
                        </span>
                      </th>
                      <th className={UI_TH}>
                        <span
                          className={tooltipLabelClass}
                          title="Finding activity: persisted findings whose cve_id equals this rule id, plus 24h/7d windows and related capabilities from evidence"
                        >
                          Finding activity <Info className="w-3 h-3" />
                        </span>
                      </th>
                      <th className={`${UI_TH} text-right`}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paginatedRules.length === 0 ? (
                      <tr>
                        <td colSpan={7} className={`${UI_TD} py-8`}>
                          <PageEmpty
                            title="No rules match current filters"
                            description="Adjust search or filter conditions."
                            className="py-6"
                          />
                        </td>
                      </tr>
                    ) : (
                      paginatedRules.map((rule) => (
                        <tr key={rule.id} className={UI_TR}>
                          <td className={UI_TD}>
                            <div className="font-medium text-text">
                              {rule.name}
                            </div>
                            <div className="text-caption text-muted font-mono">
                              {rule.id}
                            </div>
                            {rule.description && (
                              <div
                                className="text-caption text-muted mt-1 max-w-[460px] truncate"
                                title={rule.description}
                              >
                                {rule.description}
                              </div>
                            )}
                          </td>
                          <td className="px-6 py-4 text-muted">
                            {rule.category || "uncategorized"}
                            {rule.type ? ` / ${rule.type}` : ""}
                          </td>
                          <td className={UI_TD}>
                            <span
                              className={`px-2 py-0.5 rounded text-caption font-medium ${getSeverityBadgeClass(rule.severity)}`}
                            >
                              {rule.severity}
                            </span>
                          </td>
                          <td className={UI_TD}>
                            <span
                              className={`px-2 py-0.5 rounded text-caption font-medium border ${
                                rule.enabled
                                  ? "text-emerald-400 bg-emerald-500/10 border-emerald-500/20"
                                  : "text-muted bg-surface-2 border-border"
                              }`}
                            >
                              {rule.enabled ? "Enabled" : "Disabled"}
                            </span>
                          </td>
                          <td className="px-6 py-4 text-caption text-text">
                            <div title="Where this rule definition is loaded from">
                              {rule.source ?? "unknown"}
                            </div>
                            <div
                              className="mt-1 text-muted"
                              title="Primary rule = main rule in a shared signature group. Overlapping rule = same signature group, kept for compatibility/tuning."
                            >
                              {rule.isCanonical === false
                                ? `Overlapping rule of ${rule.canonicalRuleId}`
                                : "Primary rule"}
                            </div>
                          </td>
                          <td className="px-6 py-4 text-muted text-caption">
                            <div>
                              Matches:{" "}
                              <span className="text-text">
                                {rule.matches ?? 0}
                              </span>
                            </div>
                            <div
                              className="mt-1"
                              title="Impacted findings in rolling windows"
                            >
                              24h/7d:{" "}
                              <span className="text-text">
                                {rule.impactedFindings24h ?? 0}/
                                {rule.impactedFindings7d ?? 0}
                              </span>
                            </div>
                            <div className="mt-1">
                              Last:{" "}
                              <span className="text-text">
                                {rule.lastMatchedAt
                                  ? new Date(
                                      rule.lastMatchedAt,
                                    ).toLocaleString()
                                  : "Never"}
                              </span>
                            </div>
                            {rule.relatedCapabilities &&
                              rule.relatedCapabilities.length > 0 && (
                                <div
                                  className="mt-1 text-muted truncate max-w-[220px]"
                                  title={`Related capabilities from evidence: ${rule.relatedCapabilities.join(", ")}`}
                                >
                                  Caps: {rule.relatedCapabilities.join(", ")}
                                </div>
                              )}
                          </td>
                          <td className="px-6 py-4 text-right">
                            <div className="flex items-center justify-end gap-2">
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() => handleQuickTest(rule.id)}
                                disabled={testingRuleId === rule.id}
                              >
                                <FlaskConical className="w-4 h-4 mr-1" /> Test
                              </Button>
                              <Button
                                size="sm"
                                onClick={() => navigate(`/rules/uid/${encodeURIComponent(rule.uid ?? rule.id)}`)}
                              >
                                Open
                              </Button>
                            </div>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
            {filteredRules.length > 0 && (
              <div
                className={`${POLICY_WORKBENCH_SECTION} ${POLICY_WORKBENCH_PAD}`}
              >
                <Pagination
                  page={page}
                  pageSize={pageSize}
                  total={filteredRules.length}
                  onPageChange={setPage}
                  onPageSizeChange={(size) => {
                    setPageSize(size);
                    setPage(1);
                  }}
                  pageSizeOptions={PAGE_SIZE_OPTIONS}
                  itemLabel="rules"
                />
              </div>
            )}
          </>
        )}

        {activeTab === "templates" && (
          <>
            {showTplForm && (
              <TemplateFormModal
                initial={editTpl}
                saving={saving}
                error={formError}
                onSave={handleSaveTemplate}
                onClose={() => {
                  setShowTplForm(false);
                  setEditTpl(null);
                  setFormError(null);
                }}
              />
            )}
            <div className={POLICY_WORKBENCH_SECTION}>
              {auxLoading ? (
                <PageLoading message="Loading templates..." className="py-12" />
              ) : (
                <div className="ui-table-scroll">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH}>Template</th>
                        <th className={UI_TH}>Version</th>
                        <th className={UI_TH}>Category</th>
                        <th className={UI_TH}>Severity</th>
                        <th className={UI_TH}>System</th>
                        <th className={`${UI_TH} text-right`}>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredTemplates.length === 0 ? (
                        <tr>
                          <td colSpan={6} className={`${UI_TD} py-8`}>
                            <PageEmpty
                              title="No templates"
                              description="Click 'New template' to create one."
                              className="py-6"
                            />
                          </td>
                        </tr>
                      ) : (
                        filteredTemplates.map((t) => (
                          <tr
                            key={`${t.templateId}-${t.version}`}
                            className={UI_TR}
                          >
                            <td className={UI_TD}>
                              <div className="font-medium text-text">
                                {t.name}
                              </div>
                              <div className="text-caption text-muted font-mono">
                                {t.templateId}
                              </div>
                              {t.description && (
                                <div className="text-caption text-muted mt-1 line-clamp-2">
                                  {t.description}
                                </div>
                              )}
                            </td>
                            <td className="px-6 py-4 text-muted font-mono text-caption">
                              {t.version}
                            </td>
                            <td className="px-6 py-4 text-muted">
                              {t.category}
                            </td>
                            <td className={UI_TD}>
                              <span
                                className={`px-2 py-0.5 rounded text-caption font-medium ${getSeverityBadgeClass(t.defaultSeverity)}`}
                              >
                                {t.defaultSeverity}
                              </span>
                            </td>
                            <td className="px-6 py-4 text-muted text-caption">
                              {t.isSystem ? "Yes" : "No"}
                            </td>
                            <td className="px-6 py-4 text-right">
                              <div className="flex items-center justify-end gap-2">
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  disabled={!canPoliciesDraft}
                                  onClick={() => {
                                    setEditTpl(t);
                                    setFormError(null);
                                    setShowTplForm(true);
                                  }}
                                >
                                  <Pencil className="w-3.5 h-3.5" />
                                </Button>
                                {!t.isSystem && (
                                  <Button
                                    size="sm"
                                    variant="secondary"
                                    disabled={!canPoliciesDelete}
                                    onClick={() => handleDeleteTemplate(t)}
                                  >
                                    <Trash2 className="w-3.5 h-3.5 text-red-400" />
                                  </Button>
                                )}
                              </div>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </>
        )}

        {activeTab === "instances" && (
          <>
            {showInstForm && (
              <InstanceFormModal
                initial={editInst}
                templates={templates}
                saving={saving}
                error={formError}
                onSave={handleSaveInstance}
                onClose={() => {
                  setShowInstForm(false);
                  setEditInst(null);
                  setFormError(null);
                }}
              />
            )}
            <div className={POLICY_WORKBENCH_SECTION}>
              {auxLoading ? (
                <PageLoading message="Loading instances..." className="py-12" />
              ) : (
                <div className="ui-table-scroll">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH}>Instance</th>
                        <th className={UI_TH}>Template</th>
                        <th className={UI_TH}>Enabled</th>
                        <th className={UI_TH}>Action</th>
                        <th className={UI_TH}>Severity</th>
                        <th className={`${UI_TH} text-right`}>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredInstances.length === 0 ? (
                        <tr>
                          <td colSpan={6} className={`${UI_TD} py-8`}>
                            <PageEmpty
                              title="No policy instances"
                              description="Click 'New instance' to create one."
                              className="py-6"
                            />
                          </td>
                        </tr>
                      ) : (
                        filteredInstances.map((inst) => (
                          <tr key={inst.instanceName} className={UI_TR}>
                            <td className={UI_TD}>
                              <div className="font-medium text-text">
                                {inst.instanceName}
                              </div>
                              {inst.description && (
                                <div className="text-caption text-muted mt-1 line-clamp-2">
                                  {inst.description}
                                </div>
                              )}
                            </td>
                            <td className="px-6 py-4 text-muted text-caption font-mono">
                              {inst.templateId}@{inst.templateVersion}
                            </td>
                            <td className={UI_TD}>
                              <span
                                className={`px-2 py-0.5 rounded text-caption font-medium border ${
                                  inst.enabled
                                    ? "text-emerald-400 bg-emerald-500/10 border-emerald-500/20"
                                    : "text-muted bg-surface-2 border-border"
                                }`}
                              >
                                {inst.enabled ? "Yes" : "No"}
                              </span>
                            </td>
                            <td className="px-6 py-4 text-muted text-caption">
                              {inst.action ?? "—"}
                            </td>
                            <td className={UI_TD}>
                              {inst.severity ? (
                                <span
                                  className={`px-2 py-0.5 rounded text-caption font-medium ${getSeverityBadgeClass(inst.severity)}`}
                                >
                                  {inst.severity}
                                </span>
                              ) : (
                                <span className="text-muted text-caption">
                                  —
                                </span>
                              )}
                            </td>
                            <td className="px-6 py-4 text-right">
                              <div className="flex items-center justify-end gap-2">
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  disabled={!canPoliciesDraft}
                                  onClick={() => {
                                    setEditInst(inst);
                                    setFormError(null);
                                    setShowInstForm(true);
                                  }}
                                >
                                  <Pencil className="w-3.5 h-3.5" />
                                </Button>
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  disabled={!canPoliciesDelete}
                                  onClick={() => handleDeleteInstance(inst)}
                                >
                                  <Trash2 className="w-3.5 h-3.5 text-red-400" />
                                </Button>
                              </div>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </>
        )}
      </Card>
    </PageLayout>
  );
};

/* ===================== Template Form Modal ===================== */

const FORM_LABEL = "block text-caption text-muted mb-1";
const FORM_INPUT =
  "w-full bg-base border border-border rounded-lg px-3 py-2 text-body text-text focus:outline-none focus:border-brand";
const SEVERITY_OPTIONS = ["critical", "high", "medium", "low", "info"];
const ACTION_OPTIONS = ["alert", "deny", "audit", "warn"];

function TemplateFormModal({
  initial,
  saving,
  error,
  onSave,
  onClose,
}: {
  initial: PolicyTemplateRow | null;
  saving: boolean;
  error: string | null;
  onSave: (data: Omit<PolicyTemplateRow, "id">) => void;
  onClose: () => void;
}) {
  const isEdit = !!initial;
  const [templateId, setTemplateId] = useState(initial?.templateId ?? "");
  const [version, setVersion] = useState(initial?.version ?? "1.0.0");
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [category, setCategory] = useState(initial?.category ?? "security");
  const [defaultSeverity, setDefaultSeverity] = useState(
    initial?.defaultSeverity ?? "medium",
  );
  const [defaultAction, setDefaultAction] = useState(
    initial?.defaultAction ?? "alert",
  );
  const [celExpression, setCelExpression] = useState(
    initial?.celExpression ?? "",
  );
  const [isSystem, setIsSystem] = useState(initial?.isSystem ?? false);
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSave({
      templateId,
      version,
      name,
      description,
      category,
      defaultSeverity,
      defaultAction,
      celExpression,
      isSystem,
    });
  };

  const formId = "policy-template-form";

  return (
    <Dialog
      open
      title={isEdit ? "Edit template" : "New policy template"}
      size="md"
      closeDisabled={saving}
      onClose={onClose}
      footer={
        <>
          <Button type="button" variant="secondary" disabled={saving} onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form={formId} isLoading={saving}>
            {isEdit ? "Save changes" : "Create template"}
          </Button>
        </>
      }
    >
        {error && (
          <div className="mb-3 rounded border border-red-500/40 bg-red-500/10 px-3 py-2 text-red-200 text-body">
            {error}
          </div>
        )}
        <form id={formId} onSubmit={handleSubmit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={FORM_LABEL}>Template ID *</label>
              <input
                className={FORM_INPUT}
                value={templateId}
                onChange={(e) => setTemplateId(e.target.value)}
                required
                disabled={isEdit}
                placeholder="e.g. no-privileged-pods"
              />
            </div>
            <div>
              <label className={FORM_LABEL}>Version *</label>
              <input
                className={FORM_INPUT}
                value={version}
                onChange={(e) => setVersion(e.target.value)}
                required
                disabled={isEdit}
                placeholder="1.0.0"
              />
            </div>
          </div>
          <div>
            <label className={FORM_LABEL}>Name *</label>
            <input
              className={FORM_INPUT}
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="No Privileged Pods"
            />
          </div>
          <div>
            <label className={FORM_LABEL}>Description</label>
            <textarea
              className={FORM_INPUT + " min-h-[60px]"}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className={FORM_LABEL}>Category</label>
              <input
                className={FORM_INPUT}
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                placeholder="security"
              />
            </div>
            <div>
              <label className={FORM_LABEL}>Severity</label>
              <select
                className={FORM_INPUT}
                value={defaultSeverity}
                onChange={(e) => setDefaultSeverity(e.target.value)}
              >
                {SEVERITY_OPTIONS.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className={FORM_LABEL}>Default action</label>
              <select
                className={FORM_INPUT}
                value={defaultAction}
                onChange={(e) => setDefaultAction(e.target.value)}
              >
                {ACTION_OPTIONS.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div>
            <label className={FORM_LABEL}>CEL Expression *</label>
            <textarea
              className={FORM_INPUT + " font-mono text-caption min-h-[80px]"}
              value={celExpression}
              onChange={(e) => setCelExpression(e.target.value)}
              required
              rows={3}
              placeholder="resource.spec.containers.exists(c, c.securityContext.privileged == true)"
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={isSystem}
              onChange={(e) => setIsSystem(e.target.checked)}
              id="tpl-is-system"
              className="rounded border-border bg-base"
            />
            <label htmlFor="tpl-is-system" className="text-body text-text">
              System template (cannot be deleted)
            </label>
          </div>
        </form>
    </Dialog>
  );
}

/* ===================== Instance Form Modal ===================== */

function InstanceFormModal({
  initial,
  templates,
  saving,
  error,
  onSave,
  onClose,
}: {
  initial: PolicyInstanceRow | null;
  templates: PolicyTemplateRow[];
  saving: boolean;
  error: string | null;
  onSave: (data: Omit<PolicyInstanceRow, "id">) => void;
  onClose: () => void;
}) {
  const isEdit = !!initial;
  const [instanceName, setInstanceName] = useState(initial?.instanceName ?? "");
  const [templateId, setTemplateId] = useState(
    initial?.templateId ?? templates[0]?.templateId ?? "",
  );
  const [templateVersion, setTemplateVersion] = useState(
    initial?.templateVersion ?? templates[0]?.version ?? "",
  );
  const [description, setDescription] = useState(initial?.description ?? "");
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [action, setAction] = useState(initial?.action ?? "alert");
  const [severity, setSeverity] = useState(initial?.severity ?? "medium");
  const [clusters, setClusters] = useState(initial?.clusters?.join(", ") ?? "");
  const [namespaces, setNamespaces] = useState(
    initial?.namespaces?.join(", ") ?? "",
  );
  const [resourceTypes, setResourceTypes] = useState(
    initial?.resourceTypes?.join(", ") ?? "",
  );

  const splitComma = (s: string) =>
    s
      .split(",")
      .map((v) => v.trim())
      .filter(Boolean);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSave({
      instanceName,
      templateId,
      templateVersion,
      description,
      enabled,
      action,
      severity,
      clusters: splitComma(clusters).length ? splitComma(clusters) : undefined,
      namespaces: splitComma(namespaces).length
        ? splitComma(namespaces)
        : undefined,
      resourceTypes: splitComma(resourceTypes).length
        ? splitComma(resourceTypes)
        : undefined,
    });
  };

  const uniqueTemplates = useMemo(() => {
    const seen = new Set<string>();
    return templates.filter((t) => {
      const key = `${t.templateId}@${t.version}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }, [templates]);

  const formId = "policy-instance-form";

  return (
    <Dialog
      open
      title={isEdit ? "Edit instance" : "New policy instance"}
      size="md"
      closeDisabled={saving}
      onClose={onClose}
      footer={
        <>
          <Button type="button" variant="secondary" disabled={saving} onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form={formId} isLoading={saving}>
            {isEdit ? "Save changes" : "Create instance"}
          </Button>
        </>
      }
    >
        {error && (
          <div className="mb-3 rounded border border-red-500/40 bg-red-500/10 px-3 py-2 text-red-200 text-body">
            {error}
          </div>
        )}
        <form id={formId} onSubmit={handleSubmit} className="space-y-3">
          <div>
            <label className={FORM_LABEL}>Instance name *</label>
            <input
              className={FORM_INPUT}
              value={instanceName}
              onChange={(e) => setInstanceName(e.target.value)}
              required
              disabled={isEdit}
              placeholder="prod-no-privileged"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={FORM_LABEL}>Template *</label>
              <select
                className={FORM_INPUT}
                value={`${templateId}@${templateVersion}`}
                onChange={(e) => {
                  const [tid, tv] = e.target.value.split("@");
                  setTemplateId(tid);
                  setTemplateVersion(tv);
                }}
                disabled={isEdit}
              >
                {uniqueTemplates.map((t) => (
                  <option
                    key={`${t.templateId}@${t.version}`}
                    value={`${t.templateId}@${t.version}`}
                  >
                    {t.name} ({t.templateId} v{t.version})
                  </option>
                ))}
                {uniqueTemplates.length === 0 && (
                  <option value="">No templates available</option>
                )}
              </select>
            </div>
            <div>
              <label className={FORM_LABEL}>Action</label>
              <select
                className={FORM_INPUT}
                value={action}
                onChange={(e) => setAction(e.target.value)}
              >
                {ACTION_OPTIONS.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div>
            <label className={FORM_LABEL}>Description</label>
            <textarea
              className={FORM_INPUT + " min-h-[60px]"}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={FORM_LABEL}>Severity</label>
              <select
                className={FORM_INPUT}
                value={severity}
                onChange={(e) => setSeverity(e.target.value)}
              >
                {SEVERITY_OPTIONS.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-2 pt-5">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                id="inst-enabled"
                className="rounded border-border bg-base"
              />
              <label htmlFor="inst-enabled" className="text-body text-text">
                Enabled
              </label>
            </div>
          </div>
          <div>
            <label className={FORM_LABEL}>
              Clusters (comma-separated, empty = all)
            </label>
            <input
              className={FORM_INPUT}
              value={clusters}
              onChange={(e) => setClusters(e.target.value)}
              placeholder="prod-cluster, staging-cluster"
            />
          </div>
          <div>
            <label className={FORM_LABEL}>
              Namespaces (comma-separated, empty = all)
            </label>
            <input
              className={FORM_INPUT}
              value={namespaces}
              onChange={(e) => setNamespaces(e.target.value)}
              placeholder="default, kube-system"
            />
          </div>
          <div>
            <label className={FORM_LABEL}>
              Resource types (comma-separated, empty = all)
            </label>
            <input
              className={FORM_INPUT}
              value={resourceTypes}
              onChange={(e) => setResourceTypes(e.target.value)}
              placeholder="Pod, Deployment"
            />
          </div>
        </form>
    </Dialog>
  );
}
