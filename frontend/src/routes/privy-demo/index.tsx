import type { FormEvent } from 'react';

import { useMutation } from '@tanstack/react-query'
import { useAuth } from '@/components/authContext';
import Cookies from 'js-cookie';
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/privy-demo/')({
    component() {
        const { handle } = useAuth()
        const { mutate: handleSubmit } = useMutation({
            mutationFn: async (e: FormEvent<HTMLFormElement>) => {
                e.preventDefault()
                if (!handle) {
                    throw new Error('Not logged in')
                }
                const formData = new FormData(e.target as HTMLFormElement);
                const rkey = formData.get('name') as string
                const did = Cookies.get('user_did') as string
                const response = await fetch('/xrpc/com.habitat.putRecord', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        input: {
                            repo: did,
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
                if (!response.ok) {
                    throw new Error('putRecord failed');
                }
                const url = new URL('/privy-demo/view', window.location.href);
                url.searchParams.set('did', did);
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
