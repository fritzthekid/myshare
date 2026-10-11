# Requirements: Nextcloud PDF printer driver

| ID | Requirement | Covered by |
|----|-------------|-----------------|
| R1 | The driver is invoked as a CUPS backend (`job-id user title copies options [file]`) and reads the PDF print data either from the given file or from stdin. Without arguments (device discovery, e.g. `lpinfo -v`), it responds with a CUPS-compliant discovery line instead of an error. The file extension is chosen based on the actual content (magic numbers) instead of blindly assuming `.pdf`, since `lp -d <queue> arbitrary.zip` also passes non-PDF content through unchanged. | `cmd/backend/main_test.go::TestReadsPDFFromFile`, `::TestReadsPDFFromStdin`, `::TestNoArgsPrintsDiscoveryLine`, `::TestNonPDFPayloadGetsMatchingExtension`, `::TestTitleExtensionKeptWhenItDiffersFromContent`, `internal/filenames/filenames_test.go::TestDetectExtension` |
| R2 | If the PDF payload is missing (empty data), the job is aborted with an error status (exit code ≠ 0), without outputting a link. | `cmd/backend/main_test.go::TestEmptyPDFIsRejected` |
| R3 | A unique, filesystem-safe filename is generated for every file from timestamp + job title (special characters are replaced). | `internal/filenames/filenames_test.go` |
| R4 | The PDF is uploaded via WebDAV PUT into a configurable target directory of the Nextcloud instance. | `internal/nextcloud/client_test.go::TestUploadFileSuccess`, `::TestUploadFileFailureRaises` |
| R5 | A share link is created for the uploaded file via the OCS share API (`shareType=3`, public link), which automatically expires after a configurable number of days (default 1) (`expireDate`). | `internal/nextcloud/client_test.go::TestCreatePublicLinkSuccess`, `::TestCreatePublicLinkFailureRaises`, `::TestCreatePublicLinkSetsExpireDate`, `::TestCreatePublicLinkRespectsCustomExpireDays` |
| R6 | Configuration (server URL, user, password/app token, target directory) comes from environment variables, not hardcoding. If a required variable is missing, a clear error is raised. | `internal/config/config_test.go` |
| R7 | The created public link is output (for now as a log line + QR code via a replaceable `output` callback, default: `internal/output.Default`). | `cmd/backend/main_test.go::TestFullFlowOutputsLink`, `internal/output/output_test.go` |
| R8 | Errors during upload or share creation lead to an error message on stderr and exit code 1 (CUPS convention for failed jobs), instead of a crash. | `cmd/backend/main_test.go::TestUploadErrorIsReported`, `::TestShareErrorIsReported` |
| R9 | Besides the printer driver invocation, there's a simple CLI entry point (`sendfile`) to share an arbitrary file (path or stdin, with an extension from the label/filename) via the same upload+share mechanism. | `cmd/sendfile/main_test.go` |
| R10 | Additionally, there's an HTTP entry point (`webshare`) through which, e.g., a phone on the network can use the same upload+share mechanism without its own Nextcloud credentials; secured via a secret token as part of the URL (wrong/missing token → 404), QR code is delivered as a PNG in the JSON response. | `cmd/webshare/main_test.go` |
| R11 | For the case where no permanently reachable server is available, there's a standalone, client-side HTML/JS page (`web/share.html`) that talks to Nextcloud directly from the browser (e.g. on the phone) via WebDAV/OCS API - credentials never leave the browser of the respective device. | no Go test (no Go code); core logic (filename generation, basic auth encoding) manually verified via Node.js against the same test vectors as `internal/filenames`, see README |
| R12 | Alternative to the Pi framebuffer display: `cloudweb` is a standalone web server (no Nextcloud credentials needed) that displays the QR code + link (+ optional password) most recently pushed via the output hook and keeps itself up to date automatically via polling. The push endpoint (`POST /push`) only accepts requests from localhost. | `cmd/cloudweb/main_test.go` |
| R13 | The Pi display and `cloudweb` are reachable via two separate, simultaneously usable CUPS queues (`CloudToRaspi`, `CloudWeb`) instead of a single global hook switch - both use the same backend code, the `$PRINTER` name set by CUPS decides in the wrapper script which output hook runs. | no Go test (shell/CUPS configuration); manually verified live (`lp -d CloudToRaspi ...` → Pi, `lp -d CloudWeb ...` → cloudweb) |
| R14 | Via `scripts/share-to-web.sh <file> [name]`, a `CloudWeb` share can additionally be protected with a Nextcloud share password (natively via the OCS API, `CreatePublicLinkWithPassword`) - meant especially for larger/more sensitive files. `name` is passed through via a CUPS job option (`-o nc-password-seed=<name>`); the actual password is `name` + a displayed short form (`crypt`) derived from `name` + the print server's MAC address + a unique remote path (`internal/sharepassword`) - this hardened seed flows only into `crypt`, not into the password itself (the recipient only knows `name` and the displayed `crypt` after all). Without the MAC address, `crypt` could be computed from just the `name` communicated to the recipient (no real 2FA) and would be identical for every share with the same `name` - both prevented by mixing in the MAC address + remote path. Without a `name` argument, the share remains unprotected by a passphrase as before. | `internal/sharepassword/sharepassword_test.go`, `cmd/backend/main_test.go::TestPasswordSeedFromOptionsDerivesPassword`, `::TestSameNameProducesDifferentCryptOnRepeatedRuns`, `::TestPasswordSeedRequestedButMACLookupFailsAborts`, `::TestNoPasswordSeedMeansNoPassword`, `internal/nextcloud/client_test.go::TestCreatePublicLinkWithPasswordSendsPassword`, `::TestCreatePublicLinkSendsNoPasswordByDefault` |

## Deliberately left open (for now)
- Output is a log line + QR code PNG (no desktop notification, no email) – per the original spec.
- Authentication: Basic Auth with a Nextcloud app password (recommended instead of the plaintext account password).
- The actual CUPS setup (`deploy/install.sh`, PPD, wrapper script) is system configuration
  outside the automated tests – manually/smoke-tested, see README "Integration into CUPS".
- `web/share.html` (R11) was tested live against a real Nextcloud instance – there it fails as
  expected due to missing CORS (verified via `curl -X OPTIONS`). For exactly this case (no
  CORS, no reachable server), `webshare` (R10) instead runs directly on the phone in Termux
  (no Go code difference, just a cross-compile pitfall: pure-Go DNS fails on Android without
  cgo – native compilation in Termux fixes that) – see `doc/android.md`.
  This combination was tested successfully end-to-end (upload + share link + QR display).
- `scripts/share.sh` is a plain `lp` wrapper (no own upload code) for machines with
  a configured CUPS queue – no Go code, no separate test, manually verified live against the
  real queue (file argument and stdin, see README). Target queue switchable via
  `PRINTER=...` (default `CloudToRaspi`); `scripts/share-to-web.sh` is the way to reach
  `CloudWeb` incl. optional password protection (R14).
- `scripts/push-to-cloudweb.sh` (output hook for `cloudweb`, R12/R13/R14) is a plain
  `curl` wrapper, no Go code, manually verified live end-to-end (push, `current.png`,
  `current.json`, home page, incl. the password field from R14). With missing arguments it
  shows a usage message incl. a note that it (unlike `share.sh`) is not an upload tool, but
  expects an already-finished QR PNG.
