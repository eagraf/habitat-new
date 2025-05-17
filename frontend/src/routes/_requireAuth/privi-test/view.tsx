import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute('/_requireAuth/privi-test/view')({
  validateSearch(search) {
    return {
      did: search.did as string,
      rkey: search.rkey as string,
    }
  },
  loaderDeps: ({ search }) => (search),
  async loader({ deps: { did, rkey }, context }) {
    const params = new URLSearchParams()
    params.set('repo', did)
    params.set('rkey', rkey)
    params.set('collection', 'com.habitat.privyDemo.messages')
    const response = await context.authSession?.fetchHandler(`/habitat/api/xrpc/com.habitat.getRecord?${params.toString()}`, {
      headers: {
        'atproto-proxy': 'CENTRAL_PRIVI_DID_GOES_HERE#privi'
      }
    });
    return response?.json()
  },
  component() {
    const message = Route.useLoaderData()
    return <div className="border rounded p-4">
      <p>{message}</p>
    </div>
  }
})
