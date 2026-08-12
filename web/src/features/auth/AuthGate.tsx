import { useEffect } from 'react'
import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { SearchWorkspace } from '../search/SearchWorkspace'
import {
  ApiError,
  getSession,
  logout,
  type LoginResponse,
  type SessionResponse,
  unauthorizedEvent,
} from '../../shared/api/mediaHub'
import { LoginScreen } from './LoginScreen'
import './auth.css'

const sessionQueryKey = ['auth', 'session'] as const

export function AuthGate() {
  const queryClient = useQueryClient()
  const session = useQuery<SessionResponse | null>({
    queryKey: sessionQueryKey,
    queryFn: getSession,
    retry: false,
  })
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => clearAuthenticatedState(queryClient),
  })

  useEffect(() => {
    const handleUnauthorized = () => clearAuthenticatedState(queryClient)
    window.addEventListener(unauthorizedEvent, handleUnauthorized)
    return () => window.removeEventListener(unauthorizedEvent, handleUnauthorized)
  }, [queryClient])

  const handleAuthenticated = (response: LoginResponse) => {
    queryClient.setQueryData<SessionResponse>(sessionQueryKey, response)
  }

  if (session.isPending) {
    return <div className="auth-loading" role="status"><span />正在连接 Media Hub</div>
  }

  if (session.data) {
    return (
      <SearchWorkspace
        isLoggingOut={logoutMutation.isPending}
        onLogout={() => logoutMutation.mutate()}
      />
    )
  }

  const serviceError = session.error instanceof ApiError && session.error.status !== 401
    ? session.error.message
    : null
  return <LoginScreen onAuthenticated={handleAuthenticated} serviceError={serviceError} />
}

function clearAuthenticatedState(queryClient: QueryClient) {
  queryClient.setQueryData<SessionResponse | null>(sessionQueryKey, null)
  queryClient.removeQueries({
    predicate: (query) => query.queryKey[0] !== 'auth',
  })
}
