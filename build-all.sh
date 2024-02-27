#!/bin/bash

BINARY_NAME="lexido"

# Array of build targets
PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64")

# Clear previous builds
echo "Cleaning up previous builds..."
rm -rf build
mkdir build

# Loop through each platform and build
for platform in "${PLATFORMS[@]}"; do
    # Splitting platform into OS and architecture
    IFS='/' read -r -a os_arch <<< "$platform"

    # Setting up environment variables
    GOOS="${os_arch[0]}"
    GOARCH="${os_arch[1]}"
    OUTPUT="build/${BINARY_NAME}-${GOOS}-${GOARCH}"

    if [ $GOOS = "windows" ]; then
        OUTPUT+='.exe'
    fi

    echo "Building for $GOOS $GOARCH..."
    env GOOS=$GOOS GOARCH=$GOARCH go build -o $OUTPUT

    # Check if build was successful
    if [ $? -ne 0 ]; then
        echo "An error has occurred! Aborting the script execution..."
        exit 1
    fi
done

echo "Build process completed."
