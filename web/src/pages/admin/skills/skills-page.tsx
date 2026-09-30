import { App, Button, Input, Select, Skeleton } from "antd";
import { CalendarClock, Heart, LayoutGrid, RefreshCw, Search, ShieldAlert, UploadCloud, UsersRound } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";

import { skillCategoryIconOf } from "@/components/skills/skill-category-icons";
import { SkillInstallModal } from "@/components/skills/skill-install-modal";
import { cn } from "@/lib/utils";
import { AdminPageFrame } from "@/pages/admin/components/admin-shell";
import { AdminStatusBadge } from "@/pages/admin/components/admin-ui";
import { Checkbox, Switch } from "@/pages/admin/ui/controls";
import { AdminModal } from "@/pages/admin/ui/overlays";
import {
    fetchAdminSkills,
    setAdminSkillAvailability,
    setAdminSkillCategoryAvailability,
    type AdminSkillCatalog,
    type AdminSkillCategory,
    type AdminSkillItem,
} from "@/services/api/admin-skills";

import "./skills-page.css";

type AvailabilityFilter = "all" | "effective" | "disabled" | "category-disabled";

export default function AdminSkillsPage() {
    const { message, modal } = App.useApp();
    const tabsRef = useRef<HTMLDivElement>(null);
    const [catalog, setCatalog] = useState<AdminSkillCatalog | null>(null);
    const [loading, setLoading] = useState(true);
    const [savingIds, setSavingIds] = useState<Set<string>>(new Set());
    const [savingCategory, setSavingCategory] = useState("");
    const [batchSaving, setBatchSaving] = useState(false);
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
    const [activeCategory, setActiveCategory] = useState("all");
    const [activeSkill, setActiveSkill] = useState<AdminSkillItem | null>(null);
    const [installOpen, setInstallOpen] = useState(false);
    const [search, setSearch] = useState("");
    const [availabilityFilter, setAvailabilityFilter] = useState<AvailabilityFilter>("all");

    const applyCatalog = (result: AdminSkillCatalog) => {
        setCatalog(result);
        setSelectedIds((current) => new Set([...current].filter((id) => result.skills.some((skill) => skill.skillId === id))));
        setActiveSkill((current) => current ? result.skills.find((skill) => skill.skillId === current.skillId) ?? null : null);
    };

    const reload = async () => {
        setLoading(true);
        try {
            applyCatalog(await fetchAdminSkills());
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取技能管理数据失败");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void reload();
    }, []);

    const categories = catalog?.categories || [];
    const categoryTabs = useMemo(() => [
        { value: "all", label: "全部技能", totalCount: catalog?.skills.length || 0, available: true },
        ...categories,
    ], [catalog?.skills.length, categories]);

    const filteredSkills = useMemo(() => {
        const keyword = search.trim().toLocaleLowerCase();
        return (catalog?.skills || []).filter((skill) => {
            if (activeCategory !== "all" && skill.tag !== activeCategory) return false;
            if (availabilityFilter === "effective" && !skill.effectiveAvailable) return false;
            if (availabilityFilter === "disabled" && skill.effectiveAvailable) return false;
            if (availabilityFilter === "category-disabled" && skill.categoryAvailable) return false;
            if (!keyword) return true;
            return [skill.skillName, skill.skillId, skill.description, skill.authorName, skill.version].join(" ").toLocaleLowerCase().includes(keyword);
        });
    }, [activeCategory, availabilityFilter, catalog?.skills, search]);

    const sections = useMemo(() => categories
        .filter((category) => activeCategory === "all" || category.value === activeCategory)
        .map((category) => ({ category, skills: filteredSkills.filter((skill) => skill.tag === category.value) }))
        .filter((section) => activeCategory !== "all" || section.skills.length > 0), [activeCategory, categories, filteredSkills]);

    const visibleIds = filteredSkills.map((skill) => skill.skillId);
    const visibleSelectedCount = visibleIds.filter((id) => selectedIds.has(id)).length;
    const allVisibleSelected = visibleIds.length > 0 && visibleSelectedCount === visibleIds.length;

    const onTabKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
        if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
        event.preventDefault();
        const current = categoryTabs.findIndex((category) => category.value === activeCategory);
        const next = event.key === "Home"
            ? 0
            : event.key === "End"
                ? categoryTabs.length - 1
                : (current + (event.key === "ArrowRight" ? 1 : -1) + categoryTabs.length) % categoryTabs.length;
        setActiveCategory(categoryTabs[next].value);
        tabsRef.current?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus();
    };

    const toggleVisible = (checked: boolean) => {
        setSelectedIds((current) => {
            const next = new Set(current);
            if (!checked) {
                visibleIds.forEach((id) => next.delete(id));
                return next;
            }
            if (next.size + visibleIds.filter((id) => !next.has(id)).length > 200) {
                message.warning("单次最多选择 200 个技能");
                return current;
            }
            visibleIds.forEach((id) => next.add(id));
            return next;
        });
    };

    const toggleOneSelection = (id: string, checked: boolean) => {
        setSelectedIds((current) => {
            const next = new Set(current);
            if (checked) {
                if (next.size >= 200) {
                    message.warning("单次最多选择 200 个技能");
                    return current;
                }
                next.add(id);
            } else {
                next.delete(id);
            }
            return next;
        });
    };

    const saveSkills = async (ids: string[], available: boolean, batch: boolean) => {
        if (batch) setBatchSaving(true);
        else setSavingIds(new Set(ids));
        try {
            applyCatalog(await setAdminSkillAvailability(ids, available));
            message.success(`${ids.length} 个技能已${available ? "开启" : "关闭"}`);
            if (batch) setSelectedIds(new Set());
        } catch (error) {
            message.error(error instanceof Error ? error.message : "更新技能状态失败");
            await reload();
            throw error;
        } finally {
            if (batch) setBatchSaving(false);
            else setSavingIds(new Set());
        }
    };

    const runBatch = (available: boolean) => {
        const ids = [...selectedIds];
        if (ids.length === 0) return;
        if (available) {
            void saveSkills(ids, true, true).catch(() => undefined);
            return;
        }
        modal.confirm({
            title: `关闭已选的 ${ids.length} 个公共技能？`,
            content: "这些技能会立即从技能广场、我的技能、收藏和运行目录中隐藏，已有加入与收藏记录会保留。",
            okText: "确认关闭",
            okButtonProps: { danger: true },
            cancelText: "取消",
            onOk: () => saveSkills(ids, false, true),
        });
    };

    const saveCategory = async (category: AdminSkillCategory, available: boolean) => {
        setSavingCategory(category.value);
        try {
            applyCatalog(await setAdminSkillCategoryAvailability(category.value, available));
            message.success(`${category.label}已${available ? "开启" : "关闭"}`);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "更新分类状态失败");
            await reload();
            throw error;
        } finally {
            setSavingCategory("");
        }
    };

    const changeCategory = (category: AdminSkillCategory, available: boolean) => {
        if (available) {
            void saveCategory(category, true).catch(() => undefined);
            return;
        }
        modal.confirm({
            title: `关闭“${category.label}”分类？`,
            content: `分类下 ${category.totalCount} 个平台公共技能将整体停用；子项开关值会保留，重新开启分类后自动恢复。`,
            okText: "确认关闭",
            okButtonProps: { danger: true },
            cancelText: "取消",
            onOk: () => saveCategory(category, false),
        });
    };

    return (
        <AdminPageFrame
            title="技能管理"
            description="分级管理技能广场中的平台内置与社区公共技能；用户自行创建的技能不受影响"
            scroll
            actions={(
                <>
                    <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void reload()}>刷新</Button>
                    <Button type="primary" icon={<UploadCloud className="size-4" />} disabled={!catalog} onClick={() => setInstallOpen(true)}>安装技能</Button>
                </>
            )}
        >
            <div className="admin-skills-page">
                {!catalog?.globalAvailable && !loading ? (
                    <div className="admin-skills-global-warning">
                        <ShieldAlert className="size-4 shrink-0" aria-hidden="true" />
                        <div><strong>用户技能库总开关当前已关闭。</strong> 本页配置仍会保存，但所有技能能力均保持不可用。</div>
                    </div>
                ) : null}

                <div ref={tabsRef} className="admin-skills-tabs" role="tablist" aria-label="技能分类" onKeyDown={onTabKeyDown}>
                    {categoryTabs.map((category) => {
                        const active = activeCategory === category.value;
                        const Icon = category.value === "all" ? LayoutGrid : skillCategoryIconOf(category.value);
                        return (
                            <button
                                key={category.value}
                                type="button"
                                role="tab"
                                tabIndex={active ? 0 : -1}
                                aria-selected={active}
                                className={cn("admin-skills-tab", active && "is-active", category.value !== "all" && !category.available && "is-disabled-category")}
                                onClick={() => setActiveCategory(category.value)}
                            >
                                <Icon className="size-4" aria-hidden="true" />
                                <span>{category.label}</span>
                                <span className="admin-skills-tab-count">{category.totalCount}</span>
                                {category.value !== "all" && !category.available ? <span className="admin-skills-tab-state">已关闭</span> : null}
                            </button>
                        );
                    })}
                </div>

                <div className="admin-skills-toolbar">
                    <Input allowClear prefix={<Search className="size-4 text-foreground/40" />} className="admin-skills-search" value={search} placeholder="搜索技能名称、ID、作者或版本" aria-label="搜索平台公共技能" onChange={(event) => setSearch(event.target.value)} />
                    <Select<AvailabilityFilter>
                        className="admin-skills-status-filter"
                        aria-label="按技能状态筛选"
                        value={availabilityFilter}
                        onChange={setAvailabilityFilter}
                        options={[
                            { value: "all", label: "全部状态" },
                            { value: "effective", label: "有效开启" },
                            { value: "disabled", label: "当前停用" },
                            { value: "category-disabled", label: "随分类停用" },
                        ]}
                    />
                    <div className="admin-skills-batch-actions">
                        <Checkbox checked={allVisibleSelected} indeterminate={visibleSelectedCount > 0 && !allVisibleSelected} onChange={(event) => toggleVisible(event.target.checked)}>全选当前结果</Checkbox>
                        <span className="admin-skills-selected-count">已选 {selectedIds.size}</span>
                        <Button disabled={selectedIds.size === 0} loading={batchSaving} onClick={() => runBatch(true)}>批量开启</Button>
                        <Button danger disabled={selectedIds.size === 0} loading={batchSaving} onClick={() => runBatch(false)}>批量关闭</Button>
                    </div>
                </div>

                {loading && !catalog ? <AdminSkillsSkeleton /> : sections.length ? (
                    <div className="admin-skills-sections">
                        {sections.map(({ category, skills }) => (
                            <CategorySection
                                key={category.value}
                                category={category}
                                skills={skills}
                                globalAvailable={catalog?.globalAvailable ?? true}
                                savingCategory={savingCategory === category.value}
                                savingIds={savingIds}
                                selectedIds={selectedIds}
                                onCategoryAvailability={(available) => changeCategory(category, available)}
                                onSelect={toggleOneSelection}
                                onOpen={setActiveSkill}
                                onAvailability={(skill, available) => void saveSkills([skill.skillId], available, false).catch(() => undefined)}
                            />
                        ))}
                    </div>
                ) : <div className="admin-skills-empty">没有符合当前分类和筛选条件的平台公共技能</div>}
            </div>

            <SkillDetailModal skill={activeSkill} globalAvailable={catalog?.globalAvailable ?? true} categories={categories} onClose={() => setActiveSkill(null)} />
            <SkillInstallModal
                variant="admin"
                open={installOpen}
                categories={categories}
                onClose={() => setInstallOpen(false)}
                onInstalled={(result) => {
                    applyCatalog(result);
                    setInstallOpen(false);
                }}
            />
        </AdminPageFrame>
    );
}

