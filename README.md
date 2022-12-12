# freshen

Tools for automatically updating Nix Flakes. An alternative to update scripts.

## Example

A Nix Flake repo contains a `freshen.json` file:

```json
{
  "update_tasks": [
    {
      "name": "my-build-name",
      "attr_path": "my-build",
      "inputs": [
        "flake-input-name"
      ],
      "derived_hashes": [{
        "attr_path": "my-build.hashUpdate",
        "filename": "my-build/hash.json"
      }],
      "tests": [{
        "attr_path": "my-build-test"
      }]
    }
  ]
}
```

In this example, there's an update task named "my-build-name". It's updating something that can be built by running `nix build -A .#my-build`. It has one flake input called "flake-input-name". There's one derived hash stored in the `my-build/hash.json` file, and a mismatch for the derived hash can be produced by running `nix build .#my-build.hashUpdate`.

This command can be run in the repository root: `freshen update --name my-build-name`. This will update the flake inputs and derived hash file. It will run the build and associated tests.

## Derived hashes

Some derivations have extra hashes that are derived from their flake inputs and the network. For example, Rust builds often need a `cargoSha256` hash for cargo dependencies. Freshen can update these derived hashes. To do this, create an attrPath that will produce a mismatch for the derived hash. For example, override a rust build and set `cargoSha256` to `lib.fakeSha256`. This is referred to as a "mismatch attrPath". Freshen will take the mismatch attrPath, build it, extract the new hash, and store it in the "hash file" in JSON string format. The main build can load the hash file from disk.

## Tests

Each update task can specify tests to verify that an update succeeded. These are listed in "tests".

## Remote updates

Freshen can check automatically commit updates to a GitHub repo.

Example command: `freshen remote-update --config github.json --name my-build-name`

Example `github.json` file:

```json
{
  "branch": "main",
  "author": "mybotname",
  "email": "mybotname@example.com",
  "github": {
    "owner": "squalus",
    "repo": "freshen"
  }
}
```

The GitHub support requires a personal access API token. Generate one and store it in `$CREDENTIALS_DIRECTORY/github_token.txt`. Or provide the token filename in `github.json` under `.github.token_file`.

## Stability

This is experimental and there's no stability guarantee. The file format and command line arguments might change without notice.

