{
  description = "Agent-first TypeSafe CLI development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    funzzy = {
      url = "github:cristianoliveira/funzzy/788703efa18ce96f2f9174a9f5c8b43986dfe7a5";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    { funzzy, nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      version = "0.0.420";
      commit = "flake-build";
    in
    let
      packageSet = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          jeq = pkgs.buildGoModule {
            pname = "jeq";
            inherit version;
            src = ./.;
            vendorHash = "sha256-7K17JaXFsjf163g5PXCb5ng2gYdotnZ2IDKk8KFjNj0=";
            subPackages = [ "cmd/jeq" ];
            env.CGO_ENABLED = "0";
            ldflags = [
              "-s"
              "-w"
              "-X github.com/cristianoliveira/jeq/internal/cli.Version=v${version}"
              "-X github.com/cristianoliveira/jeq/internal/cli.Commit=${commit}"
            ];
          };
        in
        {
          inherit jeq;
          default = jeq;
        }
      );
      appSet = forAllSystems (system: {
        jeq = {
          type = "app";
          program = "${packageSet.${system}.jeq}/bin/jeq";
        };
        default = {
          type = "app";
          program = "${packageSet.${system}.default}/bin/jeq";
        };
      });
    in
    {
      packages = packageSet;
      apps = appSet;
      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              funzzy.packages.${system}.local
              git
              gnumake
              go
              golangci-lint
              gopls
              govulncheck
              jq
              nixfmt
              python3
            ];

            GOTOOLCHAIN = "local";
          };
        }
      );
    };
}
