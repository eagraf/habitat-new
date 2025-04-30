import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute('/_requireAuth/plc-operation')({
  component() {
    const { authSession } = Route.useRouteContext();
    const { mutate: requestOperation } = useMutation({
      async mutationFn() {
        await authSession?.fetchHandler('/xrpc/com.atproto.identity.requestPlcOperationSignature', {
          method: 'POST'
        })
      }
    })
    return (
      <article>
        <h1>PLC Operation</h1>
        <button onClick={() => requestOperation()}>Request Operation</button>
      </article>
    );
  }
});
