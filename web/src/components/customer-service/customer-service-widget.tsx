import { Headphones, MessageCircle } from "lucide-react";
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import { useLocation } from "react-router";

import { cn } from "@/lib/utils";
import { CUSTOMER_SERVICE_OPEN_EVENT, getPublicCustomerService, type PublicCustomerService } from "@/services/api/customer-service";

declare global {
    interface Window {
        chatwootSettings?: Record<string, unknown>;
        chatwootSDK?: { run: (input: { websiteToken: string; baseUrl: string }) => void };
        $chatwoot?: {
            toggle: (state?: "open" | "close") => void;
            toggleBubbleVisibility: (state: "show" | "hide") => void;
        };
    }
}
type ButtonPoint = { x: number; y: number };
type DragState = { pointerId: number; startX: number; startY: number; originX: number; originY: number; moved: boolean };

const CHATWOOT_SCRIPT_ID = "chatwoot-sdk-script";
const CHATWOOT_BASE_URL = (import.meta.env.VITE_CHATWOOT_BASE_URL || "https://app.chatwoot.com").replace(/\/$/, "");
const CHATWOOT_WEBSITE_TOKEN = import.meta.env.VITE_CHATWOOT_WEBSITE_TOKEN || "kHwNmDAfhAc12xwgLEFxmrsU";
const POSITION_STORAGE_KEY = "open-ai-canvas:customer-service-position";

function viewportEnabled(setting: PublicCustomerService) {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(max-width: 767px)").matches ? setting.mobileEnabled : setting.desktopEnabled;
}

function clampPoint(point: ButtonPoint, element: HTMLElement): ButtonPoint {
    const margin = 8;
    return {
        x: Math.min(Math.max(margin, point.x), Math.max(margin, window.innerWidth - element.offsetWidth - margin)),
        y: Math.min(Math.max(margin, point.y), Math.max(margin, window.innerHeight - element.offsetHeight - margin)),
    };
}

function readStoredPoint(): ButtonPoint | null {
    try {
        const value = JSON.parse(localStorage.getItem(POSITION_STORAGE_KEY) || "null") as ButtonPoint | null;
        return value && Number.isFinite(value.x) && Number.isFinite(value.y) ? value : null;
    } catch {
        return null;
    }
}

function defaultPositionStyle(setting: PublicCustomerService): CSSProperties {
    const horizontal = setting.position.endsWith("right") ? { right: setting.offsetX } : { left: setting.offsetX };
    const vertical = setting.position.startsWith("bottom") ? { bottom: setting.offsetY } : { top: setting.offsetY };
    return { ...horizontal, ...vertical };
}

function ensureChatwoot(position: PublicCustomerService["position"], onReady: () => void) {
    const ready = () => {
        window.$chatwoot?.toggleBubbleVisibility("hide");
        onReady();
    };
    window.addEventListener("chatwoot:ready", ready, { once: true });
    if (window.$chatwoot) {
        window.removeEventListener("chatwoot:ready", ready);
        ready();
        return () => window.removeEventListener("chatwoot:ready", ready);
    }

    window.chatwootSettings = {
        hideMessageBubble: true,
        position: position.endsWith("left") ? "left" : "right",
        type: "standard",
        darkMode: "auto",
        useBrowserLanguage: true,
    };
    const existing = document.getElementById(CHATWOOT_SCRIPT_ID) as HTMLScriptElement | null;
    if (!existing) {
        const script = document.createElement("script");
        script.id = CHATWOOT_SCRIPT_ID;
        script.src = `${CHATWOOT_BASE_URL}/packs/js/sdk.js`;
        script.async = true;
        script.onload = () => window.chatwootSDK?.run({ websiteToken: CHATWOOT_WEBSITE_TOKEN, baseUrl: CHATWOOT_BASE_URL });
        document.head.appendChild(script);
    } else if (window.chatwootSDK) {
        window.chatwootSDK.run({ websiteToken: CHATWOOT_WEBSITE_TOKEN, baseUrl: CHATWOOT_BASE_URL });
    }
    return () => window.removeEventListener("chatwoot:ready", ready);
}

