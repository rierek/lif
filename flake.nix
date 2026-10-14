{
  description = "Agentic-first issue tracker";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      version = "0.13.0";

      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          frontend = pkgs.stdenv.mkDerivation (finalAttrs: {
            pname = "lif-frontend";
            inherit version;

            src = ./frontend;

            pnpmDeps = (pkgs.fetchPnpmDeps.override { pnpm = pkgs.pnpm_10; }) {
              inherit (finalAttrs) pname version src;
              fetcherVersion = 4;
              hash = "sha256-hJxpzC9QJftvMVENtXvaS3b/vSKGqpaJd93z8cJhK+I=";
            };

            nativeBuildInputs = [
              pkgs.nodejs
              pkgs.pnpm_10
              (pkgs.pnpmConfigHook.override { pnpm = pkgs.pnpm_10; })
            ];

            buildPhase = ''
              runHook preBuild
              pnpm build
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              cp -r build $out
              runHook postInstall
            '';
          });
        in
        {
          default = pkgs.buildGoModule {
            pname = "lif";
            inherit version;

            src = ./.;

            vendorHash = "sha256-G4YT/S+weXdCs1jlxHmwWkwSyIRIGdqMjOqgf4QPUPo=";

            preBuild = ''
              rm -rf internal/web/dist
              mkdir -p internal/web/dist
              cp -r ${frontend}/. internal/web/dist/
              touch internal/web/dist/.gitkeep
            '';

            subPackages = [ "cmd/lif" "cmd/lif-serve" ];

            ldflags = [
              "-s"
              "-w"
              "-X github.com/xRiErOS/lif/internal/version.Version=${version}"
              "-X github.com/xRiErOS/lif/internal/version.Commit=${self.shortRev or "unknown"}"
              "-X github.com/xRiErOS/lif/internal/version.Date=1970-01-01T00:00:00Z"
            ];

            meta = {
              description = "Agentic-first issue tracker";
              homepage = "https://github.com/xRiErOS/lif";
              license = pkgs.lib.licenses.asl20;
              maintainers = [ ];
              mainProgram = "lif";
            };
          };

          inherit frontend;

          lif = self.packages.${system}.default;
        }
      );

      apps = forAllSystems (system: {
        default = self.apps.${system}.lif;

        lif = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/lif";
        };

        lif-serve = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/lif-serve";
        };
      });

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              gopls
              gotools
              go-tools
              nodejs
              pnpm_10
              just
              mise
            ];

            shellHook = ''
              echo "lif development environment"
              echo "Go version: $(go version)"
              echo "Node version: $(node --version)"
            '';
          };
        }
      );
    };
}
