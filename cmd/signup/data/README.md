# Embedded data

## `common_passwords.txt`

The 10,000 most common passwords, sourced from
[SecLists](https://github.com/danielmiessler/SecLists) at
`Passwords/Common-Credentials/10k-most-common.txt`.

Used by the password validator (`cmd/signup/password.go`) to flag the
`ContainsCommonPassword` penalty — mirroring Proton's own
`pass-rust-core/password` analyzer, which does the same check.

**License:** MIT © 2018 Daniel Miessler. Compatible with this repo's
MIT license. Full text: <https://github.com/danielmiessler/SecLists/blob/master/LICENSE>.
