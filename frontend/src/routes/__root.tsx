import { AuthProvider } from '@/components/authContext'
import Header from '@/components/header'
import { oauthClient } from '@/lib/oauthClient'
import { Outlet, createRootRoute } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'

export const Route = createRootRoute({
  async loader() {
    try {
      const result = await oauthClient.init()
      console.log(result)
    }
    catch (e) {
      console.log(e)
    }
  },
  component: () => (
    <AuthProvider>
      <Header />
      <Outlet />
      <TanStackRouterDevtools />
    </AuthProvider>
  ),
})
