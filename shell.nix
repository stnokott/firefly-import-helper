{ pkgs ? import <nixpkgs> {}}:

pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    # Go
    go
    gopls
    delve
    golangci-lint
    golangci-lint-langserver
    gofumpt
    # Docker
    hadolint
  ];
  shellHook = ''
    go env -w CGO_ENABLED=0
  '';
}
