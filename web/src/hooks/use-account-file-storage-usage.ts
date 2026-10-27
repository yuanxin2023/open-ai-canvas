import { useUserStore } from "@/stores/use-user-store";
import { useQuery } from "@tanstack/react-query";

import { accountFileStorageUsageQueryKey } from "@/lib/account-storage-usage";
import { getAccountFileStorageUsage } from "@/services/api/resources";

export function useAccountFileStorageUsage(enabled = true) {
    const userId = useUserStore((state) => state.user?.id);
    return useQuery({
        queryKey: [...accountFileStorageUsageQueryKey, userId],
        queryFn: ({ signal }) => getAccountFileStorageUsage(signal),
        enabled: enabled && Boolean(userId),
        staleTime: 30_000,
        refetchOnMount: "always",
    });
}
