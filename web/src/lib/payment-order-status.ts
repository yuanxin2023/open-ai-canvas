export const paymentOrderStatus: Record<string, { label: string; tone: "neutral" | "success" | "warning" | "error" | "info" }> = {
    created: { label: "创建中", tone: "info" },
    pending: { label: "待支付", tone: "warning" },
    closing: { label: "关单中", tone: "warning" },
    closed: { label: "已关闭", tone: "neutral" },
    credited: { label: "已入账", tone: "success" },
    create_failed: { label: "下单失败", tone: "error" },
};
