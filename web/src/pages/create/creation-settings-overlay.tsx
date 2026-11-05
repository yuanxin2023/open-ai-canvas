import { Popover } from "antd";
import { cloneElement, useRef, useSyncExternalStore, type ComponentPropsWithRef, type ReactElement, type ReactNode } from "react";
import { AppDrawer } from "@/components/ui/product/app-drawer";

const mobileQuery = "(max-width: 767px)";
const subscribeViewport = (notify: () => void) => {
    const query = window.matchMedia(mobileQuery);
    query.addEventListener("change", notify);
    return () => query.removeEventListener("change", notify);
};
const mobileViewport = () => window.matchMedia(mobileQuery).matches;
const serverViewport = () => false;

export function CreationSettingsOverlay({
    open,
    onOpenChange,
    content,
    children,
    mobileDrawer,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    content: ReactNode;
    children: ReactElement<ComponentPropsWithRef<"button">>;
    mobileDrawer: boolean;
}) {
    const mobile = useSyncExternalStore(subscribeViewport, mobileViewport, serverViewport);
    const triggerRef = useRef<HTMLButtonElement>(null);
    const trigger = cloneElement(children, { ref: triggerRef });
    if (mobile && mobileDrawer) {
        return (
            <>
                {trigger}
                <AppDrawer
                    title="图片生成设置"
                    placement="bottom"
                    size="85dvh"
                    open={open}
                    focusable={{ focusTriggerAfterClose: false }}
                    onClose={() => {
                        onOpenChange(false);
                        requestAnimationFrame(() => triggerRef.current?.focus({ preventScroll: true }));
                    }}
                    rootClassName="creation-settings-drawer app-user-overlays"
                >
                    {content}
                </AppDrawer>
            </>
        );
    }
    return (
        <Popover
            open={open}
            onOpenChange={onOpenChange}
            trigger="click"
            placement="bottom"
            arrow={false}
            classNames={{ root: "creation-control-popover", container: "creation-control-popover-surface", content: "creation-control-popover-content" }}
            content={content}
        >
            {trigger}
        </Popover>
    );
}
