import { useEffect, useState } from "react";
import { Alert, Button, Descriptions, Skeleton, Typography } from "antd";

import { formatCredits } from "@/constant/credits";
import { getAdminUserLedgerRedemption, type AdminUserLedgerRedemption } from "@/services/api/auth";
import { AdminModal } from "../ui/overlays";
import { AdminStatusBadge } from "./admin-ui";

export function UserLedgerRedemptionModal({ userId, entryId, onClose }: { userId: string; entryId: string; onClose: () => void }) {
    const [detail, setDetail] = useState<AdminUserLedgerRedemption | null>(null);
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(true);
    const [attempt, setAttempt] = useState(0);

    useEffect(() => {
        const controller = new AbortController();
        setDetail(null);
        setError("");
        setLoading(true);
        void getAdminUserLedgerRedemption(userId, entryId, controller.signal)
            .then((result) => {
                if (!controller.signal.aborted) setDetail(result);
            })
            .catch((reason: unknown) => {
                if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "读取核销详情失败");
            })
            .finally(() => {
                if (!controller.signal.aborted) setLoading(false);
            });
        return () => controller.abort();
    }, [userId, entryId, attempt]);

    const code = detail?.code;
    return (
        <AdminModal title="兑换码核销详情" open centered width={720} onCancel={onClose} footer={<Button onClick={onClose}>关闭</Button>}>
            {loading ? (
                <Skeleton active paragraph={{ rows: 6 }} />
            ) : error ? (
                <Alert type="error" showIcon title="核销详情加载失败" description={error} action={<Button onClick={() => setAttempt((value) => value + 1)}>重试</Button>} />
            ) : detail && code ? (
                <div className="space-y-4">
                    {!code.code ? <Alert type="info" showIcon title="完整兑换码已清理" description="仍可通过尾号、所属批次和核销信息追溯此次兑换。" /> : null}
                    <Typography.Paragraph className="!mb-0 break-all font-mono" copyable={code.code ? { text: code.code } : false}>
                        {code.code || `····${code.codeSuffix}`}
                    </Typography.Paragraph>
                    <Descriptions
                        bordered
                        size="small"
                        column={{ xs: 1, sm: 2 }}
                        items={[
                            { key: "status", label: "核销状态", children: <AdminStatusBadge label="已核销" tone="success" /> },
                            { key: "amount", label: "到账积分", children: `+${formatCredits(detail.amountMicrocredits)}` },
                            { key: "user", label: "核销用户", children: code.redeemedUsername ? `@${code.redeemedUsername}` : code.redeemedDisplayName || "未记录" },
                            { key: "userId", label: "用户 ID", children: <span className="break-all font-mono text-xs">{code.redeemedBy}</span> },
                            { key: "time", label: "核销时间", children: code.redeemedAt ? new Date(code.redeemedAt).toLocaleString("zh-CN", { hour12: false }) : "未记录" },
                            { key: "ip", label: "核销 IP", children: code.redeemedIp || "未记录" },
                            { key: "batch", label: "所属批次", span: 2, children: <span className="break-all font-mono text-xs">{detail.batch.id}</span> },
                            { key: "note", label: "批次备注", span: 2, children: <span className="whitespace-pre-wrap break-words">{detail.batch.note || "未备注"}</span> },
                        ]}
                    />
                </div>
            ) : null}
        </AdminModal>
    );
}
