package common

const Name = "ageServer"
const Cert = "cert.pem"
const Key = "key.pem"
const CACert = "cacert.pem"

// CACertPath is the route the server serves the CA certificate on. It is not
// the file name (CACert) the certificate stores keep using, and it leaves
// /cacert.pem free for the API library's own certificate route.
const CACertPath = "cacert.pem/root"

// ApiCACertPath is the route a standalone API server serves its own certificate
// on: the library's route, without the /root suffix this server's certificate
// uses.
const ApiCACertPath = "cacert.pem"
const SelfSignedCert = "selfsigned_cert.pem"
const SelfSignedKey = "selfsigned_key.pem"

// SigningKey is the RelicLink dataSignature signing keypair. The private half
// signs catalogue bodies the server serves; the public half is the SPKI the
// client process is pointed at so it verifies those signatures.
//
// The .der is public and ships with the launcher companion. The .pem is secret
// and must stay on the server.
const SigningKey = "sigkey.pem"
const SigningPublicKey = "sigkey.der"

const CertSubjectOrganization = "github.com/luskaner/" + Name
