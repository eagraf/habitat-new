'use client'

import React, { useEffect } from 'react';
import Cookies from 'js-cookie';
import { useRouter, usePathname } from 'next/navigation';
import { useAuth } from './authContext';

const withAuth = (WrappedComponent: React.FC) => {
    const ComponentWithAuth = (props: any) => {
        const pathname = usePathname();
        const router = useRouter();

        const isAuthenticated = () => {
            const handleCookie = Cookies.get('handle');
            return handleCookie !== undefined;
        }

        useEffect(() => {
            const authenticated = isAuthenticated();
            if (!authenticated) {
                console.log("Redirecting to login");
                router.push('/login');
            }
        }, [pathname]);


        return <WrappedComponent {...props} />;
    };

    return ComponentWithAuth;
};

export default withAuth;