import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

export const Route = createFileRoute('/oauth_login')({
  component() {
    const [isPending, setPending] = useState(false)
    return <article>
      <h1>Login</h1>
      <form method="get" action="/habitat/api/login" onSubmit={() => setPending(true)}>
        <input name="handle" type="text" placeholder="Handle" required defaultValue={"sashankg.bsky.social"} />
        <button aria-busy={isPending} type="submit">Login</button>
      </form>
    </article>
  }
})