export function CustomerServiceWidget() {
    const location = useLocation();
    const [setting, setSetting] = useState<PublicCustomerService | null>(null);
    const [point, setPoint] = useState<ButtonPoint | null>(null);
    const [viewportAllowed, setViewportAllowed] = useState(true);
    const [sdkReady, setSdkReady] = useState(Boolean(window.$chatwoot));
    const buttonRef = useRef<HTMLButtonElement>(null);
    const dragRef = useRef<DragState | null>(null);
    const suppressClickRef = useRef(false);
    const pendingOpenRef = useRef(false);

    const load = useCallback(async () => {
        try {
            const result = await getPublicCustomerService();
            setSetting(result);
            setViewportAllowed(viewportEnabled(result));
        } catch {
            setSetting(null);
        }
    }, []);

    useEffect(() => {
        void load();
        window.addEventListener("customer-service-config-updated", load);
        return () => window.removeEventListener("customer-service-config-updated", load);
    }, [load, location.pathname]);

    useEffect(() => {
        if (!setting) return;
        const media = window.matchMedia("(max-width: 767px)");
        const update = () => setViewportAllowed(media.matches ? setting.mobileEnabled : setting.desktopEnabled);
        update();
        media.addEventListener("change", update);
        return () => media.removeEventListener("change", update);
    }, [setting]);

    const serviceAvailable = Boolean(setting?.enabled && viewportAllowed && !location.pathname.startsWith("/admin"));
    const buttonVisible = Boolean(serviceAvailable && setting?.floatingButtonEnabled);
    const requestOpenChat = useCallback(() => {
        if (sdkReady && window.$chatwoot) window.$chatwoot.toggle("open");
        else pendingOpenRef.current = true;
    }, [sdkReady]);

    useEffect(() => {
        const handleOpen = () => {
            if (serviceAvailable) requestOpenChat();
        };
        window.addEventListener(CUSTOMER_SERVICE_OPEN_EVENT, handleOpen);
        return () => window.removeEventListener(CUSTOMER_SERVICE_OPEN_EVENT, handleOpen);
    }, [requestOpenChat, serviceAvailable]);

    useEffect(() => {
        if (!serviceAvailable) {
            window.$chatwoot?.toggle("close");
            return;
        }
        return ensureChatwoot(setting!.position, () => {
            setSdkReady(true);
            if (pendingOpenRef.current) {
                pendingOpenRef.current = false;
                window.$chatwoot?.toggle("open");
            }
        });
    }, [serviceAvailable, setting?.position]);

    useLayoutEffect(() => {
        if (!buttonVisible || !setting?.draggable || !buttonRef.current) {
            setPoint(null);
            return;
        }
        const stored = readStoredPoint();
        if (stored) setPoint(clampPoint(stored, buttonRef.current));
    }, [buttonVisible, setting?.displayType, setting?.draggable]);

    useEffect(() => {
        if (!point || !buttonRef.current) return;
        const update = () => buttonRef.current && setPoint((current) => (current ? clampPoint(current, buttonRef.current!) : current));
        window.addEventListener("resize", update);
        return () => window.removeEventListener("resize", update);
    }, [point]);

    if (!buttonVisible || !setting) return null;

    const customImage = setting.displayType === "custom-image" && setting.imageUrl;
    const square = setting.displayType === "circle" || setting.displayType === "custom-image";
    const buttonSize = Number.isFinite(setting.buttonSize) && setting.buttonSize >= 20 && setting.buttonSize <= 96 ? setting.buttonSize : 56;
    const iconSize = Math.max(12, Math.min(28, Math.round(buttonSize * 0.38)));
    const startDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
        if (!setting.draggable || !buttonRef.current) return;
        const rect = buttonRef.current.getBoundingClientRect();
        buttonRef.current.setPointerCapture(event.pointerId);
        dragRef.current = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, originX: rect.left, originY: rect.top, moved: false };
    };
    const moveDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
        const drag = dragRef.current;
        if (!drag || drag.pointerId !== event.pointerId || !buttonRef.current) return;
        const deltaX = event.clientX - drag.startX;
        const deltaY = event.clientY - drag.startY;
        if (!drag.moved && Math.hypot(deltaX, deltaY) < 5) return;
        drag.moved = true;
        event.preventDefault();
        setPoint(clampPoint({ x: drag.originX + deltaX, y: drag.originY + deltaY }, buttonRef.current));
    };
    const endDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
        const drag = dragRef.current;
        if (!drag || drag.pointerId !== event.pointerId) return;
        suppressClickRef.current = drag.moved;
        dragRef.current = null;
        if (buttonRef.current?.hasPointerCapture(event.pointerId)) buttonRef.current.releasePointerCapture(event.pointerId);
        if (drag.moved && buttonRef.current) {
            const finalPoint = clampPoint({ x: drag.originX + event.clientX - drag.startX, y: drag.originY + event.clientY - drag.startY }, buttonRef.current);
            setPoint(finalPoint);
            localStorage.setItem(POSITION_STORAGE_KEY, JSON.stringify(finalPoint));
        }
    };
    const openChat = () => {
        if (suppressClickRef.current) {
            suppressClickRef.current = false;
            return;
        }
        requestOpenChat();
    };

    return (
        <button
            ref={buttonRef}
            type="button"
            aria-label={setting.label}
            title={setting.draggable ? `${setting.label}（可拖动）` : setting.label}
            className={cn(
                "fixed z-[1200] inline-flex touch-none select-none items-center justify-center gap-2 text-white shadow-[0_14px_36px_rgba(0,0,0,0.28)] transition-[filter,transform] hover:brightness-110 active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/85 focus-visible:ring-offset-2",
                setting.displayType === "circle" && "rounded-full",
                setting.displayType === "pill" && "rounded-full",
                setting.displayType === "icon-text" && "rounded-xl",
                setting.displayType === "custom-image" && "overflow-hidden rounded-2xl",
                setting.draggable && "cursor-grab active:cursor-grabbing",
            )}
            style={{
                ...(point ? { left: point.x, top: point.y } : defaultPositionStyle(setting)),
                width: square ? buttonSize : undefined,
                height: buttonSize,
                paddingInline: square ? undefined : Math.round(buttonSize * 0.36),
                backgroundColor: customImage ? "transparent" : setting.color,
            }}
            onPointerDown={startDrag}
            onPointerMove={moveDrag}
            onPointerUp={endDrag}
            onPointerCancel={endDrag}
            onClick={openChat}
        >
            {customImage ? (
                <img src={setting.imageUrl} alt="" draggable={false} className="size-full object-contain" />
            ) : (
                <>
                    {setting.displayType === "pill" ? <Headphones size={iconSize} aria-hidden="true" /> : <MessageCircle size={iconSize} aria-hidden="true" />}
                    {setting.displayType !== "circle" ? <span className="whitespace-nowrap text-sm font-semibold">{setting.label}</span> : null}
                </>
            )}
        </button>
    );
}
