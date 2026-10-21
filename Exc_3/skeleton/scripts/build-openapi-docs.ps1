Set-Location (Join-Path $PSScriptRoot "..")

$directories = "./"

& "$(go env GOPATH)\bin\swag.exe" init --pd --st -g main.go --dir $directories -o docs
