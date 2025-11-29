import {
    exchangeCodeAsync,
    loadAsync,
    makeRedirectUri,
} from "expo-auth-session";
import { createContext, PropsWithChildren, useContext, useMemo } from "react";
import * as SecureStore from "expo-secure-store";
import { useQuery, useQueryClient } from "@tanstack/react-query";

const clientId = "https://sashankg.github.io/client-metadata.json"; // fake for now
const domain = "privi.dwelf-mirzam.ts.net";
const redirectUri = makeRedirectUri({
    scheme: "habitat.camera",
    path: "oauth",
});

const issuer = {
    authorizationEndpoint: `https://${domain}/oauth/authorize`,
    tokenEndpoint: `https://${domain}/oauth/token`,
};

const secureStoreKey = "token";

export type FetchWithAuth = (
    url: string,
    options?: Parameters<typeof fetch>[1],
) => Promise<Response>;

interface AuthContextData {
    signIn: (handle: string) => Promise<void>;
    signOut: () => void;
    token: string | null;
    isLoading: boolean;
    fetchWithAuth: FetchWithAuth;
}

const AuthContext = createContext<AuthContextData>({
    signIn: async () => { },
    signOut: () => { },
    token: null,
    isLoading: false,
    fetchWithAuth: fetch,
});

export const useAuth = () => useContext(AuthContext);

export const AuthProvider = ({ children }: PropsWithChildren) => {
    const queryClient = useQueryClient();
    const { data: token, isLoading } = useQuery({
        queryKey: ["token"],
        queryFn: async () => {
            const token = await SecureStore.getItemAsync(secureStoreKey);
            if (!token) return null;
            return token;
        },
    });
    const value = useMemo<AuthContextData>(
        () => ({
            signIn: async (handle: string) => {
                const authRequest = await loadAsync(
                    {
                        extraParams: {
                            handle,
                        },
                        clientId: clientId,
                        scopes: [],
                        redirectUri: redirectUri,
                    },
                    issuer,
                );
                const authResponse = await authRequest.promptAsync(issuer);
                if (authResponse.type !== "success") return;
                const tokenResponse = await exchangeCodeAsync(
                    {
                        clientId,
                        code: authResponse.params.code,
                        redirectUri,
                        extraParams: {
                            code_verifier: authRequest.codeVerifier ?? "",
                        },
                    },
                    issuer,
                );
                await SecureStore.setItemAsync(
                    secureStoreKey,
                    tokenResponse.accessToken,
                );
                await queryClient.invalidateQueries({ queryKey: ["token"] });
            },
            token: token ?? null,
            signOut: () => {
                SecureStore.deleteItemAsync(secureStoreKey);
                queryClient.invalidateQueries({ queryKey: ["token"] });
            },
            isLoading,
            fetchWithAuth: (url, options) => {
                return fetch(new URL(url, `https://${domain}`), {
                    ...options,
                    headers: {
                        Authorization: `Bearer ${token}`,
                        "Habitat-Auth-Method": "oauth",
                        ...options?.headers,
                    },
                });
            },
        }),
        [token, isLoading, queryClient],
    );
    return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
