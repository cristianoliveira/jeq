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
    in
    {
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
            ];

            GOTOOLCHAIN = "local";
          };
        }
      );
    };
}
