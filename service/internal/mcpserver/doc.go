// Package mcpserver serves the recipe collection to AI assistants over the
// Model Context Protocol: a stateless Streamable HTTP endpoint whose tools
// call recipe.Service directly and are offered per request according to the
// calling API token's scopes.
package mcpserver
