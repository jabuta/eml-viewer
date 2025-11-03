#!/bin/bash
set -e

# EML Viewer Build Script
# Builds binaries for Windows, macOS, and Linux

VERSION="v1.0.0"
APP_NAME="eml-viewer"
OUTPUT_DIR="dist"

echo "🚀 Building $APP_NAME $VERSION"
echo "================================"

# Create output directory
mkdir -p $OUTPUT_DIR

# Build flags
LDFLAGS="-s -w -X main.Version=$VERSION"

# Platforms to build
declare -a PLATFORMS=(
    "windows/amd64"
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
)

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}

    OUTPUT_NAME="$APP_NAME-$GOOS-$GOARCH"

    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi

    echo ""
    echo "📦 Building for $GOOS/$GOARCH..."

    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="$LDFLAGS" \
        -o "$OUTPUT_DIR/$OUTPUT_NAME" \
        .

    if [ $? -eq 0 ]; then
        SIZE=$(ls -lh "$OUTPUT_DIR/$OUTPUT_NAME" | awk '{print $5}')
        echo "   ✅ Built $OUTPUT_NAME ($SIZE)"
    else
        echo "   ❌ Failed to build $OUTPUT_NAME"
        exit 1
    fi
done

echo ""
echo "================================"
echo "✨ Build complete!"
echo ""
echo "Binaries created in $OUTPUT_DIR/:"
ls -lh $OUTPUT_DIR/ | tail -n +2 | awk '{print "   " $9 " (" $5 ")"}'

# Optional: Create checksums
echo ""
echo "📝 Generating checksums..."
cd $OUTPUT_DIR
shasum -a 256 * > SHA256SUMS
cd ..
echo "   ✅ SHA256SUMS created"

echo ""
echo "🎉 Done!"
