import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useUserStore } from "@/stores/use-user-store";

export function usePreviewQuery<T>(key: readonly unknown[], load: (signal: AbortSignal) => Promise<T>, interval: number, enabled = true) {
    const userId = useUserStore((state) => state.user?.id);
    const [visible, setVisible] = useState(() => document.visibilityState !== "hidden");
    useEffect(() => {
        const update = () => setVisible(document.visibilityState !== "hidden");
        document.addEventListener("visibilitychange", update);
        return () => document.removeEventListener("visibilitychange", update);
    }, []);
    return useQuery({
        queryKey: ["admin-control-preview", userId, ...key],
        queryFn: ({ signal }) => load(signal),
        enabled: Boolean(userId) && enabled && visible,
        refetchInterval: interval > 0 && visible ? interval : false,
        refetchIntervalInBackground: false,
        refetchOnWindowFocus: "always",
        retry: false,
        staleTime: 0,
        gcTime: 0,
    });
}
