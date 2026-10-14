import type { RuntimeResourcePolicy } from "@/services/api/auth";

export const resourcePolicyFields: Array<{ group: "resource"; name: keyof RuntimeResourcePolicy; label: string; extra: string; unit: string; max: number; min?: number }> = [
    { group: "resource", name: "resourceUploadMB", label: "普通资源单文件", extra: "素材上传和远程导入的完整文件业务上限，分片上传同样适用。", unit: "MB", max: 999 },
    { group: "resource", name: "generatedFileMB", label: "单个生成资源", extra: "上游生成响应和落库资源的单文件上限。", unit: "MB", max: 999 },
    { group: "resource", name: "dailyUploadMB", label: "每日文件总量", extra: "按 UTC 自然日累计上传、导入和生成资源落库。", unit: "MB", max: 999_999 },
    { group: "resource", name: "storedFileGB", label: "账号文件总量", extra: "上传、生成资源与会话附件合计，回收站中仍占用存储的文件也计入。", unit: "GB", max: 999 },
    { group: "resource", name: "structuredDataMB", label: "结构化数据总量", extra: "画布、素材和创作会话等结构化数据。", unit: "MB", max: 999_999 },
    { group: "resource", name: "taskDataGB", label: "任务数据总量", extra: "任务历史、结果和上游请求日志合计。", unit: "GB", max: 999 },
    { group: "resource", name: "assetCount", label: "素材数量", extra: "单账号可保存的素材记录数。", unit: "条", max: 999_999_999 },
    { group: "resource", name: "canvasCount", label: "画布数量", extra: "单账号可保存的画布数量。", unit: "个", max: 999_999_999 },
    { group: "resource", name: "taskCount", label: "任务历史数量", extra: "单账号保留的任务历史记录数。", unit: "条", max: 999_999_999 },
    { group: "resource", name: "apiCallLogCount", label: "请求日志数量", extra: "单账号保留的上游请求日志数。", unit: "条", max: 999_999_999 },
    { group: "resource", name: "recycleBinRetentionDays", label: "回收站自动清理时间", extra: "移入回收站后自动清理的天数，0 表示不自动清理。", unit: "天", min: 0, max: 365 },
];
