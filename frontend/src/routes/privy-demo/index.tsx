import type { FormEvent } from 'react';

import { useMutation } from '@tanstack/react-query'
import Cookies from 'js-cookie';
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/privy-demo/')({
    component() {
        const { authSession } = Route.useRouteContext()
        const { mutate: handleSubmit } = useMutation({
            mutationFn: async (e: FormEvent<HTMLFormElement>) => {
                e.preventDefault()
                if (!authSession) { throw new Error('No auth session'); }
                const formData = new FormData(e.target as HTMLFormElement);
                const rkey = formData.get('name') as string
                const response = await authSession?.fetchHandler('http://localhost:3000/xrpc/com.habitat.putRecord', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'atproto-proxy': 'did:web:sashankg.github.io#privi',
                    },
                    body: JSON.stringify({
                        input: {
                            repo: authSession.did,
                            collection: 'com.habitat.privyDemo.messages',
                            rkey,
                            record: {
                                message: formData.get('message'),
                            },
                            validate: false,
                        },
                        encrypt: true,
                    }),
                });
                if (!response?.ok) {
                    throw new Error('putRecord failed');
                }
                const url = new URL('/privy-demo/view', window.location.href);
                url.searchParams.set('did', authSession.did);
                url.searchParams.set('rkey', rkey);

                console.log(url.toString());
                navigator.clipboard.writeText(url.toString());
            },
            onError: (error) => {
                alert(`Error submitting form: ${error}`);
            }
        })
        return (
            <form onSubmit={handleSubmit}>
                <input name="name" type="text" placeholder="Name" required />
                <textarea name='message' placeholder="Message" required />
                <button type="submit">Save and copy link</button>
            </form>
        )
    }
});
