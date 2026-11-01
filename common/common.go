package common

const Name = "ageServer"
const Cert = "cert.pem"
const Key = "key.pem"
const CACert = "cacert.pem"
const SelfSignedCert = "selfsigned_cert.pem"
const SelfSignedKey = "selfsigned_key.pem"

// RelicLink dataSignature signing keypair. The private half signs catalogue
// bodies the server serves; the public half is the SPKI the client process is
// pointed at so it verifies those signatures.
//
// The .der is public and ships with the launcher companion. The .pem is secret
// and must stay on the server.
const SigningKey = "sigkey.pem"
const SigningPublicKey = "sigkey.der"

const CertSubjectOrganization = "github.com/luskaner/" + Name
