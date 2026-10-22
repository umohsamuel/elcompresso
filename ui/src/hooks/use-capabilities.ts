import { useQuery } from "@tanstack/react-query";
import { getCapabilities } from "@/lib/api/jobs";
import { queryKeys } from "@/constants/api";

export function useCapabilities() {
  return useQuery({
    queryKey: queryKeys.capabilities,
    queryFn: getCapabilities,
    staleTime: 5 * 60 * 1000,
    retry: 2,
    refetchOnWindowFocus: (q) => q.state.status === "error",
    // Keep checking while the server is down, so the page recovers on its
    // own once it starts (e.g. right after restarting the backend).
    refetchInterval: (q) => (q.state.status === "error" ? 5000 : false),
  });
}
