import { AuthProvider } from '@/components/authContext'
import Header from '@/components/header'
import { Outlet, createRootRoute, useLocation } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'

export const Route = createRootRoute({
  component: () => {
    const location = useLocation();
    const isLoginPage = location.pathname === '/login';
    
    return (
      <AuthProvider>
        {!isLoginPage && <Header />}
        <Outlet />
        <TanStackRouterDevtools />
      </AuthProvider>
    );
  },
})
