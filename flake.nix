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
    {
      self,
      funzzy,
      nixpkgs,
      ...
    }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      version = "0-unstable-${builtins.substring 0 8 self.lastModifiedDate}";
      commit = if self ? shortRev then self.shortRev else "dirty";
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
            vendorHash = "sha256-ObM5Xn5Bp0h4+Zvk1II2RAXR9JVE6ftoOrP6VE7B2ds=";
            subPackages = [ "cmd/jeq" ];
            env.CGO_ENABLED = "0";
            meta = {
              description = "Agent-first TypeSafe judgment CLI";
              license = pkgs.lib.licenses.mit;
            };
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
      checkSet = forAllSystems (system: {
        packaged-version = nixpkgs.legacyPackages.${system}.runCommand "jeq-packaged-version" { } ''
          output=$(${packageSet.${system}.jeq}/bin/jeq version)
          grep -F "jeq v${version}" <<<"$output"
          grep -F "Commit: ${commit}" <<<"$output"
          touch $out
        '';
      });
    in
    {
      packages = packageSet;
      apps = appSet;
      checks = checkSet;
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
              goreleaser
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
