{
  description = "Lasso — agent-native monorepo workspace";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "lasso";
          version = "0.1.0";
          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter =
              path: type:
              let
                rel = pkgs.lib.removePrefix (toString ./. + "/") (toString path);
              in
              type == "directory"
              || builtins.elem rel [
                "go.mod"
                "go.sum"
              ]
              || pkgs.lib.hasPrefix "cmd/" rel
              || pkgs.lib.hasPrefix "internal/" rel;
          };
          vendorHash = "sha256-9NOuJhh2ixGigHb+W5oJXIPZKcDNBNzAymMjjqTyHhU=";
          subPackages = [ "cmd/lasso" ];
          ldflags = [
            "-X github.com/dravengarden/lasso/internal/cli.Version=0.1.0"
          ];
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            just
            git
            shellcheck
            shfmt
          ];
          shellHook = ''
            export PATH="$PWD/scripts:$PWD/bin:$PATH"
            export LASSO_KIT_ROOT="$PWD"
            echo "lasso dev shell — $(go version | cut -d' ' -f3)" >&2
          '';
        };
      }
    );
}
