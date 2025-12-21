#!/bin/bash


APP_NAME="BiliRecorder"

OUTPUT_DIR="build"
mkdir -p "$OUTPUT_DIR"


declare -A TARGETS=(
  ["windows_amd64"]="GOOS=windows GOARCH=amd64"
  ["linux_amd64"]="GOOS=linux GOARCH=amd64"
)

for target in "${!TARGETS[@]}"; do
  eval "${TARGETS[$target]}" go build -ldflags="-s" -o "$OUTPUT_DIR/$APP_NAME-$target"
  # eval "${TARGETS[$target]}"
  # echo "Building for $target: GOOS=${GOOS} GOARCH=${GOARCH}"
  # go build -o "$OUTPUT_DIR/$APP_NAME-$target" -ldflags="-s -w"
  if [ $? -eq 0 ]; then
    echo "Build successful: $OUTPUT_DIR/$APP_NAME-$target"
    if [[ "$target" != darwin_* ]]; then
        upx --best --lzma "$OUTPUT_DIR/$APP_NAME-$target"
    fi
  else
    echo "Build failed: $target"
  fi
done