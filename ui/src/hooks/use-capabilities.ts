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
  });
}
