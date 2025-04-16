import Header from '@/components/header'
import type { BrowserOAuthClient, OAuthSession } from '@atproto/oauth-client-browser'
import { useMutation, type QueryClient } from '@tanstack/react-query'
import { Outlet, createRootRouteWithContext, useRouter } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'

interface RouterContext {
  queryClient: QueryClient
  oauthClient: BrowserOAuthClient
  authSession: OAuthSession | undefined
}

export const Route = createRootRouteWithContext<RouterContext>()({
  async beforeLoad({ context }) {
    const result = await context.oauthClient.init()
    return {
      authSession: result?.session
    }
  },
  async loader({ context }) {
    if (!context.authSession) {
      return {}
    }
    const identityReq = new URL('/xrpc/com.atproto.repo.describeRepo', window.location.origin)
    identityReq.searchParams.set('repo', context.authSession.did)

    const response = await context.authSession.fetchHandler(identityReq.toString())
    const details = await response.json() as { handle: string }
    return {
      handle: details.handle,
    }
  },
  component() {
    const { invalidate } = useRouter()
    const { authSession, oauthClient } = Route.useRouteContext()
    const { handle } = Route.useLoaderData();
    const { mutate: logout } = useMutation({
      async mutationFn() {
        if (!authSession) return
        oauthClient.revoke(authSession.sub)
        invalidate()
      }
    })
    return (
      <>
        <Header isAuthenticated={!!authSession} handle={handle} onLogout={logout} />
        <Outlet />
        <TanStackRouterDevtools />
      </>
    )
  },
})
