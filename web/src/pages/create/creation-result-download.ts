import { resourceIdFromStorageKey } from "@/services/api/resources";
import type { BrowserMediaDownload } from "@/services/browser-download";
import type { Asset, ImageAsset, VideoAsset } from "@/stores/use-asset-store";
import type { CreationResultEntry } from "./creation-result-urls";

const mimeExtensions: Record<string, string> = {
    "image/png": "png",
    "image/jpeg": "jpg",
    "image/webp": "webp",
    "image/svg+xml": "svg",
    "image/gif": "gif",
    "image/avif": "avif",
    "video/mp4": "mp4",
    "video/webm": "webm",
    "video/quicktime": "mov",
    "video/ogg": "ogv",
};
const mediaExtension = /\.(png|jpe?g|webp|svg|gif|avif|mp4|webm|mov|ogv)$/i;

/** 下载保留结果的稳定身份，不能用过滤空结果后的下标匹配 storageKey。 */
export function creationResultDownload(entry: CreationResultEntry, storageKeys: string[], assets: Asset[], isVideo: boolean, index: number, origin: string): BrowserMediaDownload {
    const asset = assets.find((candidate): candidate is ImageAsset | VideoAsset => {
        if (candidate.kind !== "image" && candidate.kind !== "video") return false;
        return candidate.data.storageKey === entry.key || (!storageKeys.includes(entry.key) && (candidate.kind === "image" ? candidate.data.dataUrl : candidate.data.url) === entry.url);
    });
    const storageKey = storageKeys.includes(entry.key) || resourceIdFromStorageKey(entry.key) ? entry.key : asset?.data.storageKey;
    const target = new URL(entry.url, origin);
    if (!resourceIdFromStorageKey(storageKey) && !(target.protocol === "blob:" || target.protocol === "data:" || (target.origin === origin && /^(https?:)$/.test(target.protocol)))) {
        throw new Error("无法下载此图片或视频，请先重新获取结果后重试");
    }
    const mimeType = asset?.data.mimeType || /^data:([^;,]+)/i.exec(entry.url)?.[1];
    const extension = mimeExtensions[mimeType?.split(";", 1)[0]?.trim().toLowerCase() || ""] || target.pathname.match(mediaExtension)?.[1]?.toLowerCase();
    const ordinal = storageKeys.indexOf(entry.key);
    const fallback = `${isVideo ? "生成视频" : "生成图片"}_${(ordinal >= 0 ? ordinal : index) + 1}`;
    const title =
        (asset?.title || fallback)
            .replace(mediaExtension, "")
            .replace(/[\\/:*?"<>|\u0000-\u001f]/g, "_")
            .trim()
            .slice(0, 100)
            .replace(/[. ]+$/g, "") || fallback;
    const name = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/i.test(title) ? `_${title}` : title;
    return { storageKey, url: entry.url, fileName: extension ? `${name}.${extension}` : name };
}
