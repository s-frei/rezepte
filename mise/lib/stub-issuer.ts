// A stand-in OpenID provider for the user-docs screenshots (RZP_DEMO_OIDC=stub
// in mise/tasks/demo.sh): it answers discovery and nothing else, which is all
// saving a provider in the settings asks of one. No sign-in ever runs against
// it. Prints its origin, the issuer, on stdout and serves until killed.

const server = Bun.serve({
	hostname: '127.0.0.1',
	port: 0,
	fetch(req) {
		if (new URL(req.url).pathname !== '/.well-known/openid-configuration') {
			return new Response('not found', { status: 404 });
		}
		return Response.json({
			issuer: origin,
			authorization_endpoint: `${origin}/authorize`,
			token_endpoint: `${origin}/token`,
			jwks_uri: `${origin}/jwks`,
			response_types_supported: ['code'],
			subject_types_supported: ['public'],
			id_token_signing_alg_values_supported: ['RS256']
		});
	}
});
const origin = `http://127.0.0.1:${server.port}`;
console.log(origin);
