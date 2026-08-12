import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ApiError } from './shared/api/mediaHub'
import { AuthGate } from './features/auth/AuthGate'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false
        return failureCount < 1
      },
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthGate />
    </QueryClientProvider>
  )
}
