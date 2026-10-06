export type AdminLevel = "full" | "scoped";

export const ADMIN_PERMISSION_GROUPS = [
    { label: "经营分析", items: [
        { permission: "admin.analytics.overview", label: "数据概览", path: "/admin" },
        { permission: "admin.analytics.api_logs", label: "请求明细", path: "/admin/logs" },
    ] },
    { label: "平台资源", items: [
        { permission: "admin.platform.channels", label: "系统渠道", path: "/admin/channels" },
        { permission: "admin.platform.logical_models", label: "前台模型", path: "/admin/models" },
        { permission: "admin.platform.plugins", label: "插件管理", path: "/admin/plugins" },
        { permission: "admin.platform.skills", label: "技能管理", path: "/admin/skills" },
        { permission: "admin.platform.prompt_templates", label: "提示词模板", path: "/admin/prompt-templates" },
    ] },
    { label: "用户与服务", items: [
        { permission: "admin.users.accounts", label: "用户管理", path: "/admin/users" },
        { permission: "admin.users.customer_service", label: "客服配置", path: "/admin/customer-service" },
        { permission: "admin.users.agent_lessons", label: "Agent 记忆", path: "/admin/agent-lessons" },
    ] },
    { label: "商品运营", items: [
        { permission: "admin.commerce.products", label: "商品管理", path: "/admin/product-operations" },
        { permission: "admin.commerce.redeem_codes", label: "兑换码", path: "/admin/redemption-codes" },
    ] },
    { label: "财务管理", items: [
        { permission: "admin.finance.payment_providers", label: "支付渠道", path: "/admin/payments" },
        { permission: "admin.finance.payment_orders", label: "支付订单", path: "/admin/payment-orders" },
        { permission: "admin.finance.reconciliation", label: "支付对账", path: "/admin/payment-reconciliation" },
        { permission: "admin.finance.credits", label: "积分运营", path: "/admin/credit-operations" },
        { permission: "admin.finance.referrals", label: "邀请返利", path: "/admin/referrals" },
    ] },
    { label: "内容与通知", items: [
        { permission: "admin.content.inspirations", label: "提示词运营", path: "/admin/inspirations" },
        { permission: "admin.content.announcements", label: "系统公告", path: "/admin/announcements" },
        { permission: "admin.content.banner_announcements", label: "常驻通知", path: "/admin/banner-announcements" },
    ] },
    { label: "系统配置", items: [
        { permission: "admin.settings.appearance", label: "站点及外观", path: "/admin/settings/appearance" },
        { permission: "admin.settings.features", label: "功能开放", path: "/admin/settings/features" },
        { permission: "admin.settings.drawing_engine", label: "绘图工具", path: "/admin/settings/drawing-engine" },
        { permission: "admin.settings.system_performance", label: "系统性能", path: "/admin/settings/system-performance" },
        { permission: "admin.settings.access", label: "登录与注册", path: "/admin/settings/access" },
        { permission: "admin.settings.email", label: "邮件服务", path: "/admin/settings/email" },
        { permission: "admin.settings.ark_private_assets", label: "方舟素材库", path: "/admin/settings/ark-private-assets" },
        { permission: "admin.settings.response_interception", label: "模型响应拦截", path: "/admin/settings/response-interception" },
        { permission: "admin.settings.third_party", label: "第三方参数配置", path: "/admin/settings/third-party" },
        { permission: "admin.settings.system_update", label: "系统更新", path: "/admin/settings/system-update" },
    ] },
    { label: "存储与备份", items: [
        { permission: "admin.storage.resources", label: "存储资源", path: "/admin/resources" },
        { permission: "admin.storage.runtime_policy", label: "资源与策略", path: "/admin/settings/runtime-policy" },
        { permission: "admin.storage.service", label: "存储服务", path: "/admin/settings/storage" },
    ] },
] as const;

export type AdminPermission = typeof ADMIN_PERMISSION_GROUPS[number]["items"][number]["permission"];

export type AdminAccess = {
    level: AdminLevel;
    permissions: AdminPermission[];
};

type AdminPermissionItem = typeof ADMIN_PERMISSION_GROUPS[number]["items"][number];

export const ALL_ADMIN_PERMISSIONS = ADMIN_PERMISSION_GROUPS.reduce<AdminPermission[]>((permissions, group) => {
    permissions.push(...group.items.map((item) => item.permission));
    return permissions;
}, []);

const ALL_ADMIN_PERMISSION_ITEMS = ADMIN_PERMISSION_GROUPS.reduce<AdminPermissionItem[]>((items, group) => {
    items.push(...group.items);
    return items;
}, []);

export function hasAdminPermission(access: AdminAccess | undefined, permission: AdminPermission) {
    return access?.level === "full" || Boolean(access?.permissions.includes(permission));
}

export function firstAdminPath(access: AdminAccess | undefined) {
    return ALL_ADMIN_PERMISSION_ITEMS.find((item) => hasAdminPermission(access, item.permission))?.path || "/";
}
