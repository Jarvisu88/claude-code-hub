package selector

// This file documents the session reuse stage (Stage 1) of the provider selection pipeline.
//
// Session reuse is implemented directly in selector.go (trySessionReuse method)
// since it is tightly coupled to the overall pipeline flow.
//
// Session reuse validation checks (in order):
//  1. Session service availability + non-empty session ID
//  2. Provider bound to session in Redis
//  3. Provider exists and is enabled
//  4. Provider has not opted out of session reuse (DisableSessionReuse)
//  5. Provider is within its active time schedule
//  6. Provider circuit breaker is not open
//  7. Provider supports the requested model
//  8. Provider format is compatible with the request format
//  9. Provider group matches the user/key group
//  10. Provider client restrictions allow the user-agent
//  11. Provider cost limits are not exceeded
//  12. Provider is not in the exclude list
//
// If any check fails, session reuse falls through to the normal selection pipeline.
// This matches the behavior of the Node.js findReusable() method.
