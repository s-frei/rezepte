package demo

import (
	"encoding/base64"
	"encoding/binary"
)

// demoIdentityMember is the household member the demo pre-links to an
// identity provider when OIDC is configured, so the people list and profile
// show a "Signs in with …" line from the first start, without anyone
// connecting an account by hand. demo and mila still link themselves from
// their own profile, the ordinary way; see docs/memory/content/features/demo-mode.mdx.
const demoIdentityMember = "jonas"

// dexConnector is the connector id Dex's built-in password database uses for
// its static users (`enablePasswordDB: true` in mise/oidc/dex.yaml.tmpl).
// dexSubject reproduces the "sub" claim Dex issues for demoIdentityMember on
// it, so the pre-linked identity matches what a real sign-in through the
// test Dex presents.
const dexConnector = "local"

// dexSubject reproduces the "sub" claim Dex's OpenID Connect provider issues
// for a static user with the given userID, signed in through the connector
// named connectorID. Dex encodes it as the unpadded base64url of a protobuf
// message (field 1 the user id, field 2 the connector id - Dex's
// server/internal/types.proto, IDTokenSubject). Hand-rolling that two-field
// wire format avoids a protobuf dependency for it; this is fine for demo/test
// data the seed constructs itself, never for anything Rezepte verifies
// untrusted input against - login itself still goes through go-oidc's real
// token verification.
func dexSubject(userID, connectorID string) string {
	b := appendProtoString(nil, 1, userID)
	b = appendProtoString(b, 2, connectorID)
	return base64.RawURLEncoding.EncodeToString(b)
}

// appendProtoString appends a protobuf length-delimited string field (wire
// type 2) to b: a tag byte, the length as a varint, then the bytes. field is
// a byte, not an int, so the tag's shift can never overflow it.
func appendProtoString(b []byte, field byte, s string) []byte {
	b = append(b, field<<3|2)
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}
