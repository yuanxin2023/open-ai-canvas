import { useQuery } from "@tanstack/react-query";
import { useUserStore } from "@/stores/use-user-store";

export function usePreviewQuery<T>(key: readonly unknown[], load: (signal: AbortSignal) => Promise<T>, enabled = true) {
    const userId = useUserStore((state) => state.user?.id);
    return useQuery({
        queryKey: ["admin-control-preview", userId, ...key],
        queryFn: ({ signal }) => load(signal),
        enabled: Boolean(userId) && enabled,
        refetchOnWindowFocus: false,
        retry: false,
        staleTime: 0,
        gcTime: 0,
    });
}
