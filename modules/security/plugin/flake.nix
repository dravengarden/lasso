{
  description = "Lasso Security Codex plugin development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          # Announce a pinned Columbus shell to the repository toolchain guard.
          COLUMBUS_HARNESS_DEV_SHELL = "1";

          packages = with pkgs; [
            python314
            uv
            ruff
            ty
            shellcheck
            shfmt
            just
            osv-scanner
            pip-audit
            trivy
            grype
          ];
        };
        formatter = pkgs.nixfmt;
      }
    );
}
