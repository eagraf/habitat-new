This is how it would work "in the wild".
The ideal is everyone runs their own privi instance, wrapping private data.

[Arushi Private PDS][Arushi Privi]
[Sashank Private PDS][Sashanki Privi]

If Arushi wants to request private data from Sashank's PPDS:
0. Arushi is given Sashank's DID.

Arushi resolves Sashank's DID document to get Sashank Privi DID.
Arushi requests her own PDS with atproto-proxy header containing Sashank Privi DID.
Arushi PDS signs the request (jwt) which means request is coming from me.
Arushi PDS forwards the request to Sashank's Privi instance via DID resolve.
Privi checks permissions and returns result to Arushi
