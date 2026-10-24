import { Alert, Button } from "antd";
import type { ReactNode } from "react";

export function PreviewChoice<T extends string | number>({ label, options, value, onChange }: { label: string; options: readonly { value: T; label: string }[]; value: T; onChange: (value: T) => void }) {
    return (
        <div className="admin-preview-choice" role="group" aria-label={label}>
            {options.map((option) => (
                <button key={option.value} type="button" aria-pressed={option.value === value} onClick={() => onChange(option.value)}>
                    {option.label}
                </button>
            ))}
        </div>
    );
}

export function PreviewPanel({ title, description, actions, children, id }: { title: string; description?: string; actions?: ReactNode; children: ReactNode; id?: string }) {
    return (
        <section id={id} className="admin-preview-panel">
            <div className="admin-preview-heading">
                <div>
                    <h2>{title}</h2>
                    {description && <p>{description}</p>}
                </div>
                {actions}
            </div>
            {children}
        </section>
    );
}

export function PreviewError({ error, stale, retry }: { error: Error | null; stale: boolean; retry: () => void }) {
    if (!error) return null;
    return (
        <Alert
            type="error"
            showIcon
            title={stale ? "更新失败，当前显示上次成功的数据" : "读取控制预览失败"}
            description={error.message}
            action={
                <Button size="small" onClick={retry}>
                    重试
                </Button>
            }
        />
    );
}

export function PreviewEmpty({ children = "当前统计范围暂无数据" }: { children?: ReactNode }) {
    return <div className="admin-preview-empty">{children}</div>;
}
