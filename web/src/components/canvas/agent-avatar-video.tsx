import { useEffect, useRef, useSyncExternalStore } from "react";

const reducedMotionQuery = "(prefers-reduced-motion: reduce)";
function subscribeReducedMotion(onChange: () => void) {
    const media = window.matchMedia(reducedMotionQuery);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
}
const readReducedMotion = () => window.matchMedia(reducedMotionQuery).matches;

// Upstream mascot clips use a green background; key it out into a transparent canvas.
export function AgentAvatarVideo({ src, loop = true, paused = false, onError }: { src: string; loop?: boolean; paused?: boolean; onError?: () => void }) {
    const reducedMotion = useSyncExternalStore(subscribeReducedMotion, readReducedMotion, () => false);
    const shouldPause = paused || reducedMotion;
    const videoRef = useRef<HTMLVideoElement>(null);
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const errorRef = useRef(onError);
    errorRef.current = onError;

    useEffect(() => {
        const video = videoRef.current;
        const canvas = canvasRef.current;
        const context = canvas?.getContext("2d", { willReadFrequently: true });
        if (!video || !canvas || !context) {
            errorRef.current?.();
            return;
        }
        let animationFrame = 0;
        let videoFrame = 0;
        let disposed = false;
        let lastFrameTime = -1;
        const drawFrame = () => {
            if (disposed || video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA || video.videoWidth <= 0 || video.videoHeight <= 0 || video.currentTime === lastFrameTime) return;
            lastFrameTime = video.currentTime;
            try {
                const scale = Math.min(1, 320 / Math.max(video.videoWidth, video.videoHeight));
                const width = Math.max(1, Math.round(video.videoWidth * scale));
                const height = Math.max(1, Math.round(video.videoHeight * scale));
                if (canvas.width !== width || canvas.height !== height) {
                    canvas.width = width;
                    canvas.height = height;
                }
                context.drawImage(video, 0, 0, width, height);
                const image = context.getImageData(0, 0, width, height);
                const pixels = image.data;
                for (let index = 0; index < pixels.length; index += 4) {
                    const red = pixels[index];
                    const green = pixels[index + 1];
                    const blue = pixels[index + 2];
                    const greenLead = green - Math.max(red, blue);
                    if (green > 70 && greenLead > 12 && green > red * 1.12 && green > blue * 1.08) {
                        const opacity = Math.max(0, Math.min(1, (38 - greenLead) / 26));
                        pixels[index + 1] = Math.max(red, blue, green - Math.round((1 - opacity) * greenLead * 0.75));
                        pixels[index + 3] = Math.round(pixels[index + 3] * opacity);
                    }
                }
                context.putImageData(image, 0, 0);
            } catch {
                disposed = true;
                errorRef.current?.();
            }
        };
        const scheduleFrame = () => {
            if (disposed || shouldPause) return;
            if (typeof video.requestVideoFrameCallback === "function") {
                videoFrame = video.requestVideoFrameCallback(() => {
                    drawFrame();
                    scheduleFrame();
                });
            } else {
                animationFrame = requestAnimationFrame(() => {
                    drawFrame();
                    scheduleFrame();
                });
            }
        };
        video.addEventListener("loadeddata", drawFrame);
        video.addEventListener("seeked", drawFrame);
        drawFrame();
        scheduleFrame();
        return () => {
            disposed = true;
            video.removeEventListener("loadeddata", drawFrame);
            video.removeEventListener("seeked", drawFrame);
            if (animationFrame) cancelAnimationFrame(animationFrame);
            if (videoFrame && typeof video.cancelVideoFrameCallback === "function") video.cancelVideoFrameCallback(videoFrame);
        };
    }, [src, shouldPause]);

    useEffect(() => {
        const video = videoRef.current;
        if (!video) return;
        let active = true;
        if (shouldPause) video.pause();
        else
            void video.play().catch((error: unknown) => {
                if (active && !(error instanceof DOMException && error.name === "AbortError")) errorRef.current?.();
            });
        return () => {
            active = false;
        };
    }, [src, shouldPause]);

    return (
        <>
            <canvas ref={canvasRef} className="canvas-agent-keyed-video" style={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "contain", pointerEvents: "none", userSelect: "none" }} aria-hidden="true" />
            <video
                ref={videoRef}
                src={src}
                autoPlay={!shouldPause}
                muted
                loop={loop}
                playsInline
                preload="auto"
                aria-hidden="true"
                draggable={false}
                onError={() => errorRef.current?.()}
                className="canvas-agent-video-source"
                style={{ position: "absolute", width: 1, height: 1, opacity: 0, pointerEvents: "none" }}
            />
        </>
    );
}