function CategorySection({ category, skills, globalAvailable, savingCategory, savingIds, selectedIds, onCategoryAvailability, onSelect, onOpen, onAvailability }: {
    category: AdminSkillCategory;
    skills: AdminSkillItem[];
    globalAvailable: boolean;
    savingCategory: boolean;
    savingIds: Set<string>;
    selectedIds: Set<string>;
    onCategoryAvailability: (available: boolean) => void;
    onSelect: (id: string, checked: boolean) => void;
    onOpen: (skill: AdminSkillItem) => void;
    onAvailability: (skill: AdminSkillItem, available: boolean) => void;
}) {
    const Icon = skillCategoryIconOf(category.value);
    return (
        <section className="admin-skills-section" data-category={category.value} aria-labelledby={`admin-skills-category-${category.value}`}>
            <header className="admin-skills-section-heading">
                <div className="admin-skills-section-title">
                    <span className="admin-skills-category-icon" aria-hidden="true"><Icon /></span>
                    <div>
                        <div className="admin-skills-section-title-line">
                            <h2 id={`admin-skills-category-${category.value}`}>{category.label}</h2>
                            <AdminStatusBadge label={category.available ? "分类已开启" : "分类已关闭"} tone={category.available ? "success" : "warning"} />
                        </div>
                        <p>共 {category.totalCount} 项，当前有效开启 {category.availableCount} 项</p>
                    </div>
                </div>
                <div className="admin-skills-category-switch">
                    <span>分类总开关</span>
                    <Switch loading={savingCategory} checked={category.available} aria-label={`${category.label}分类总开关`} onChange={onCategoryAvailability} />
                </div>
            </header>
            {skills.length ? (
                <div className="admin-skills-grid">
                    {skills.map((skill) => (
                        <AdminSkillCard
                            key={skill.skillId}
                            skill={skill}
                            categoryLabel={category.label}
                            globalAvailable={globalAvailable}
                            selected={selectedIds.has(skill.skillId)}
                            saving={savingIds.has(skill.skillId)}
                            onSelect={(checked) => onSelect(skill.skillId, checked)}
                            onOpen={() => onOpen(skill)}
                            onAvailability={(available) => onAvailability(skill, available)}
                        />
                    ))}
                </div>
            ) : <div className="admin-skills-section-empty">当前分类没有符合筛选条件的技能</div>}
        </section>
    );
}

