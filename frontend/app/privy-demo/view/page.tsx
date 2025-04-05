'use client'

import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

export default function View() {
    const searchParams = useSearchParams()
    const did = searchParams.get('did')
    const rkey = searchParams.get('rkey')
    const enabled = !!did && !!rkey
    const { data: message, isLoading } = useQuery({
        queryKey: ['message', did, rkey],
        queryFn: async () => {
            const params = new URLSearchParams(searchParams)
            params.set('collection', 'com.habitat.privyDemo.messages')
            const response = await fetch(`/habitat/api/xrpc/com.habitat.getRecord?${params.toString()}`);
            const data = await response.json();
            return data.value.message as string
        },
        enabled,
        retry: false,
    })

    if (!enabled) {
        return <p>Invalid url</p>
    }
    if (isLoading) {
        return <p>Loading...</p>
    }

    return <div className="border rounded p-4">
        <p>{message}</p>
    </div>
}
