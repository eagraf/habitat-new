'use client'

import React, { createContext, useContext, useState, ReactNode, useEffect } from 'react';
import Cookies from 'js-cookie';
import { useRouter } from 'next/navigation';
import Header from './header';

interface AuthContextType {
    isAuthenticated: () => boolean;
    handle: string | null;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {

        
    const [handle, setHandle] = useState<string | null>(null);
    const router = useRouter();

    useEffect(() => {
        const handle = Cookies.get('handle');
        if (handle) {
            setHandle(handle);
        }
    }, []);


    const logout = () => {
        Cookies.remove('state');
        Cookies.remove('handle');

        router.push('/login');
    };

    const isAuthenticated = () => {
        const handleCookie = Cookies.get('handle');
        return handleCookie !== undefined;
    }

    return (
        <AuthContext.Provider value={{ isAuthenticated, handle, logout }}>
            <div className="flex flex-col items-center justify-center w-full h-screen">
                <div className="flex flex-col items-center justify-center w-full">
                    <Header isAuthenticated={isAuthenticated()} handle={handle} logout={logout} />
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