import { App, Button, Drawer, Form, Input, Select as AntSelect, Tabs } from "antd";
import { useEffect, useState } from "react";

import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { normalizeUsername, usernameValidationMessage } from "@/lib/username";
import { Select } from "@/pages/admin/ui/controls";
import {
    createAdministrator,
    listAdminUsers,
    promoteAdministrator,
    type AdminManagedUser,
    type AdminUser,
    type LocalUser,
} from "@/services/api/auth";
import type { AdminLevel, AdminPermission } from "@/lib/admin-permissions";
import { AdminAccessFields } from "./users-drawer";

type AccessValues = {
    adminLevel: AdminLevel;
    permissions?: AdminPermission[];
};

type CreateValues = AccessValues & {
    username: string;
    email?: string;
    remark?: string;
    password: string;
    status: LocalUser["status"];
};

type PromoteValues = AccessValues & { userId: string };

function adminAccess(values: AccessValues) {
    return {
        level: values.adminLevel || "scoped",
        permissions: values.adminLevel === "full" ? [] : (values.permissions || []),
    } as const;
}

export function AdministratorCreateDrawer({ open, onClose, onCompleted }: { open: boolean; onClose: () => void; onCompleted: (user: AdminUser | AdminManagedUser) => void }) {
    const { message, modal } = App.useApp();
    const [mode, setMode] = useState<"create" | "promote">("create");
    const [saving, setSaving] = useState(false);
    const [candidateSearch, setCandidateSearch] = useState("");
    const debouncedCandidateSearch = useDebouncedValue(candidateSearch);
    const [candidates, setCandidates] = useState<AdminUser[]>([]);
    const [candidateLoading, setCandidateLoading] = useState(false);
    const [createForm] = Form.useForm<CreateValues>();
    const [promoteForm] = Form.useForm<PromoteValues>();
    const createLevel = Form.useWatch("adminLevel", createForm);
    const promoteLevel = Form.useWatch("adminLevel", promoteForm);

    useEffect(() => {
        if (!open) return;
        setMode("create");
        setCandidateSearch("");
        setCandidates([]);
        createForm.resetFields();
        promoteForm.resetFields();
        createForm.setFieldsValue({ status: "active", adminLevel: "scoped", permissions: [] });
        promoteForm.setFieldsValue({ adminLevel: "scoped", permissions: [] });
    }, [createForm, open, promoteForm]);

    useEffect(() => {
        if (!open || mode !== "promote") return;
        let active = true;
        setCandidateLoading(true);
        void listAdminUsers({ keyword: debouncedCandidateSearch || undefined, page: 1, pageSize: 20 })
            .then((result) => active && setCandidates(result.users))
            .catch((error) => active && message.error(error instanceof Error ? error.message : "读取普通用户失败"))
            .finally(() => active && setCandidateLoading(false));
        return () => { active = false; };
    }, [debouncedCandidateSearch, message, mode, open]);

    const close = () => {
        if (saving) return;
        const touched = mode === "create" ? createForm.isFieldsTouched() : promoteForm.isFieldsTouched();
        if (!touched) {
            onClose();
            return;
        }
        modal.confirm({
            title: "放弃添加管理员？",
            content: "尚未保存的账号或权限配置将丢失。",
            okText: "放弃并关闭",
            cancelText: "继续编辑",
            okButtonProps: { danger: true },
            onOk: onClose,
        });
    };

    const save = async () => {
        setSaving(true);
        try {
            let completedUser: AdminUser | AdminManagedUser;
            if (mode === "create") {
                const values = await createForm.validateFields();
                const result = await createAdministrator({
                    username: normalizeUsername(values.username),
                    email: values.email?.trim() || "",
                    remark: values.remark?.trim() || "",
                    password: values.password,
                    status: values.status,
                    adminAccess: adminAccess(values),
                });
                completedUser = result.user;
                message.success("管理员账号已创建");
            } else {
                const values = await promoteForm.validateFields();
                const result = await promoteAdministrator({ userId: values.userId, adminAccess: adminAccess(values) });
                completedUser = result.user;
                message.success("用户已晋升为管理员，原登录态已撤销");
            }
            onCompleted(completedUser);
            onClose();
        } catch (error) {
            if (error instanceof Error) message.error(error.message);
        } finally {
            setSaving(false);
        }
    };

    return (
        <Drawer
            title="添加管理员"
            open={open}
            size="min(560px, 100vw)"
            onClose={close}
            mask={{ closable: !saving }}
            destroyOnHidden
            extra={<Button type="primary" loading={saving} onClick={() => void save()}>保存</Button>}
        >
            <Tabs
                activeKey={mode}
                onChange={(key) => setMode(key as "create" | "promote")}
                items={[
                    {
                        key: "create",
                        label: "创建新账号",
                        children: (
                            <Form form={createForm} layout="vertical" requiredMark={false}>
                                <Form.Item name="username" label="用户名" rules={[{ validator: (_, value?: string) => { const error = usernameValidationMessage(value || ""); return error ? Promise.reject(new Error(error)) : Promise.resolve(); } }]}>
                                    <Input placeholder="3-9 位中文、英文字母或数字" />
                                </Form.Item>
                                <Form.Item name="email" label="邮箱" rules={[{ type: "email", message: "请输入有效邮箱" }]}>
                                    <Input placeholder="name@example.com" />
                                </Form.Item>
                                <Form.Item name="remark" label="备注" extra="仅管理员可见。">
                                    <Input.TextArea rows={3} maxLength={500} showCount />
                                </Form.Item>
                                <Form.Item name="password" label="初始密码" rules={[{ required: true, message: "请设置初始密码" }, { min: 8, message: "密码至少 8 位" }]}>
                                    <Input.Password autoComplete="new-password" />
                                </Form.Item>
                                <Form.Item name="status" label="账号状态">
                                    <Select options={[{ label: "已启用", value: "active" }, { label: "已停用", value: "disabled" }]} />
                                </Form.Item>
                                <AdminAccessFields level={createLevel} />
                            </Form>
                        ),
                    },
                    {
                        key: "promote",
                        label: "晋升现有用户",
                        children: (
                            <Form form={promoteForm} layout="vertical" requiredMark={false}>
                                <Form.Item name="userId" label="普通用户" rules={[{ required: true, message: "请选择要晋升的用户" }]} extra="晋升会保留账号数据，并撤销该账号当前的全部登录态。">
                                    <AntSelect
                                        showSearch
                                        filterOption={false}
                                        loading={candidateLoading}
                                        placeholder="搜索用户名、邮箱或备注"
                                        onSearch={setCandidateSearch}
                                        options={candidates.map((user) => ({
                                            value: user.id,
                                            label: `${user.username}${user.email ? ` · ${user.email}` : ""}${user.status === "disabled" ? " · 已停用" : ""}`,
                                        }))}
                                    />
                                </Form.Item>
                                <AdminAccessFields level={promoteLevel} />
                            </Form>
                        ),
                    },
                ]}
            />
        </Drawer>
    );
}
