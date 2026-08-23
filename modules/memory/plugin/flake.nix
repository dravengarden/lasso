{
  description = "Lasso Memory Codex plugin development shell";

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
            python314FreeThreading
            uv
            ruff
            ty
            shellcheck
            shfmt
            just
          ];
        };
        formatter = pkgs.nixfmt;
      }
    );
}
