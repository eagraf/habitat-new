import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute('/_requireAuth/privy-test')({
  component() {
    const { authSession } = Route.useRouteContext()
    const { mutate } = useMutation({
      async mutationFn() {
        const response = await authSession?.fetchHandler('/xrpc/com.habitat.putRecord', {
          method: 'POST',
          body: JSON.stringify({
            input: {
              collection: 'com.habitat.test',
              record: {
                foo: 'bar'
              },
              repo: authSession.did,
            }
          })
        })
        console.log(response)
      },
      onError(e) {
        console.error(e)
      }
    })
    return (<article>
      <h1> Privy Test</h1 >
      <button onClick={() => mutate()}>Test</button>
    </article >)
  }

})
