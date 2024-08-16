package util

const WaitUntilInitDone = `
until [[ -f /fly-init/pg-ready && -f /fly-init/nss-ready ]]; do printf .; sleep 1; done;
`
