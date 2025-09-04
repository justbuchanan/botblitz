{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs =
    inputs@{
      self,
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
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [
            python312
            python3Packages.pip
            python3Packages.virtualenv
            gcc
          ];
          shellHook = ''
            export LD_LIBRARY_PATH="${pkgs.stdenv.cc.cc.lib.outPath}/lib:${pkgs.pythonManylinuxPackages.manylinux2014Package}/lib:$LD_LIBRARY_PATH";

            if [ ! -d ".venv" ]; then
              echo "Creating virtual environment..."
              python3.12 -m venv .venv
              source .venv/bin/activate
              echo "Installing python packages..."
              pip install -r requirements.txt
            else
              source .venv/bin/activate
            fi
          '';
        };
      }
    );
}
