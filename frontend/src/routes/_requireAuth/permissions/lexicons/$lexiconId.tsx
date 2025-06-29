import { listPermissions } from "@/queries/permissions";
import { useMutation } from "@tanstack/react-query";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { useForm } from 'react-hook-form'

interface Data {
    did: string
}

export const Route = createFileRoute('/_requireAuth/permissions/lexicons/$lexiconId')({
    async loader({ context, params }) {
        const response = await context.queryClient.fetchQuery({
            ...listPermissions(context.authSession),
        })
        console.log(response[params.lexiconId])
        return response[params.lexiconId]
        //return [
        //    { did: 'sdlkjfhalskdjfalsdkjfh' },
        //    { did: 'lcvkjhcxlgkjhxcllckxjh' },
        //]
    },
    component() {
        const router = useRouter()
        const { authSession } = Route.useRouteContext()
        const params = Route.useParams()
        const people = Route.useLoaderData()
        const form = useForm<Data>({})
        const { mutate: add, isPending: isAdding } = useMutation({
            async mutationFn(data: Data) {
                const response = await authSession?.fetchHandler(`/xrpc/com.habitat.addPermission`, {
                    method: 'POST',
                    body: JSON.stringify({
                        did: data.did,
                        lexicon: params.lexiconId
                    }),
                    headers: {
                        'atproto-proxy': 'did:web:localhost-0.taile529e.ts.net#privi'
                    }
                })
                console.log(data.did)
                form.reset()
                router.invalidate()
                return
            },
            onError(e) {
                console.error(e)
            }
        })

        const { mutate: remove } = useMutation({
            async mutationFn(data: Data) {
                // remove permission
                const response = await authSession?.fetchHandler(`/xrpc/com.habitat.removePermission`, {
                    method: 'POST',
                    body: JSON.stringify({
                        did: data.did,
                        lexicon: params.lexiconId
                    }),
                    headers: {
                        'atproto-proxy': 'did:web:localhost-0.taile529e.ts.net#privi'
                    }
                })
                router.invalidate()
                return
            },
            onError(e) {
                console.error(e)
            }
        })
        return <>
            <h3>{params.lexiconId}</h3>
            <form onSubmit={form.handleSubmit((data) => add(data))}>
                <fieldset role="group">
                    <input type="text" {...form.register('did')} />
                    <button type="submit" aria-busy={isAdding}>Add</button>
                </fieldset>
            </form>
            <table>
                <thead>
                    <tr>
                        <th>Person</th>
                        <th />
                    </tr>
                </thead>
                <tbody>
                    {people?.map((person) => <tr key={person} >
                        <td>{person}</td>
                        <td><button type="button" onClick={() => remove({ did: person })}>🗑️</button></td>
                    </tr>)}
                </tbody>
            </table>
        </>
    }
});