function AdminSkillCard({ skill, categoryLabel, globalAvailable, selected, saving, onSelect, onOpen, onAvailability }: {
    skill: AdminSkillItem;
    categoryLabel: string;
    globalAvailable: boolean;
    selected: boolean;
    saving: boolean;
    onSelect: (checked: boolean) => void;
    onOpen: () => void;
    onAvailability: (available: boolean) => void;
}) {
    const Icon = skillCategoryIconOf(skill.tag);
    return (
        <article className={cn("admin-skill-card", selected && "is-selected", !skill.effectiveAvailable && "is-unavailable")} data-category={skill.tag}>
            <div className="admin-skill-card-top">
                <Checkbox checked={selected} aria-label={`选择 ${skill.skillName}`} onChange={(event) => onSelect(event.target.checked)} />
                <span className="admin-skill-card-icon" aria-hidden="true"><Icon /></span>
                <SkillAvailabilityBadge skill={skill} globalAvailable={globalAvailable} />
            </div>
            <button type="button" className="admin-skill-card-content" aria-label={`查看 ${skill.skillName} 管理详情`} onClick={onOpen}>
                <h3>{skill.skillName}</h3>
                <code>{skill.skillId}</code>
                <p>{skill.description || "暂无技能简介"}</p>
                <div className="admin-skill-card-meta">
                    <span className="admin-skill-card-author" title={skill.authorName || "平台"}>{skill.authorName || "平台"}</span>
                    <span>v{skill.version || "1.0"}</span>
                    <span className="admin-skill-card-category">{categoryLabel}</span>
                </div>
                <div className="admin-skill-card-metrics">
                    <span><Heart aria-hidden="true" />{formatCount(skill.likeCount)} 收藏</span>
                    <span><UsersRound aria-hidden="true" />{formatCount(skill.addedCount)} 加入</span>
                </div>
            </button>
            <div className="admin-skill-card-control">
                <div><strong>技能开关</strong><span>{skill.available ? "原始状态已开启" : "原始状态已关闭"}</span></div>
                <Switch loading={saving} checked={skill.available} aria-label={`${skill.skillName}技能开关`} onChange={onAvailability} />
            </div>
        </article>
    );
}

