import { useEffect, useState } from "react";
import { loadRasterImage } from "@/lib/image-raster";
import type { PixelSize } from "@/lib/canvas/image-operation-plan";
import { imageToDataUrl } from "@/services/image-storage";

export function useImageToolSource(source: string, storageKey?: string) {
    const empty = { source, storageKey, previewUrl: "", size: null, error: "" };
    const [result, setResult] = useState<{ source: string; storageKey?: string; previewUrl: string; size: PixelSize | null; error: string }>(empty);
    useEffect(() => {
        const request = new AbortController();
        setResult({ source, storageKey, previewUrl: "", size: null, error: "" });
        void imageToDataUrl({ url: source, storageKey }, { signal: request.signal })
            .then(async (previewUrl) => {
                const image = await loadRasterImage(previewUrl, request.signal);
                if (!request.signal.aborted) setResult({ source, storageKey, previewUrl, size: { width: image.naturalWidth, height: image.naturalHeight }, error: "" });
            })
            .catch((error: unknown) => {
                if (!request.signal.aborted) setResult({ source, storageKey, previewUrl: "", size: null, error: error instanceof Error ? error.message : "图片读取失败" });
            });
        return () => request.abort();
    }, [source, storageKey]);
    return result.source === source && result.storageKey === storageKey ? result : empty;
}
