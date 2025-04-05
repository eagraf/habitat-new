import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute('/privy-demo/view')({
    validateSearch(search) {
        return {
            did: search.did as string,
            rkey: search.rkey as string,
        }
    },
    async loader() {
        const { did, rkey } = Route.useSearch()
        const params = new URLSearchParams()
        params.set('did', did)
        params.set('rkey', rkey)
        params.set('collection', 'com.habitat.privyDemo.messages')
        const response = await fetch(`/habitat/api/xrpc/com.habitat.getRecord?${params.toString()}`);
        const data = await response.json();
        return data.value.message as string

    },
    component() {
        const message = Route.useLoaderData()
        return <div className="border rounded p-4">
            <p>{message}</p>
        </div>
    }
});
