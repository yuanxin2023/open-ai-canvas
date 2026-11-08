import { useQueryClient } from "@tanstack/react-query";
import { App, Button } from "antd";
import axios from "axios";
import { useEffect, useRef } from "react";
import { useNavigate } from "react-router";

import { translate } from "@/i18n";
import { accountFileStorageUsageQueryKey } from "@/lib/account-storage-usage";
import { STORAGE_LIMIT_REASONS } from "@/lib/workspace-storage";
import { apiClient } from "@/services/api/request";

const NOTICE_KEY = "workspace-storage-limit";
const NOTICE_INTERVAL_MS = 20_000;

/**
 * 业务请求因容量不足被拒时，刷新用量并提供清理入口。
 * 只补充提醒，调用方原有的错误处理和提示保持不变。
 */
export function WorkspaceStorageLimitNotice() {
    const { notification } = App.useApp();
    const queryClient = useQueryClient();
    const navigate = useNavigate();
    const lastNoticeAt = useRef(0);

    useEffect(() => {
        const interceptor = apiClient.interceptors.response.use(undefined, (error: unknown) => {
            const reason = axios.isAxiosError<{ reason?: string }>(error) ? error.response?.data?.reason : undefined;
            if (reason && STORAGE_LIMIT_REASONS.has(reason)) {
                void queryClient.invalidateQueries({ queryKey: accountFileStorageUsageQueryKey });
                const now = Date.now();
                if (now - lastNoticeAt.current > NOTICE_INTERVAL_MS) {
                    lastNoticeAt.current = now;
                    notification.warning({
                        key: NOTICE_KEY,
                        title: translate("account.layout.storageNotice.fullTitle"),
                        description: "请清理素材或联系管理员调整配额。",
                        actions: (
                            <span style={{ display: "inline-flex", gap: "var(--space-2, 8px)" }}>
                                <Button
                                    size="small"
                                    onClick={() => {
                                        notification.destroy(NOTICE_KEY);
                                        navigate("/assets");
                                    }}
                                >
                                    {translate("account.layout.storageNotice.cleanUp")}
                                </Button>
                            </span>
                        ),
                    });
                }
            }
            return Promise.reject(error);
        });
        return () => apiClient.interceptors.response.eject(interceptor);
    }, [navigate, notification, queryClient]);

    return null;
}
