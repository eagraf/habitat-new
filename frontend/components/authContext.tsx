'use client'

import React, { createContext, useContext, useState, ReactNode, useEffect } from 'react';
import Cookies from 'js-cookie';
import { useRouter, usePathname } from 'next/navigation';
import Header from './header';

import axios from 'axios';
interface AuthContextType {
    authenticated: boolean;
    handle: string | null;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {

    const [handle, setHandle] = useState<string | null>(null);
    const [authenticated, setAuthenticated] = useState(false);
    const router = useRouter();
    const pathname = usePathname();

    // Check the handle, every time a page navigation change occurs. Since the cookie
    // is set by the oauth server's redirect, this will detect changes in the handle.
    useEffect(() => {
        const handle = Cookies.get('handle');
        if (handle) {
            setHandle(handle);
        }
    }, [pathname]);


    const logout = async () => {
        // Remove all session state from the browser side.
        Cookies.remove('state');
        Cookies.remove('handle');
        setHandle(null);
        setAuthenticated(false);

        // Tell the server to invalidate the session
        const response = await axios.post('/habitat/oauth/logout');
        if (response.status === 200) {
            router.push('/login');
        } else {
            console.error('Logout failed: ', response.status);
        }
    };

    useEffect(() => {
        const handleCookie = Cookies.get('handle');
        setAuthenticated(handleCookie !== undefined);
    }, [pathname]);

    return (
        <AuthContext.Provider value={{ authenticated, handle, logout }}>
            <div className="flex flex-col items-center justify-center w-full h-screen">
                <div className="flex flex-col items-center justify-center w-full">
                    <Header authenticated={authenticated} handle={handle} logout={logout} />
                </div>
                <div className="flex flex-col items-center justify-center w-full h-screen">
                    {children}
                </div>
            </div>
        </AuthContext.Provider>
    );
};

export const useAuth = (): AuthContextType => {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
};