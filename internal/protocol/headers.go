package protocol

const (
	// HeaderAuthCorrelationID is the correlation header required by Aegis.
	HeaderAuthCorrelationID = "Auth-CorrelationId"
	// HeaderAuthKid identifies the shared secret to use on the server side.
	HeaderAuthKid = "Auth-Kid"
	// HeaderAuthHeaders lists the additional signed headers separated by semicolons.
	HeaderAuthHeaders = "Auth-Headers"
	// HeaderSignature contains the final base64-encoded request signature.
	HeaderSignature = "Signature"
)