function SkillAvailabilityBadge({ skill, globalAvailable }: { skill: AdminSkillItem; globalAvailable: boolean }) {
    if (skill.effectiveAvailable) return <AdminStatusBadge label="有效开启" tone="success" />;
    if (!skill.available) return <AdminStatusBadge label="已停用" tone="neutral" />;
    if (!globalAvailable) return <AdminStatusBadge label="随总开关停用" tone="warning" />;
    return <AdminStatusBadge label="随分类停用" tone="warning" />;
}

function SkillDetailModal({ skill, globalAvailable, categories, onClose }: { skill: AdminSkillItem | null; globalAvailable: boolean; categories: AdminSkillCategory[]; onClose: () => void }) {
    if (!skill) return null;
    const Icon = skillCategoryIconOf(skill.tag);
    const category = categories.find((item) => item.value === skill.tag);
    return (
        <AdminModal
            centered
            title="技能管理详情"
            width="min(620px, calc(100vw - 32px))"
            open
            onCancel={onClose}
            footer={null}
            rootClassName="admin-skill-detail-modal"
            styles={{ body: { maxHeight: "min(72vh, 720px)", overflowX: "hidden", overflowY: "auto" } }}
        >
            <div className="admin-skill-detail" data-category={skill.tag}>
                <div className="admin-skill-detail-heading">
                    <span className="admin-skill-detail-icon" aria-hidden="true"><Icon /></span>
                    <div className="min-w-0"><h2>{skill.skillName}</h2><code>{skill.skillId}</code></div>
                    <SkillAvailabilityBadge skill={skill} globalAvailable={globalAvailable} />
                </div>
                <section><h3>技能简介</h3><p className="admin-skill-detail-description">{skill.description || "暂无技能简介"}</p></section>
                <section>
                    <h3>基本信息</h3>
                    <dl className="admin-skill-detail-grid">
                        <div><dt>分类</dt><dd>{category?.label || "其他"}</dd></div>
                        <div><dt>来源</dt><dd>{sourceLabel(skill.sourceType)}</dd></div>
                        <div><dt>作者</dt><dd>{skill.authorName || "平台"}</dd></div>
                        <div><dt>版本</dt><dd>v{skill.version || "1.0"}</dd></div>
                        <div><dt>收藏量</dt><dd>{formatCount(skill.likeCount)}</dd></div>
                        <div><dt>加入量</dt><dd>{formatCount(skill.addedCount)}</dd></div>
                        <div className="is-wide"><dt>更新时间</dt><dd className="inline-flex items-center gap-1.5"><CalendarClock className="size-3.5" aria-hidden="true" />{formatDate(skill.updatedAt)}</dd></div>
                    </dl>
                </section>
                <section>
                    <h3>三级可用状态</h3>
                    <div className="admin-skill-detail-statuses">
                        <div><span>用户技能库总开关</span><AdminStatusBadge label={globalAvailable ? "已开启" : "已关闭"} tone={globalAvailable ? "success" : "warning"} /></div>
                        <div><span>{category?.label || "所属分类"}总开关</span><AdminStatusBadge label={skill.categoryAvailable ? "已开启" : "已关闭"} tone={skill.categoryAvailable ? "success" : "warning"} /></div>
                        <div><span>技能原始开关</span><AdminStatusBadge label={skill.available ? "已开启" : "已关闭"} tone={skill.available ? "success" : "neutral"} /></div>
                    </div>
                </section>
            </div>
        </AdminModal>
    );
}

function AdminSkillsSkeleton() {
    return <div className="admin-skills-skeleton"><Skeleton active paragraph={{ rows: 1 }} /><div className="admin-skills-grid">{Array.from({ length: 8 }, (_, index) => <div key={index} className="admin-skill-card-skeleton" />)}</div></div>;
}

function sourceLabel(sourceType: string) {
    return ({ builtin: "平台内置", github: "GitHub", zip: "ZIP 技能包", markdown: "Markdown" } as Record<string, string>)[sourceType] || sourceType || "社区公共技能";
}

function formatCount(value: number) {
    return new Intl.NumberFormat("zh-CN", { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

function formatDate(value: string) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "未知时间";
    return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date);
}
