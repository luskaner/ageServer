# Launcher Common

This module contains the code used by all internal `launcher-*` modules.

**No guarantees are given for the stability of the API.**

The launcher session configures the Age of Empires API separately when
`[Server.Api] Host` names a standalone API server: it probes `/test`, trusts the
certificate served on `/cacert.pem`, maps the API domains to it and only then
maps the root server's domains. The domain halves are `common.ApiHosts` and
`common.RootHosts`, and the certificate pass is `ops.AddApiCert`.

The host-set halves survive the elevation hop: `config setup` forwards them to
`config-admin` (directly or through `config-admin-agent`) as `--api`/`--rootOnly`
(`--rootOnly` is hidden; it is the pass that maps every domain but the API
ones). The agent accepts repeated host and certificate additions — a session
may run an API pass and the server's own through the very same agent — and one
revert still restores the whole hosts file and both certificates.
