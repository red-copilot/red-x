import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 20_000,
      refetchOnWindowFocus: false,
      retry: (count, error) => {
        const status = (error as { status?: number }).status
        return status !== 401 && status !== 403 && count < 2
      },
    },
  },
})
