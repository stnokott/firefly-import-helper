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
    # yaml
    yaml-language-server
  ];
  shellHook = ''
    go env -w CGO_ENABLED=0
  '';
}
