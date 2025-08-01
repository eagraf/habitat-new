"""Demo of a serverless app using `wasi-http` to handle inbound HTTP requests.

This demonstrates how to use WASI's asynchronous capabilities to manage multiple
concurrent requests and streaming bodies.  It uses a custom `asyncio` event loop
to thread I/O through coroutines.
"""

import hashlib

from wit_world.imports.types import (
    Method_Get,
    Method_Post,
    Scheme,
    Scheme_Http,
    Scheme_Https,
    Scheme_Other,
    IncomingRequest,
    ResponseOutparam,
    OutgoingResponse,
    Fields,
    OutgoingBody,
    OutgoingRequest,
)

from wit_world.imports.outgoing_handler import handle as fetch
from wit_world.types import Ok, Result, Some
from wit_world import exports


class IncomingHandler(exports.IncomingHandler):
    def handle(self, request: IncomingRequest, response_out: ResponseOutparam) -> None:
        fetch(OutgoingRequest(request.headers()), None)
        resp = OutgoingResponse(Fields())
        resp.set_status_code(200)
        resp.body().write().write(b"Hello, World!")
        ResponseOutparam.set(response_out, Ok(resp))

