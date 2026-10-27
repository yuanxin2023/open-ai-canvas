import type { AgentSkillPresetPreview } from "@/services/agent-skill-presets";
import { AppModal } from "@/components/ui/product/app-modal";

export function AgentPresetReviewContents({ preview }: { preview: AgentSkillPresetPreview }) {
    const missing = preview.skills.filter((skill) => !skill.isAdded).length;
    return (
        <div className="space-y-3">
            <p>
                将选用「{preview.preset.name}」的 {preview.skills.length} 个技能，其中 {missing} 个需要加入我的技能库。
            </p>
            <ul className="space-y-2" aria-label="本次预设技能">
                {preview.skills.map((skill) => (
                    <li key={skill.skillId} className="flex items-center justify-between gap-3">
                        <span>{skill.skillName}</span>
                        <span className="text-caption text-muted-foreground">{skill.isAdded ? "已在技能库" : "待安装"}</span>
                    </li>
                ))}
            </ul>
            <p className="text-caption text-muted-foreground">确认只安装并选用这些技能。生成仍需单独提交，原有权限、审批和预算继续生效。</p>
        </div>
    );
}

export function AgentPresetConfirmation({ preview, busy, disabled, onConfirm, onCancel }: { preview: AgentSkillPresetPreview | null; busy: boolean; disabled?: boolean; onConfirm: () => void; onCancel: () => void }) {
    return (
        <AppModal
            open={Boolean(preview)}
            title="确认场景预设"
            okText="确认安装并选用"
            cancelText="取消"
            confirmLoading={busy}
            okButtonProps={{ disabled }}
            cancelButtonProps={{ disabled: busy }}
            closable={!busy}
            maskClosable={!busy}
            keyboard={!busy}
            onOk={onConfirm}
            onCancel={onCancel}
            destroyOnHidden
        >
            {preview ? <AgentPresetReviewContents preview={preview} /> : null}
        </AppModal>
    );
}
