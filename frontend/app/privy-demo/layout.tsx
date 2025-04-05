'use client'
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { PropsWithChildren } from "react";

const client = new QueryClient();

export default function PrivyDemoLayout({ children }: PropsWithChildren) {
    return <>
        <link
            rel="stylesheet"
            href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css"
        />
        <QueryClientProvider client={client}>
            <div style={{ 'backgroundColor': 'var(--pico-background-color)' }} className="w-full h-full flex flex-col justify-center p-6 items-center" >
                <h1 className="">Privy Demo</h1>
                <div className='flex justify-center flex-col w-full max-w-md'>
                    {children}
                </div>
            </div>
        </QueryClientProvider>
    </>
}
