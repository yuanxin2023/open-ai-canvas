import { lazy, Suspense } from "react";
import { fillablePrompt } from "@/lib/inspirations/resolve";
import { useUserStore } from "@/stores/use-user-store";
import type { CreationMode } from "./creation-types";

const Hall = lazy(() => import("@/pages/inspirations"));
const Gallery = lazy(() => import("./creation-inspiration-tunnel").then((module) => ({ default: module.CreationInspirationTunnel })));
type Props = { mode: CreationMode; onStartPrompt: (mode: CreationMode, prompt: string) => void };

export function CreationInspirationModules({ mode, onStartPrompt }: Props) {
    const hallEnabled = useUserStore((state) => state.features.inspirationHallEnabled === true);
    const galleryEnabled = useUserStore((state) => state.features.inspirationGalleryEnabled === true);
    const scope = useUserStore((state) => state.user?.id || "guest");
    return (
        <>
            {galleryEnabled ? (
                <Suspense fallback={null}>
                    <Gallery key={`gallery:${scope}`} mode={mode} onStartPrompt={onStartPrompt} />
                </Suspense>
            ) : null}
            {hallEnabled ? (
                <Suspense fallback={<div role="status">正在准备灵感大厅…</div>}>
                    <Hall key={`hall:${scope}`} onUse={(item) => onStartPrompt(item.mode, fillablePrompt(item))} />
                </Suspense>
            ) : null}
        </>
    );
}
